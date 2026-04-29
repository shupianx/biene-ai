package session

import (
	"log/slog"
	"os"
	"strings"

	"biene/internal/auth"
	"biene/internal/config"
)

// chatgptServiceTierEnv is the env-var override for the ChatGPT
// official entry's `service_tier`. Recognised values match the OpenAI
// Codex backend's enum: "default" / "flex" / "priority" / "scale" /
// "auto". An empty / unset value means "no override" — the upstream
// picks its own default. Exposed as an env var (rather than UI) for
// now because the right tradeoff between latency and cost varies by
// account plan and is rarely changed once set.
const chatgptServiceTierEnv = "BIENE_CHATGPT_SERVICE_TIER"

// validCodexServiceTiers enumerates the strings the Codex backend
// accepts for `service_tier`. Anything outside this set is dropped on
// read so a typo in the env var can't 400 every Codex turn.
var validCodexServiceTiers = map[string]struct{}{
	"default":  {},
	"flex":     {},
	"priority": {},
	"scale":    {},
	"auto":     {},
}

func resolveChatGPTServiceTier() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(chatgptServiceTierEnv)))
	if v == "" {
		return ""
	}
	if _, ok := validCodexServiceTiers[v]; !ok {
		slog.Warn("ignoring unknown chatgpt service tier",
			"env", chatgptServiceTierEnv, "value", v)
		return ""
	}
	return v
}

// resolveImagesAvailable applies the default-true semantics for
// ModelEntry.ImagesAvailable (nil → true).
func resolveImagesAvailable(entry config.ModelEntry) bool {
	if entry.ImagesAvailable == nil {
		return true
	}
	return *entry.ImagesAvailable
}

// resolveModelEntry maps a stored model_id back to the configuration
// that drives provider construction. The lookup is layered:
//
//  1. Synthetic "chatgpt_official:<model>" IDs are constructed in-memory
//     against the OAuth-authenticated user's account. They never appear
//     in cfg.ModelList — the auth state is the source of truth.
//  2. User-defined named configs come from cfg.ModelList directly.
//  3. Anything that fails to resolve (legacy ID, deleted config, OAuth
//     logout) falls back to the default model.
func (m *SessionManager) resolveModelEntry(cfg *config.Config, requestedID string) (config.ModelEntry, string, error) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID != "" {
		if auth.IsChatGPTOfficialModelID(requestedID) {
			// Synthetic entries are returned regardless of OAuth
			// state — the session stays pinned and Session.metaLocked
			// surfaces ModelAvailable=false to the renderer when the
			// user is logged out (handler_chat then refuses Send).
			// Rebinding to the default model would silently switch a
			// "ChatGPT" agent to Anthropic and is not what the user
			// expects after a revoke. The only way `ok` is false here
			// is a malformed id (no model component after the colon),
			// which falls through to the regular cfg.GetModel lookup
			// below and ultimately to the default-model fallback.
			if entry, ok := m.chatgptOfficialEntry(requestedID); ok {
				return entry, requestedID, nil
			}
		}
		if entry, err := cfg.GetModel(requestedID); err == nil {
			return entry, entry.ID, nil
		}
	}

	entry, err := cfg.GetModel("")
	if err != nil {
		return config.ModelEntry{}, "", err
	}
	return entry, entry.ID, nil
}

// BroadcastChatGPTAuthChanged tells every session whose model is
// chatgpt_official to re-emit its meta. The model_id stays the same
// but ModelAvailable flips to match the current OAuth state, so
// connected renderers update grid opacity and chat composer state in
// real time without polling.
//
// Called by the OAuth handler after a successful login or logout.
func (m *SessionManager) BroadcastChatGPTAuthChanged() {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	for _, sess := range sessions {
		sess.mu.Lock()
		modelID := sess.modelID
		sess.mu.Unlock()
		if !auth.IsChatGPTOfficialModelID(modelID) {
			continue
		}
		// Recompute under the meta lock and push to subscribers.
		sess.notifyMetaChanged(sess.Meta())
	}
}

// modelAvailabilityChecker builds a per-session callback the Session
// uses to compute SessionMeta.ModelAvailable. Most providers are
// always available; chatgpt_official depends on the OAuth manager's
// current state.
//
// We capture the modelID at wire time rather than reading
// sess.modelID under the lock — modelID is stable for the lifetime
// of a session (it only changes via UpdateConfig, which redoes the
// whole wiring), and avoiding the lock here means the meta builder
// doesn't deadlock when it's already holding s.mu.
func (m *SessionManager) modelAvailabilityChecker(sess *Session) func() bool {
	modelID := sess.modelID
	if !auth.IsChatGPTOfficialModelID(modelID) {
		return func() bool { return true }
	}
	return func() bool {
		mgr := m.ChatGPTAuth()
		if mgr == nil {
			return false
		}
		return mgr.Authenticated()
	}
}

// chatgptOfficialEntry returns a synthetic ModelEntry that drives the
// OAuth provider path. The entry has no real API key — newProvider is
// responsible for routing it to api.NewChatGPTOAuthProvider, which
// pulls a fresh key from the auth manager on every Stream call.
//
// We return the entry regardless of current auth state. A logged-out
// session keeps its model_id pinned, surfaces a model_available=false
// flag in its meta (see Session.modelAvailableLocked), and the chat
// send handler refuses new turns. Silently rebinding to the default
// model would have the wrong UX: the user expects "this agent is
// stuck" not "this agent quietly switched to Anthropic Claude".
func (m *SessionManager) chatgptOfficialEntry(modelID string) (config.ModelEntry, bool) {
	model := auth.ParseChatGPTOfficialModelID(modelID)
	if model == "" {
		return config.ModelEntry{}, false
	}
	return config.ModelEntry{
		ID:       modelID,
		Name:     "ChatGPT · " + model,
		Provider: "chatgpt_official",
		Model:    model,
		// ContextWindow has to be set explicitly — without it the
		// session falls back to the conservative 32K default and the
		// compaction policy fires before there's any history worth
		// cutting, surfacing a "no safe cut point" warning every turn.
		// auth.ChatGPTOfficialContextWindow returns 400K for the
		// GPT-5 family (matching the OpenAI Codex backend's actual
		// cap).
		ContextWindow: auth.ChatGPTOfficialContextWindow(model),
		// images_available is true: the Codex provider's
		// convertMessagesToResponsesInput routes user turns with
		// ImageBlocks through buildCodexUserMessageWithImages, which
		// inlines each attachment as a base64 `input_image` content
		// part. Set explicitly (instead of leaving nil for the
		// "default true") so the contract is visible at the
		// declaration site — flip back to ptrBool(false) if the
		// content-list path ever regresses.
		ImagesAvailable:   ptrBool(true),
		ThinkingAvailable: true,
		// thinking_on / thinking_off: the Codex provider reads the
		// "reasoning" fragment off RequestOptions.ThinkingExtra and
		// applies per-model clamping (see buildCodexReasoning in
		// api/chatgpt_codex.go). "high" + "auto summary" gives the
		// richest chain-of-thought when the user toggles thinking
		// on; "low" is the cheapest legal value across the GPT-5
		// family (gpt-5.2/3/4/5 reject "minimal", which is why we
		// don't use it as the off-state value).
		ThinkingOn: map[string]any{
			"reasoning": map[string]any{
				"effort":  "high",
				"summary": "auto",
			},
		},
		ThinkingOff: map[string]any{
			"reasoning": map[string]any{
				"effort":  "low",
				"summary": "auto",
			},
		},
		// service_tier: env-var override (BIENE_CHATGPT_SERVICE_TIER).
		// Default is "" → upstream chooses. Most accounts want "flex"
		// to halve cost on long-running agents at the price of slower
		// scheduling, but flipping that on by default would surprise
		// Pro users on Priority who expect snappy responses, so we
		// require an explicit opt-in.
		ServiceTier: resolveChatGPTServiceTier(),
	}, true
}

func ptrBool(v bool) *bool { return &v }

// snapshotConfig returns the current global config under the manager's
// read lock. Sessions consult this through their configProvider closure
// so reads stay race-free across UpdateConfig calls.
func (m *SessionManager) snapshotConfig() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *SessionManager) ModelUsageCounts() map[string]int {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	counts := make(map[string]int, len(sessions))
	for _, sess := range sessions {
		meta := sess.Meta()
		if meta.ModelID == "" {
			continue
		}
		counts[meta.ModelID]++
	}
	return counts
}

// UpdateConfig replaces the active runtime config and refreshes session providers
// for subsequent turns while preserving each agent's pinned model selection.
func (m *SessionManager) UpdateConfig(cfg *config.Config) error {
	if _, _, err := m.resolveModelEntry(cfg, ""); err != nil {
		return err
	}

	m.mu.Lock()
	m.cfg = cfg
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.Unlock()

	for _, sess := range sessions {
		sess.mu.Lock()
		currentModelID := sess.modelID
		sess.mu.Unlock()

		modelEntry, resolvedID, err := m.resolveModelEntry(cfg, currentModelID)
		if err != nil {
			return err
		}

		sess.mu.Lock()
		thinkingAvailable := modelEntry.ThinkingAvailable
		thinkingEnabled := thinkingAvailable
		if sess.thinkingAvailable {
			thinkingEnabled = thinkingAvailable && sess.thinkingEnabled
		}
		sess.provider = m.newProvider(modelEntry)
		sess.modelID = resolvedID
		sess.modelName = modelEntry.Name
		sess.thinkingAvailable = thinkingAvailable
		sess.thinkingEnabled = thinkingEnabled
		sess.thinkingOn = modelEntry.ThinkingOn
		sess.thinkingOff = modelEntry.ThinkingOff
		sess.imagesAvailable = resolveImagesAvailable(modelEntry)
		sess.contextWindow = modelEntry.ContextWindow
		sess.serviceTier = modelEntry.ServiceTier
		// modelID may have flipped between user/synthetic via the
		// resolver above; rebind the availability checker so future
		// metaLocked() reflects the new model's gate.
		sess.modelAvailableProvider = m.modelAvailabilityChecker(sess)
		meta := sess.metaLocked()
		persistedMeta := sess.persistentMetaLocked()
		sess.mu.Unlock()

		if sess.store != nil {
			if err := sess.store.SaveMeta(persistedMeta); err != nil {
				slog.Error("persist session meta after config update", "session_id", sess.ID, "err", err)
			}
		}
		sess.notifyMetaChanged(meta)
	}

	return nil
}
