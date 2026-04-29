package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"biene/internal/auth"
	"biene/internal/logging"
)

// requireLocalOrigin gates side-effecting OAuth endpoints against
// CSRF-style requests from public web pages on the same machine.
// Without this, any browser tab the user happens to have open could
// POST /api/auth/chatgpt/start, bind port 1455, and open the user's
// browser to OpenAI's authorize URL — annoying at best, a phishing
// vector at worst (the page could observe the redirect URL by other
// means and complete the flow against its own attacker-controlled
// account).
//
// The accept rule is intentionally generous so we don't break any
// legitimate caller:
//   - No Origin header → programmatic client (curl, Electron preload
//     IPC fetch, Go HTTP client). Browsers always send Origin on
//     non-GET requests, so absence is a strong "not a browser" signal.
//   - Origin == "null" → file:// load (production Electron build).
//   - Origin's host is a loopback address (127.0.0.1 / localhost / ::1)
//     → the Vite dev server or another local tool.
//
// Anything else — a public scheme://domain.tld origin — gets a 403.
// authToken middleware already blocks unauthorized callers when
// configured; this is defense-in-depth for the side-effecting verbs.
func requireLocalOrigin(r *http.Request) error {
	o := strings.TrimSpace(r.Header.Get("Origin"))
	if o == "" || o == "null" {
		return nil
	}
	u, err := url.Parse(o)
	if err != nil || u.Host == "" {
		return errForbiddenOrigin(o)
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return errForbiddenOrigin(o)
}

func errForbiddenOrigin(origin string) *forbiddenOriginErr {
	return &forbiddenOriginErr{origin: origin}
}

type forbiddenOriginErr struct{ origin string }

func (e *forbiddenOriginErr) Error() string {
	return "request origin not permitted: " + e.origin
}

// pendingChatGPTFlow holds the in-memory half of an in-progress OAuth
// dance. The codeVerifier never leaves the server: the client gets the
// authUrl + flowId only, and the redirect-relay loop matches state back
// to the verifier on completion.
type pendingChatGPTFlow struct {
	flowID       string
	state        string
	codeVerifier string
	createdAt    time.Time
	cancel       context.CancelFunc
}

const chatgptFlowTTL = 5 * time.Minute

// chatgptOAuthCoordinator owns active OAuth flows. The map is small (a
// single in-progress flow at a time in practice) so a plain mutex is
// enough; we expire any flow older than chatgptFlowTTL on each lookup.
//
// onAuthChanged is invoked after a successful SetFromCode (login) so
// the surrounding server can broadcast session_updated to renderers
// — agent cards pinned to chatgpt_official re-evaluate their
// availability without needing to poll. Logout fires the same hook
// from handleChatGPTLogout.
type chatgptOAuthCoordinator struct {
	mu            sync.Mutex
	flows         map[string]*pendingChatGPTFlow
	auth          *auth.ChatGPTManager
	starter       chatgptCallbackStarter
	onAuthChanged func()
}

// chatgptCallbackStarter is satisfied by auth.StartChatGPTCallback.
// Made an interface only so tests can swap it without binding to port 1455.
type chatgptCallbackStarter func() (*auth.ChatGPTCallbackListener, error)

func newChatGPTCoordinator(mgr *auth.ChatGPTManager) *chatgptOAuthCoordinator {
	return &chatgptOAuthCoordinator{
		flows:   make(map[string]*pendingChatGPTFlow),
		auth:    mgr,
		starter: auth.StartChatGPTCallback,
	}
}

func (c *chatgptOAuthCoordinator) cleanupExpired() {
	now := time.Now()
	for state, flow := range c.flows {
		if now.Sub(flow.createdAt) > chatgptFlowTTL {
			if flow.cancel != nil {
				flow.cancel()
			}
			delete(c.flows, state)
		}
	}
}

// ── HTTP handlers ─────────────────────────────────────────────────────

type chatgptStartResponse struct {
	AuthURL   string `json:"auth_url"`
	State     string `json:"state"`
	FlowID    string `json:"flow_id"`
	Port      int    `json:"port"`
	ExpiresIn int    `json:"expires_in_seconds"`
	// ManualPasteRequired is true when the localhost:1455 listener
	// could not be bound (port already in use — typically because
	// Codex CLI itself is running, or a stale Biene process didn't
	// release the socket). The renderer surfaces a paste box where
	// the user copies the redirected URL back from the browser; the
	// server then completes the flow via /api/auth/chatgpt/manual-callback.
	ManualPasteRequired bool `json:"manual_paste_required,omitempty"`
	// PortBindError carries the underlying bind error message so the
	// UI can explain *why* the manual fallback kicked in (e.g.
	// "address already in use"). Empty when the listener bound
	// normally.
	PortBindError string `json:"port_bind_error,omitempty"`
}

func (s *Server) handleChatGPTStart(w http.ResponseWriter, r *http.Request) {
	if err := requireLocalOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if s.chatgptOAuth == nil {
		writeError(w, http.StatusServiceUnavailable, "chatgpt oauth not initialised")
		return
	}
	c := s.chatgptOAuth

	prepared, err := auth.PrepareChatGPTOAuth()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Try to bind the callback listener. We previously 409'd here when
	// port 1455 was busy, which dead-ended any user who happened to
	// have Codex CLI running. Now we register the flow either way and
	// flip the manual-paste flag when the listener can't start; the
	// renderer then shows a paste box that POSTs to manual-callback.
	listener, listenErr := c.starter()

	// Wipe any error from a previous failed attempt so the renderer's
	// poll loop doesn't immediately surface stale text on retry.
	c.auth.SetLastAuthError("")

	flowCtx, cancel := context.WithTimeout(context.Background(), chatgptFlowTTL)
	flow := &pendingChatGPTFlow{
		flowID:       generateFlowID(),
		state:        prepared.State,
		codeVerifier: prepared.CodeVerifier,
		createdAt:    time.Now(),
		cancel:       cancel,
	}

	c.mu.Lock()
	c.cleanupExpired()
	c.flows[flow.state] = flow
	c.mu.Unlock()

	if listener != nil {
		// The listener resolves on the first request the browser makes
		// to http://localhost:1455/auth/callback. The completion
		// routine matches the returned state to the pending flow, then
		// runs the token exchange under the auth.ChatGPTManager's lock.
		go c.completeFlowFromCallback(flowCtx, listener, flow)
	} else {
		// Manual-paste fallback: nothing watches the listener, but the
		// flow still expires after chatgptFlowTTL via the timeout
		// context above so the verifier eventually drops out of the
		// map even if the user abandons the paste box.
		logging.Lifecycle().Warn("chatgpt callback listener unavailable; manual paste required",
			"err", listenErr)
	}

	resp := chatgptStartResponse{
		AuthURL:   prepared.AuthURL,
		State:     prepared.State,
		FlowID:    flow.flowID,
		Port:      auth.ChatGPTCallbackPort(),
		ExpiresIn: int(chatgptFlowTTL / time.Second),
	}
	if listener == nil {
		resp.ManualPasteRequired = true
		if listenErr != nil {
			resp.PortBindError = listenErr.Error()
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleChatGPTManualCallback completes a pending OAuth flow when the
// user pastes the redirected URL (or just the `code` parameter) back
// into Biene because the local 1455 listener couldn't be opened. The
// state is matched against an in-flight flow; on success the manager
// lands the same SetFromCode → broadcast path used by the listener.
//
// We accept three input shapes for the `code` field:
//   - The full redirect URL ("http://localhost:1455/auth/callback?code=…&state=…")
//   - The query fragment alone ("code=…&state=…")
//   - Just the bare code ("…")
//
// pi-coding-agent's parseAuthorizationInput uses the same heuristic.
// The state can be supplied either inside the pasted URL or
// alongside it as the `state` field; explicit-state-with-bare-code is
// the common path when the renderer already has the state from the
// /start response.
func (s *Server) handleChatGPTManualCallback(w http.ResponseWriter, r *http.Request) {
	if err := requireLocalOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if s.chatgptOAuth == nil {
		writeError(w, http.StatusServiceUnavailable, "chatgpt oauth not initialised")
		return
	}
	c := s.chatgptOAuth

	var req struct {
		State string `json:"state"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	parsedCode, parsedState := parseAuthorizationInput(req.Code)
	if parsedCode != "" {
		req.Code = parsedCode
	}
	// The pasted URL's state, when present, takes precedence — it's
	// what the upstream actually round-tripped, so an explicit `state`
	// field elsewhere in the body should agree with it.
	if parsedState != "" {
		if req.State != "" && req.State != parsedState {
			writeError(w, http.StatusBadRequest, "state mismatch between body field and pasted URL")
			return
		}
		req.State = parsedState
	}
	if req.Code == "" || req.State == "" {
		writeError(w, http.StatusBadRequest, "code and state are required")
		return
	}

	c.mu.Lock()
	flow, ok := c.flows[req.State]
	if ok {
		// Remove the entry up-front so a duplicate paste can't run the
		// exchange twice. Cancel the flow context to clean up the
		// timeout goroutine.
		delete(c.flows, req.State)
		if flow.cancel != nil {
			flow.cancel()
		}
	}
	c.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "no pending flow for that state — restart login")
		return
	}

	exchangeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.auth.SetFromCode(exchangeCtx, req.Code, flow.codeVerifier); err != nil {
		// SetFromCode already records the error for the status poll;
		// surface it directly here as well so the renderer doesn't
		// have to round-trip.
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.onAuthChanged != nil {
		c.onAuthChanged()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// parseAuthorizationInput extracts the OAuth `code` (and optional
// `state`) from a free-form pasted string. Returns ("", "") when the
// input doesn't look like any recognised form so callers can decide
// whether to error or fall through.
//
// Mirrors pi-coding-agent's heuristic: try as full URL first, then as
// "code#state", then as a "code=…&state=…" query fragment, finally
// as a bare token.
func parseAuthorizationInput(raw string) (code, state string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", ""
	}
	// Full URL: http(s)://…?code=…&state=…
	if u, err := url.Parse(v); err == nil && u.Scheme != "" {
		q := u.Query()
		return q.Get("code"), q.Get("state")
	}
	// "code#state" — some OAuth providers use this shape on the
	// hash fragment.
	if idx := strings.Index(v, "#"); idx >= 0 {
		return v[:idx], v[idx+1:]
	}
	// "code=…&state=…" without a scheme.
	if strings.Contains(v, "code=") {
		q, err := url.ParseQuery(v)
		if err == nil {
			return q.Get("code"), q.Get("state")
		}
	}
	// Bare code; caller supplies state out of band.
	return v, ""
}

func (c *chatgptOAuthCoordinator) completeFlowFromCallback(
	ctx context.Context,
	listener *auth.ChatGPTCallbackListener,
	flow *pendingChatGPTFlow,
) {
	defer func() {
		c.mu.Lock()
		delete(c.flows, flow.state)
		c.mu.Unlock()
		if flow.cancel != nil {
			flow.cancel()
		}
	}()

	res, err := listener.Wait(ctx)
	if err != nil {
		// Listener closed before delivering — typically the flow TTL
		// expired or the user aborted. The renderer's poll loop sees
		// authenticated=false and surfaces a generic timeout.
		logging.Lifecycle().Warn("chatgpt callback wait", "err", err)
		return
	}
	if res.Error != "" {
		logging.Lifecycle().Warn("chatgpt oauth provider error", "error", res.Error)
		c.auth.SetLastAuthError(res.Error)
		return
	}
	if res.Code == "" || res.State == "" {
		c.auth.SetLastAuthError("oauth provider returned no authorization code")
		return
	}
	if res.State != flow.state {
		// State mismatch is a security boundary, not a UX one — refuse
		// the exchange and tell the user explicitly so they can retry
		// from a clean slate.
		c.auth.SetLastAuthError("oauth state mismatch — restart the login flow")
		return
	}
	exchangeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.auth.SetFromCode(exchangeCtx, res.Code, flow.codeVerifier); err != nil {
		// SetFromCode already records the error; just log for
		// post-mortem diagnostics.
		logging.Lifecycle().Error("chatgpt oauth token exchange", "err", err)
		return
	}
	if c.onAuthChanged != nil {
		c.onAuthChanged()
	}
}

func (s *Server) handleChatGPTStatus(w http.ResponseWriter, _ *http.Request) {
	if s.chatgptAuth == nil {
		writeJSON(w, http.StatusOK, auth.ChatGPTPublicStatus{Authenticated: false})
		return
	}
	writeJSON(w, http.StatusOK, s.chatgptAuth.Snapshot())
}

// chatgptUsageResponse is the JSON shape returned by
// GET /api/auth/chatgpt/usage. The boolean `available` distinguishes
// "no data yet, send a message first" (false) from "we have a snapshot"
// (true), so the renderer can render an empty state without having to
// nil-check every nested field.
type chatgptUsageResponse struct {
	Available bool                    `json:"available"`
	Snapshot  *auth.RateLimitSnapshot `json:"snapshot,omitempty"`
}

// handleChatGPTUsage returns the most recent rate-limit snapshot
// captured from a Codex /responses turn. Unlike the earlier active
// implementation, this handler never reaches out to upstream — the
// snapshot is populated as a side-effect of the user's normal
// chatting via api.ChatGPTCodexProvider. That avoids Cloudflare's
// bot manager on /api/codex/usage entirely, at the cost of needing
// at least one prior turn before the panel has data.
//
// Returns 401 when the user isn't signed in (matches the renderer's
// existing "no token, prompt to authorize" branch). Otherwise 200
// with `available: false` when no snapshot has landed yet, or
// `available: true` plus the snapshot.
func (s *Server) handleChatGPTUsage(w http.ResponseWriter, _ *http.Request) {
	if s.chatgptAuth == nil || !s.chatgptAuth.Authenticated() {
		writeError(w, http.StatusUnauthorized, auth.ErrChatGPTNotAuthenticated.Error())
		return
	}
	snap := s.chatgptAuth.RateLimits()
	if snap == nil {
		writeJSON(w, http.StatusOK, chatgptUsageResponse{Available: false})
		return
	}
	writeJSON(w, http.StatusOK, chatgptUsageResponse{Available: true, Snapshot: snap})
}

func (s *Server) handleChatGPTLogout(w http.ResponseWriter, r *http.Request) {
	if err := requireLocalOrigin(r); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if s.chatgptAuth == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if err := s.chatgptAuth.Logout(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Tell every chatgpt_official agent to re-emit its meta — they
	// just flipped to model_available=false and the renderer needs to
	// know so it can grey out the cards and disable composers.
	if s.mgr != nil {
		s.mgr.BroadcastChatGPTAuthChanged()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChatGPTCancel(w http.ResponseWriter, r *http.Request) {
	if s.chatgptOAuth == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	var req struct {
		State string `json:"state"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.State == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	c := s.chatgptOAuth
	c.mu.Lock()
	if flow, ok := c.flows[req.State]; ok {
		if flow.cancel != nil {
			flow.cancel()
		}
		delete(c.flows, req.State)
	}
	c.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── helpers ───────────────────────────────────────────────────────────

func generateFlowID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failures are essentially impossible on supported
		// platforms; falling back to a timestamp keeps requests serving.
		return "flow-" + time.Now().Format("20060102-150405.000000000")
	}
	return hex.EncodeToString(buf)
}
