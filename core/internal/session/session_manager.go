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

// newProvider creates an api.Provider from a model config entry.
func newProvider(entry config.ModelEntry) api.Provider {
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

		st, err := store.Open(storeDir)
		if err != nil {
			slog.Error("init: open store", "session_id", id, "err", err)
			continue
		}

		var meta SessionMeta
		if err := st.LoadMeta(&meta); err != nil {
			slog.Error("init: load meta", "session_id", id, "err", err)
			st.Close()
			continue
		}
		if meta.PendingPermission != nil {
			meta.PendingPermission.Expired = true
			if err := st.SaveMeta(meta); err != nil {
				slog.Error("init: save expired permission state", "session_id", id, "err", err)
			}
		}

		// Load display history; clear any streaming-in-progress flags.
		rawDisplay, err := st.LoadDisplayMessages()
		if err != nil {
			slog.Error("init: load display", "session_id", id, "err", err)
			st.Close()
			continue
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
			slog.Error("init: load api msgs", "session_id", id, "err", err)
			st.Close()
			continue
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
		modelEntry, resolvedModelID, err := resolveModelEntry(m.cfg, meta.ModelID)
		if err != nil {
			slog.Error("init: get model for", "session_id", id, "err", err)
			st.Close()
			continue
		}
		if meta.ModelID != resolvedModelID || meta.ModelName != modelEntry.Name {
			meta.ModelID = resolvedModelID
			meta.ModelName = modelEntry.Name
			if err := st.SaveMeta(meta); err != nil {
				slog.Error("init: save normalized model state", "session_id", id, "err", err)
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
				slog.Error("init: save thinking state", "session_id", id, "err", err)
			}
		}
		provider := newProvider(modelEntry)
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
		sess.onMetaChanged = m.emitSessionUpdated
		sess.onProcessStateChanged = m.emitSessionProcessState
		checker.OnPermissionNeeded = func(req webperm.PermissionRequest) {
			payload := sess.setPendingPermission(req)
			sess.send(makeFrame("permission_request", payload))
		}
		checker.OnPermissionSettled = sess.clearPendingPermission
		checker.OnPermissionsChanged = sess.persistPermissions
		sess.startProcessWatcher()

		m.mu.Lock()
		m.sessions[id] = sess
		m.mu.Unlock()
	}
}

// Create allocates a new session with its own workspace directory.
func (m *SessionManager) Create(name string, permissions tools.PermissionSet, profile prompt.AgentProfile, modelID string) (*Session, error) {
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

	modelEntry, resolvedModelID, err := resolveModelEntry(m.cfg, modelID)
	if err != nil {
		return nil, err
	}

	provider := newProvider(modelEntry)
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
