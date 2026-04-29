// Package auth implements OAuth flows that turn user credentials into
// runnable Provider keys.
//
// chatgpt_oauth.go contains the ChatGPT (Codex CLI public client) PKCE
// flow. The strategy mirrors badlogic/pi-mono and the upstream Codex
// CLI itself:
//
//  1. Server generates PKCE + auth URL using the public Codex CLI client_id.
//  2. Client opens the URL in the user's browser; OAuth callback lands on
//     a one-shot HTTP listener bound to localhost:1455.
//  3. Server exchanges the authorization code for { id_token, access_token,
//     refresh_token }.
//  4. The access_token IS the bearer the Codex backend
//     (chatgpt.com/backend-api/codex/responses) accepts directly — no
//     separate sk-… key step. The chatgpt-account-id header value is
//     parsed from the access_token's JWT claim path
//     `https://api.openai.com/auth.chatgpt_account_id`. The id_token is
//     only used at login to extract the email claim for UI display.
//
// The Codex CLI client_id and 1455 port are baked into OpenAI's OAuth
// registration; we cannot register our own without applying with OpenAI.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"biene/internal/templates"
)

const (
	chatgptClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	chatgptAuthURL      = "https://auth.openai.com/oauth/authorize"
	chatgptTokenURL     = "https://auth.openai.com/oauth/token"
	chatgptRedirectURI  = "http://localhost:1455/auth/callback"
	chatgptScopes       = "openid profile email offline_access"
	chatgptCallbackPort = 1455

	// chatgptOriginator identifies us to the Codex backend. Codex CLI
	// uses "codex_cli_rs"; pi-coding-agent uses "pi". This value is
	// echoed in both the authorize URL and the request headers so the
	// two ends of the OAuth dance agree on who's calling. OpenAI may
	// some day whitelist originators — that's the same gray-area risk
	// as reusing the public Codex client_id, accepted at design time.
	chatgptOriginator = "biene"
)

// ChatGPTOfficialModels returns the curated list of OpenAI models the
// "ChatGPT (official)" virtual provider exposes. The list is derived
// from the templates.Builtin entry for vendor=openai_compatible at
// https://api.openai.com/v1, so adding a new model means editing one
// place (templates.go) instead of two — previously this was a separate
// hardcoded slice that drifted from the template every time.
//
// Function rather than a top-level var because templates.Builtin is a
// package-level value that may not be initialised by the time auth's
// init runs in some orderings; deferring the read keeps the dependency
// edge clean.
func ChatGPTOfficialModels() []string {
	return templates.ModelsForVendor("openai_compatible", chatgptOfficialBaseURL)
}

// chatgptOfficialBaseURL is the canonical URL the templates entry uses
// for OpenAI. It also drives ChatGPTOfficialContextWindow's lookup —
// keep both call sites consistent so a future move to a different
// vendor URL only changes one line.
const chatgptOfficialBaseURL = "https://api.openai.com/v1"

// ChatGPTOfficialContextWindow returns the input-token capacity of a
// ChatGPT-OAuth-provisioned model.
//
// The lookup delegates to templates.LookupContextWindow against the
// `openai_compatible` vendor at https://api.openai.com/v1 — the same
// row a user would land on if they manually configured an OpenAI
// API key against this model. Routing both paths through one source
// keeps the synthetic OAuth provider and the "manual sk-..." preset
// from drifting (they describe the same OpenAI-side model).
//
// Without an explicit window the session manager would fall back to
// config.DefaultContextWindow (32K) and compaction would fire every
// turn against a model that actually has hundreds of thousands of
// tokens of headroom — the user-visible symptom is a constant "no
// safe cut point in current history; will retry next turn" warning.
//
// chatgptOfficialDefaultContextWindow is the fallback when a model
// id isn't yet in templates.go. We use the GPT-5 family value rather
// than the conservative 32K so a freshly added model still behaves
// reasonably until its template entry lands.
const chatgptOfficialDefaultContextWindow = 400_000

func ChatGPTOfficialContextWindow(model string) int {
	if w, ok := templates.LookupContextWindow(
		"openai_compatible", model, chatgptOfficialBaseURL,
	); ok {
		return w
	}
	return chatgptOfficialDefaultContextWindow
}

// PreparedFlow carries the artifacts of a fresh PKCE handshake. The
// codeVerifier MUST stay on the server — it's the secret half of the
// PKCE pair and is only paired with the matching code at exchange time.
type PreparedFlow struct {
	AuthURL      string
	State        string
	CodeVerifier string
}

// ChatGPTTokens is the post-exchange credential set. RefreshToken is
// optional because not every OAuth provider issues one (we always do,
// thanks to `offline_access` in the scope).
type ChatGPTTokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpiresAt is unix milliseconds — matches the format craft-agents-oss
	// uses on disk for symmetry. Zero means "unknown".
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// PrepareChatGPTOAuth generates a fresh state + PKCE verifier and assembles
// the authorize URL. Side-effect free; the caller is responsible for
// stashing the verifier somewhere keyed by the returned state.
func PrepareChatGPTOAuth() (*PreparedFlow, error) {
	state, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate pkce: %w", err)
	}

	q := url.Values{}
	q.Set("client_id", chatgptClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", chatgptRedirectURI)
	q.Set("scope", chatgptScopes)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	// These two extras are required by the Codex CLI registration:
	// without them OpenAI redirects through the regular ChatGPT login
	// path and the resulting tokens can't be exchanged for an API key.
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("id_token_add_organizations", "true")
	q.Set("originator", chatgptOriginator)

	return &PreparedFlow{
		AuthURL:      chatgptAuthURL + "?" + q.Encode(),
		State:        state,
		CodeVerifier: verifier,
	}, nil
}

// ExchangeChatGPTCode trades the OAuth authorization code for tokens.
func ExchangeChatGPTCode(ctx context.Context, code, codeVerifier string) (*ChatGPTTokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", chatgptClientID)
	form.Set("code", code)
	form.Set("redirect_uri", chatgptRedirectURI)
	form.Set("code_verifier", codeVerifier)
	return postTokenForm(ctx, form, "token exchange")
}

// RefreshChatGPTTokens uses the refresh_token to mint a new access/id token
// pair. OpenAI rotates the refresh_token sometimes; we keep the old one
// when the response omits a new value (matches reference impl).
func RefreshChatGPTTokens(ctx context.Context, refreshToken string) (*ChatGPTTokens, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", chatgptClientID)
	form.Set("refresh_token", refreshToken)
	t, err := postTokenForm(ctx, form, "token refresh")
	if err != nil {
		return nil, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

// ChatGPTCallbackPort returns the fixed port the OAuth callback listener
// must bind to. Exposed so the server's listener and config display can
// agree on the value without duplicating it.
func ChatGPTCallbackPort() int { return chatgptCallbackPort }

// IsChatGPTOfficialModelID reports whether modelID is the synthetic
// "chatgpt_official:<model>" form used to route a session to the OAuth
// provider. The colon syntax mirrors the existing config IDs being
// kebab-cased — it can never collide with a sanitized user ID.
func IsChatGPTOfficialModelID(modelID string) bool {
	return strings.HasPrefix(modelID, "chatgpt_official:")
}

// ParseChatGPTOfficialModelID splits a "chatgpt_official:<model>" ID
// into the OpenAI model name component. Returns "" on malformed input.
func ParseChatGPTOfficialModelID(modelID string) string {
	if !IsChatGPTOfficialModelID(modelID) {
		return ""
	}
	return strings.TrimPrefix(modelID, "chatgpt_official:")
}

// ── helpers ───────────────────────────────────────────────────────────

func postTokenForm(ctx context.Context, form url.Values, op string) (*ChatGPTTokens, error) {
	body, err := postForm(ctx, chatgptTokenURL, form, op)
	if err != nil {
		return nil, err
	}
	var resp struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", op, err)
	}
	t := &ChatGPTTokens{
		IDToken:      resp.IDToken,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}
	if resp.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).UnixMilli()
	}
	return t, nil
}

func postForm(ctx context.Context, urlStr string, form url.Values, op string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s failed: %d %s", op, resp.StatusCode, extractOAuthError(body))
	}
	return body, nil
}

// extractOAuthError pulls a human-readable error description out of a
// failed token endpoint body. Falls back to the raw payload if the body
// isn't a typical {"error_description": "..."} envelope.
func extractOAuthError(body []byte) string {
	var env struct {
		ErrorDescription string `json:"error_description"`
		Error            any    `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil {
		if env.ErrorDescription != "" {
			return env.ErrorDescription
		}
		switch v := env.Error.(type) {
		case string:
			if v != "" {
				return v
			}
		case map[string]any:
			if msg, ok := v["message"].(string); ok && msg != "" {
				return msg
			}
		}
	}
	return strings.TrimSpace(string(body))
}

func generatePKCE() (verifier, challenge string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
