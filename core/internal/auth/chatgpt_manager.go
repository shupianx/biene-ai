package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// chatgptRefreshSkew is how long before token expiry we proactively
// refresh. 5 minutes matches the reference implementation.
const chatgptRefreshSkew = 5 * time.Minute

// ChatGPTManager owns the in-memory mirror of ~/.biene/chatgpt_tokens.json
// and is the single point through which providers obtain a working
// access_token. The token IS what gets sent as `Authorization: Bearer
// …` to chatgpt.com/backend-api/codex/responses — there is no separate
// sk-… API key step; pi-coding-agent's Codex provider works the same
// way ([openai-codex.ts:448-450](https://github.com/badlogic/pi-mono/
// blob/main/packages/ai/src/utils/oauth/openai-codex.ts#L448)).
//
// The manager is safe to share across goroutines: every read takes
// the mutex, expired tokens trigger an in-place refresh, and the
// refreshed credentials persist back to disk under the same lock so
// concurrent sessions can't double-refresh.
//
// lastError is a transient field that records the most recent OAuth
// failure (token exchange, refresh, etc.) so the status endpoint can
// surface it to the renderer's poll loop. It's intentionally NOT
// persisted — restarting the core wipes it.
type ChatGPTManager struct {
	mu        sync.Mutex
	state     *ChatGPTState
	lastError string
	// rateLimits is the most recent rate-limit snapshot derived from
	// the `x-codex-*` headers on a /responses stream. Nil until the
	// user sends their first message after login (or after core
	// restart). Re-populated on every subsequent stream so the
	// Settings panel always reflects the most recent turn's data.
	rateLimits *RateLimitSnapshot
}

// NewChatGPTManager loads any persisted credentials. A missing file is
// fine — the manager simply reports "not authenticated" until login.
func NewChatGPTManager() (*ChatGPTManager, error) {
	state, err := LoadChatGPTState()
	if err != nil {
		return nil, err
	}
	return &ChatGPTManager{state: state}, nil
}

// Authenticated returns true when a refresh token is on file. The
// access_token may be expired — APIKey()/AccessToken() refresh on demand.
func (m *ChatGPTManager) Authenticated() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state != nil && m.state.RefreshToken != ""
}

// Snapshot returns a copy of the current state suitable for status
// responses. Tokens are NOT included to avoid leaking them through
// logs/JSON encoders. LastError surfaces the most recent OAuth failure
// so renderer poll loops can fail fast instead of timing out.
func (m *ChatGPTManager) Snapshot() ChatGPTPublicStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil || m.state.RefreshToken == "" {
		return ChatGPTPublicStatus{
			Authenticated: false,
			LastError:     m.lastError,
		}
	}
	return ChatGPTPublicStatus{
		Authenticated: true,
		Email:         m.state.Email,
		AccountID:     m.state.AccountID,
		ExpiresAt:     m.state.ExpiresAt,
	}
}

// SetLastAuthError records a recent OAuth failure for the status
// endpoint to surface. Pass "" on success to clear stale messages.
func (m *ChatGPTManager) SetLastAuthError(msg string) {
	m.mu.Lock()
	m.lastError = msg
	m.mu.Unlock()
}

// IngestRateLimitHeaders parses the `x-codex-*` headers off a Codex
// response and caches the resulting snapshot. No-op when the
// response carried no rate-limit headers (some fast-path routes
// strip them). The Settings panel reads the cached value via
// RateLimits().
func (m *ChatGPTManager) IngestRateLimitHeaders(h http.Header) {
	snap := ParseRateLimitHeaders(h)
	if snap == nil {
		return
	}
	m.mu.Lock()
	m.rateLimits = snap
	m.mu.Unlock()
}

// RateLimits returns the most recent cached snapshot, or nil if the
// user hasn't sent a Codex turn since core started. The returned
// pointer references the cached instance — callers must treat it as
// read-only. The snapshot is small and immutable post-ingest; we
// don't deep-copy on read.
func (m *ChatGPTManager) RateLimits() *RateLimitSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rateLimits
}

// ChatGPTPublicStatus is the non-secret view of the credential state
// surfaced through the REST API. LastError carries the message from
// the most recent failed login attempt so the renderer can stop
// polling and tell the user what went wrong.
//
// `Authenticated` is the single source of truth for "is this user
// signed in" — there is no separate has_refresh_token flag because
// Authenticated is *defined* as RefreshToken != "" (see Authenticated
// and Snapshot above), so the two were always equivalent.
type ChatGPTPublicStatus struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
	AccountID     string `json:"account_id,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

// SetFromCode runs the post-callback token exchange and persists the
// resulting credentials.
//
// access_token is what we use as the Bearer token against
// chatgpt.com/backend-api. account_id is parsed from the access_token
// JWT (path: `https://api.openai.com/auth.chatgpt_account_id`) per
// pi-coding-agent's reference implementation.
func (m *ChatGPTManager) SetFromCode(ctx context.Context, code, codeVerifier string) error {
	tokens, err := ExchangeChatGPTCode(ctx, code, codeVerifier)
	if err != nil {
		m.SetLastAuthError(err.Error())
		return err
	}
	state := &ChatGPTState{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}
	state.AccountID = extractCodexAccountID(tokens.AccessToken)
	if state.AccountID == "" {
		// account_id is required for every Codex backend request as the
		// `chatgpt-account-id` header. If it's missing the credentials
		// are unusable; refuse to persist them and tell the user.
		msg := "access_token did not include a chatgpt_account_id claim"
		m.SetLastAuthError(msg)
		return errors.New(msg)
	}
	state.Email = extractIDTokenEmail(tokens.IDToken)

	if err := SaveChatGPTState(state); err != nil {
		m.SetLastAuthError(err.Error())
		return err
	}
	m.mu.Lock()
	m.state = state
	m.lastError = ""
	m.mu.Unlock()
	return nil
}

// Logout clears the in-memory state and the on-disk file.
func (m *ChatGPTManager) Logout() error {
	m.mu.Lock()
	m.state = nil
	m.lastError = ""
	m.mu.Unlock()
	return DeleteChatGPTState()
}

// ErrChatGPTNotAuthenticated is returned when the user has not
// completed OAuth (or has logged out).
var ErrChatGPTNotAuthenticated = errors.New("not signed in to ChatGPT")

// APIKey returns the bearer token providers should send to
// chatgpt.com/backend-api. It refreshes the access_token automatically
// when within chatgptRefreshSkew of expiry. Refresh + persistence
// happen under the manager's mutex.
func (m *ChatGPTManager) APIKey(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil || m.state.RefreshToken == "" {
		return "", ErrChatGPTNotAuthenticated
	}
	if !m.expiringSoonLocked() && m.state.AccessToken != "" {
		return m.state.AccessToken, nil
	}
	if err := m.refreshLocked(ctx); err != nil {
		return "", err
	}
	return m.state.AccessToken, nil
}

// AccountID returns the chatgpt-account-id header value the Codex
// backend requires alongside the bearer token. Triggers a refresh on
// the same skew as APIKey() so callers can read either independently.
func (m *ChatGPTManager) AccountID(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil || m.state.RefreshToken == "" {
		return "", ErrChatGPTNotAuthenticated
	}
	if m.expiringSoonLocked() {
		if err := m.refreshLocked(ctx); err != nil {
			return "", err
		}
	}
	return m.state.AccountID, nil
}

func (m *ChatGPTManager) expiringSoonLocked() bool {
	if m.state == nil || m.state.ExpiresAt == 0 {
		// Unknown expiry — be conservative and refresh on every call.
		return true
	}
	deadline := time.UnixMilli(m.state.ExpiresAt).Add(-chatgptRefreshSkew)
	return time.Now().After(deadline)
}

func (m *ChatGPTManager) refreshLocked(ctx context.Context) error {
	tokens, err := RefreshChatGPTTokens(ctx, m.state.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh chatgpt tokens: %w", err)
	}
	m.state.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		m.state.RefreshToken = tokens.RefreshToken
	}
	m.state.ExpiresAt = tokens.ExpiresAt
	if accountID := extractCodexAccountID(tokens.AccessToken); accountID != "" {
		m.state.AccountID = accountID
	}
	// Refresh the email opportunistically: the id_token's email claim
	// is stable for an account, but the user may have changed it
	// upstream (e.g. corporate SSO email update) and a token refresh
	// is the only point we ever see a fresh id_token. We don't store
	// the id_token itself — only the extracted email.
	if email := extractIDTokenEmail(tokens.IDToken); email != "" {
		m.state.Email = email
	}
	return SaveChatGPTState(m.state)
}

// extractCodexAccountID pulls the chatgpt_account_id claim out of an
// access_token. The claim path matches what the Codex backend expects
// in the `chatgpt-account-id` header.
//
// Reference: pi-coding-agent's getAccountId reads the same path —
// `payload["https://api.openai.com/auth"].chatgpt_account_id`
// ([openai-codex.ts:283-288](https://github.com/badlogic/pi-mono/blob/
// main/packages/ai/src/utils/oauth/openai-codex.ts#L283)).
//
// We don't verify the JWT signature — the token is only consumed by
// OpenAI's own backend, which validates it on its side, so any tamper
// would surface as an upstream 401 anyway.
func extractCodexAccountID(accessToken string) string {
	payload := decodeJWTPayload(accessToken)
	if payload == nil {
		return ""
	}
	auth, _ := payload["https://api.openai.com/auth"].(map[string]any)
	if auth == nil {
		return ""
	}
	id, _ := auth["chatgpt_account_id"].(string)
	return id
}

// extractIDTokenEmail pulls the email claim from the id_token (a
// separate JWT from the access_token). This is for display only — if
// it's missing the Settings card just shows "Authorized" without an
// address.
func extractIDTokenEmail(idToken string) string {
	payload := decodeJWTPayload(idToken)
	if payload == nil {
		return ""
	}
	email, _ := payload["email"].(string)
	return email
}

func decodeJWTPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Tolerate base64 padding variants — some issuers emit padded
		// segments that RawURLEncoding rejects.
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil
	}
	return claims
}
