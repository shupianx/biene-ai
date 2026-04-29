package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"biene/internal/api"
	"biene/internal/auth"
	"biene/internal/config"
	"biene/internal/permission/webperm"
	"biene/internal/processes"
	"biene/internal/prompt"
	"biene/internal/skills"
	"biene/internal/store"
	"biene/internal/tools"
	"biene/internal/tools/builtins"
)

// SessionManager holds all active agent sessions.
type SessionManager struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	workspaceRoot string // absolute path of the workspace directory
	cfg           *config.Config
	chatgptAuth   *auth.ChatGPTManager
	subscribers   map[int]chan ManagerFrame

	subscribersMu    sync.RWMutex
	nextSubscriberID int
}

// NewSessionManager creates a manager with the given workspace root.
func NewSessionManager(workspaceRoot string, cfg *config.Config) *SessionManager {
	rootAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		rootAbs = workspaceRoot
	}
	return &SessionManager{
		sessions:      make(map[string]*Session),
		workspaceRoot: rootAbs,
		cfg:           cfg,
		subscribers:   make(map[int]chan ManagerFrame),
	}
}

// SetChatGPTAuth wires the ChatGPT OAuth manager into session creation.
// Called by the server during startup; nil disables the synthetic
// "ChatGPT (official)" provider entirely.
//
// In practice this is set exactly once at startup (server.New) and
// never reassigned afterwards. We still go through the mutex so a
// future feature ("switch ChatGPT account at runtime") doesn't
// quietly become a data race.
func (m *SessionManager) SetChatGPTAuth(mgr *auth.ChatGPTManager) {
	m.mu.Lock()
	m.chatgptAuth = mgr
	m.mu.Unlock()
}

// ChatGPTAuth returns the OAuth manager wired into this session manager,
// or nil if the server hasn't installed one. **All reads of
// m.chatgptAuth must go through this accessor** — direct field reads
// from helper methods (newProvider, modelAvailabilityChecker,
// BroadcastChatGPTAuthChanged, …) historically slipped past the lock.
func (m *SessionManager) ChatGPTAuth() *auth.ChatGPTManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chatgptAuth
}

// newProvider creates an api.Provider from a model config entry.
func (m *SessionManager) newProvider(entry config.ModelEntry) api.Provider {
	if entry.Provider == "chatgpt_official" {
		// Routes through the Codex backend (chatgpt.com/backend-api),
		// the same path Codex CLI uses. The auth manager handles
		// access_token refresh and account_id extraction; the provider
		// re-asks per Stream() so rotated tokens take effect without
		// recreating the session.
		if mgr := m.ChatGPTAuth(); mgr != nil {
			return api.NewChatGPTCodexProvider(mgr, entry.Model)
		}
		// Fallback only triggers if a stale persisted session names
		// chatgpt_official after the OAuth manager failed to load
		// (corrupt tokens file, etc.). Returning an ErrorProvider
		// surfaces "not signed in to ChatGPT" verbatim rather than a
		// generic OpenAI 401, which is what an empty-key OpenAI
		// provider used to produce. resolveModelEntry usually catches
		// this earlier; this is the belt to its suspenders.
		return api.NewErrorProvider(
			"chatgpt-official/"+entry.Model,
			auth.ErrChatGPTNotAuthenticated,
		)
	}
	switch entry.Provider {
	case "openai_compatible", "openai":
		return api.NewOpenAIProvider(entry.APIKey, entry.Model, entry.BaseURL)
	default:
		return api.NewAnthropicProvider(entry.APIKey, entry.Model, entry.BaseURL)
	}
}

// Init scans the workspace root and rehydrates any sessions that have a
// persisted .biene/meta.json. It is called once at server startup.
func (m *SessionManager) Init() {
	entries, err := os.ReadDir(m.workspaceRoot)
	if err != nil {
		return // workspace may not exist yet
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "sess_") {
			continue
		}
		id := entry.Name()
		workDir := filepath.Join(m.workspaceRoot, id)
		storeDir := filepath.Join(workDir, ".biene")

		if !store.MetaExists(storeDir) {
			continue
		}

		sess, err := m.loadSessionFromDisk(id, workDir)
		if err != nil {
			slog.Error("init: load session", "session_id", id, "err", err)
			continue
		}

		m.mu.Lock()
		m.sessions[id] = sess
		m.mu.Unlock()
	}
}

// loadSessionFromDisk hydrates a Session from its on-disk meta + history.
// Used by Init() to re-attach existing sessions on startup, and by
// Fork() to attach the freshly-copied session that Fork wrote to disk.
//
// The returned Session is fully wired (provider, registry, checker,
// process controller, callbacks) but NOT registered in m.sessions —
// that's the caller's job, since Init runs without holding the lock
// for each session and Fork wants to enforce its own ordering.
func (m *SessionManager) loadSessionFromDisk(id, workDir string) (*Session, error) {
	storeDir := filepath.Join(workDir, ".biene")
	st, err := store.Open(storeDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	var meta SessionMeta
	if err := st.LoadMeta(&meta); err != nil {
		st.Close()
		return nil, fmt.Errorf("load meta: %w", err)
	}
	if meta.PendingPermission != nil {
		meta.PendingPermission.Expired = true
		if err := st.SaveMeta(meta); err != nil {
			slog.Error("save expired permission state", "session_id", id, "err", err)
		}
	}

	// Load display history; clear any streaming-in-progress flags.
	rawDisplay, err := st.LoadDisplayMessages()
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("load display: %w", err)
	}
	history := make([]DisplayMessage, 0, len(rawDisplay))
	for _, raw := range rawDisplay {
		var msg DisplayMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		msg.Streaming = false
		history = append(history, msg)
	}

	// Load API messages.
	rawAPI, err := st.LoadAPIMessages()
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("load api msgs: %w", err)
	}
	apiMsgs := make([]api.Message, 0, len(rawAPI))
	for _, raw := range rawAPI {
		msg, err := unmarshalAPIMessage(raw)
		if err != nil {
			continue
		}
		apiMsgs = append(apiMsgs, msg)
	}

	perms := meta.Permissions
	profile := normalizeProfile(meta.Profile)
	toolMode := defaultToolModeForProfile(profile)

	// Rebuild provider / registry / checker from the pinned model selection.
	modelEntry, resolvedModelID, err := m.resolveModelEntry(m.cfg, meta.ModelID)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("resolve model: %w", err)
	}
	if meta.ModelID != resolvedModelID || meta.ModelName != modelEntry.Name {
		meta.ModelID = resolvedModelID
		meta.ModelName = modelEntry.Name
		if err := st.SaveMeta(meta); err != nil {
			slog.Error("save normalized model state", "session_id", id, "err", err)
		}
	}
	thinkingAvailable := modelEntry.ThinkingAvailable
	thinkingEnabled := thinkingAvailable
	if meta.ThinkingAvailable {
		thinkingEnabled = thinkingAvailable && meta.ThinkingEnabled
	}
	if meta.ThinkingAvailable != thinkingAvailable || meta.ThinkingEnabled != thinkingEnabled {
		meta.ThinkingAvailable = thinkingAvailable
		meta.ThinkingEnabled = thinkingEnabled
		if err := st.SaveMeta(meta); err != nil {
			slog.Error("save thinking state", "session_id", id, "err", err)
		}
	}
	// avatar is now a frontend-owned key transported through
	// SessionMeta.Extras (see CLAUDE.md "Schema 设计准则"). The
	// backend doesn't generate it, validate it, or backfill it —
	// the renderer takes over both choosing and persisting it.
	provider := m.newProvider(modelEntry)
	registry := builtins.RegistryForWorkDir(workDir)
	checker := webperm.NewChecker(perms)
	procCtl := processes.New(workDir)
	registry.Register(builtins.NewListAgentsTool(m, id))
	registry.Register(builtins.NewSendMessageToAgentTool(m, id))
	registry.Register(builtins.NewCoworkWithAgentTool(m, id))
	registry.Register(builtins.NewEndCoworkWithAgentTool(m, id))
	registry.Register(builtins.NewListCoworksTool(m, id))
	registry.Register(builtins.NewStartProcessTool(workDir, procCtl))
	registry.Register(builtins.NewReadProcessOutputTool(procCtl))
	registry.Register(builtins.NewStopProcessTool(procCtl))

	installedIDs, scanErr := skills.InstalledSkillIDsForWorkDir(workDir)
	if scanErr != nil {
		slog.Error("scan installed skills for", "session_id", id, "err", scanErr)
	}
	sess := &Session{
		ID:                id,
		Name:              meta.Name,
		WorkDir:           meta.WorkDir,
		Status:            StatusIdle,
		permissions:       perms,
		profile:           profile,
		toolMode:          toolMode,
		CreatedAt:         meta.CreatedAt,
		LastActive:        meta.LastActive,
		provider:          provider,
		registry:          registry,
		checker:           checker,
		modelID:           resolvedModelID,
		modelName:         modelEntry.Name,
		thinkingAvailable: thinkingAvailable,
		thinkingEnabled:   thinkingEnabled,
		thinkingOn:        modelEntry.ThinkingOn,
		thinkingOff:       modelEntry.ThinkingOff,
		imagesAvailable:   resolveImagesAvailable(modelEntry),
		contextWindow:     modelEntry.ContextWindow,
		serviceTier:       modelEntry.ServiceTier,
		extras:            cloneExtras(meta.Extras),
		activeSkills:      append([]string(nil), meta.ActiveSkills...),
		installedSkillIDs: installedIDs,
		coworksGranted:    append([]GrantedCowork(nil), meta.CoworksGranted...),
		apiMessages:       apiMsgs,
		history:           history,
		pendingPermission: clonePermissionPayload(meta.PendingPermission),
		persistedCount:    len(history),
		processes:         procCtl,
		subscribers:       make(map[int]chan Frame),
		store:             st,
	}
	registry.Register(builtins.NewUseSkillTool(sess))
	sess.systemPrompt = prompt.Build(registry, workDir, profile, prompt.AgentIdentity{
		ID:      id,
		Name:    meta.Name,
		WorkDir: workDir,
	}, nil)
	sess.configProvider = m.snapshotConfig
	sess.modelAvailableProvider = m.modelAvailabilityChecker(sess)
	sess.onMetaChanged = m.emitSessionUpdated
	sess.onProcessStateChanged = m.emitSessionProcessState
	checker.OnPermissionNeeded = func(req webperm.PermissionRequest) {
		payload := sess.setPendingPermission(req)
		sess.send(makeFrame("permission_request", payload))
	}
	checker.OnPermissionSettled = sess.clearPendingPermission
	checker.OnPermissionsChanged = sess.persistPermissions
	sess.startProcessWatcher()

	return sess, nil
}

// Create allocates a new session with its own workspace directory.
//
// `extras` carries any frontend-owned keys (the renderer's `avatar`
// pick is the canonical example) that should land in meta.json. The
// backend never inspects them — see CLAUDE.md "Schema 设计准则".
func (m *SessionManager) Create(name string, permissions tools.PermissionSet, profile prompt.AgentProfile, modelID string, extras store.Extras) (*Session, error) {
	id := newSessionID()
	workDir := filepath.Join(m.workspaceRoot, id)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}
	if err := skills.InstallDefaultEnabled(workDir); err != nil {
		return nil, fmt.Errorf("installing default-enabled skills: %w", err)
	}
	profile = normalizeProfile(profile)
	toolMode := defaultToolModeForProfile(profile)

	modelEntry, resolvedModelID, err := m.resolveModelEntry(m.cfg, modelID)
	if err != nil {
		return nil, err
	}

	provider := m.newProvider(modelEntry)
	registry := builtins.RegistryForWorkDir(workDir)
	checker := webperm.NewChecker(permissions)
	procCtl := processes.New(workDir)

	registry.Register(builtins.NewListAgentsTool(m, id))
	registry.Register(builtins.NewSendMessageToAgentTool(m, id))
	registry.Register(builtins.NewCoworkWithAgentTool(m, id))
	registry.Register(builtins.NewEndCoworkWithAgentTool(m, id))
	registry.Register(builtins.NewListCoworksTool(m, id))
	registry.Register(builtins.NewStartProcessTool(workDir, procCtl))
	registry.Register(builtins.NewReadProcessOutputTool(procCtl))
	registry.Register(builtins.NewStopProcessTool(procCtl))

	installedIDs, scanErr := skills.InstalledSkillIDsForWorkDir(workDir)
	if scanErr != nil {
		slog.Error("scan installed skills for", "session_id", id, "err", scanErr)
	}

	now := time.Now()
	sess := &Session{
		ID:                id,
		Name:              name,
		WorkDir:           workDir,
		Status:            StatusIdle,
		permissions:       permissions,
		profile:           profile,
		toolMode:          toolMode,
		CreatedAt:         now,
		LastActive:        now,
		provider:          provider,
		registry:          registry,
		checker:           checker,
		modelID:           resolvedModelID,
		modelName:         modelEntry.Name,
		thinkingAvailable: modelEntry.ThinkingAvailable,
		thinkingEnabled:   false,
		thinkingOn:        modelEntry.ThinkingOn,
		thinkingOff:       modelEntry.ThinkingOff,
		imagesAvailable:   resolveImagesAvailable(modelEntry),
		contextWindow:     modelEntry.ContextWindow,
		serviceTier:       modelEntry.ServiceTier,
		extras:            cloneExtras(extras),
		installedSkillIDs: installedIDs,
		apiMessages:       []api.Message{},
		history:           []DisplayMessage{},
		pendingPermission: nil,
		processes:         procCtl,
		subscribers:       make(map[int]chan Frame),
	}
	registry.Register(builtins.NewUseSkillTool(sess))
	sess.systemPrompt = prompt.Build(registry, workDir, profile, prompt.AgentIdentity{
		ID:      id,
		Name:    name,
		WorkDir: workDir,
	}, nil)
	sess.configProvider = m.snapshotConfig
	sess.modelAvailableProvider = m.modelAvailabilityChecker(sess)
	sess.onMetaChanged = m.emitSessionUpdated
	sess.onProcessStateChanged = m.emitSessionProcessState
	checker.OnPermissionNeeded = func(req webperm.PermissionRequest) {
		payload := sess.setPendingPermission(req)
		sess.send(makeFrame("permission_request", payload))
	}
	checker.OnPermissionSettled = sess.clearPendingPermission
	checker.OnPermissionsChanged = sess.persistPermissions
	sess.startProcessWatcher()

	storeDir := filepath.Join(workDir, ".biene")
	if st, err := store.Open(storeDir); err != nil {
		slog.Error("open store for", "session_id", id, "err", err)
	} else {
		sess.store = st
		sess.mu.Lock()
		initialMeta := sess.persistentMetaLocked()
		sess.mu.Unlock()
		if err := st.SaveMeta(initialMeta); err != nil {
			slog.Error("save initial meta for", "session_id", id, "err", err)
		}
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	m.emitSessionCreated(sess.Meta())

	return sess, nil
}

// Get returns the session by ID, or nil.
func (m *SessionManager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// ActiveBackgroundProcess describes one running background process along with
// the session it belongs to. Used for the app-quit confirmation prompt.
type ActiveBackgroundProcess struct {
	SessionID   string   `json:"session_id"`
	SessionName string   `json:"session_name"`
	Command     string   `json:"command"`
	Args        []string `json:"args,omitempty"`
	PID         int      `json:"pid,omitempty"`
}

// ActiveBackgroundProcesses returns every session that currently has a
// running background process.
func (m *SessionManager) ActiveBackgroundProcesses() []ActiveBackgroundProcess {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	var out []ActiveBackgroundProcess
	for _, sess := range sessions {
		st := sess.ProcessState()
		if !st.Active {
			continue
		}
		out = append(out, ActiveBackgroundProcess{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			Command:     st.Command,
			Args:        st.Args,
			PID:         st.PID,
		})
	}
	return out
}

func (m *SessionManager) NameTaken(name, excludeID string) bool {
	normalized := normalizeAgentName(name)
	if normalized == "" {
		return false
	}

	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for id, sess := range m.sessions {
		if excludeID != "" && id == excludeID {
			continue
		}
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	for _, sess := range sessions {
		sess.mu.Lock()
		n := sess.Name
		sess.mu.Unlock()
		if normalizeAgentName(n) == normalized {
			return true
		}
	}
	return false
}

// List returns metadata for all sessions, ordered by creation time.
func (m *SessionManager) List() []SessionMeta {
	// Snapshot the session pointers under the manager lock, then read
	// per-session state outside the lock to avoid nested locking.
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.RUnlock()

	out := make([]SessionMeta, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, s.Meta())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// ListAgents exposes the sessions to tools.AgentDirectory.
func (m *SessionManager) ListAgents(excludeID string) []tools.AgentPeer {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		if sess.ID != excludeID {
			sessions = append(sessions, sess)
		}
	}
	m.mu.RUnlock()

	out := make([]tools.AgentPeer, 0, len(sessions))
	for _, sess := range sessions {
		meta := sess.Meta()
		out = append(out, tools.AgentPeer{
			ID:      meta.ID,
			Name:    meta.Name,
			WorkDir: meta.WorkDir,
			Status:  string(meta.Status),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// DeliverFromAgent exposes peer-to-peer delivery to tools.AgentDirectory.
func (m *SessionManager) DeliverFromAgent(ctx context.Context, fromAgentID string, req tools.DeliveryRequest) (tools.DeliveryResult, error) {
	fromSess := m.Get(fromAgentID)
	if fromSess == nil {
		return tools.DeliveryResult{}, fmt.Errorf("source agent %q not found", fromAgentID)
	}
	toSess := m.Get(req.TargetAgentID)
	if toSess == nil {
		return tools.DeliveryResult{}, fmt.Errorf("target agent %q not found", req.TargetAgentID)
	}
	if fromAgentID == req.TargetAgentID {
		return tools.DeliveryResult{}, fmt.Errorf("cannot send to the same agent")
	}

	messageMeta := fromSess.prepareOutboundAgentDelivery(req.TargetAgentID)

	outcome, err := copyFilesBetweenWorkspaces(ctx, fromSess.WorkDir, toSess.WorkDir, fromSess.ID, req.FilePaths, req.CollisionStrategy)
	if err != nil {
		return tools.DeliveryResult{}, err
	}

	toSess.enqueueAgentInput(fromSess.ID, fromSess.Name, req.Message, outcome.Attachments, messageMeta)
	return tools.DeliveryResult{
		TargetID:    toSess.ID,
		TargetName:  toSess.Name,
		StoredPaths: attachmentPaths(outcome.Attachments),
		Skipped:     outcome.Skipped,
		Overwritten: outcome.Overwritten,
		Renamed:     outcome.Renamed,
		MessageMeta: messageMeta,
	}, nil
}

// DetectFileCollisions reports which requested source files would clash with
// existing names in the target agent's inbox folder. Called before the sender
// requests permission so the UI can surface conflicts to the user.
func (m *SessionManager) DetectFileCollisions(fromAgentID, targetAgentID string, filePaths []string) ([]tools.FileCollision, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}
	fromSess := m.Get(fromAgentID)
	if fromSess == nil {
		return nil, fmt.Errorf("source agent %q not found", fromAgentID)
	}
	toSess := m.Get(targetAgentID)
	if toSess == nil {
		return nil, fmt.Errorf("target agent %q not found", targetAgentID)
	}
	return detectAgentInboxCollisions(fromSess.WorkDir, toSess.WorkDir, fromSess.ID, filePaths)
}

// Delete cancels the session's query, removes it from the manager,
// and deletes its workspace directory from disk.
func (m *SessionManager) Delete(id string) bool {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		sess.mu.Lock()
		grants := append([]GrantedCowork(nil), sess.coworksGranted...)
		sess.mu.Unlock()
		m.cleanupCoworksForDeletedSession(id, grants)

		m.emitSessionDeleted(id)
		sess.close()
		if sess.store != nil {
			sess.store.Close()
		}
		os.RemoveAll(sess.WorkDir)
	}
	return ok
}
