package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"biene/internal/api"
	"biene/internal/config"
	"biene/internal/permission/webperm"
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
			log.Printf("init: open store %s: %v", id, err)
			continue
		}

		var meta SessionMeta
		if err := st.LoadMeta(&meta); err != nil {
			log.Printf("init: load meta %s: %v", id, err)
			st.Close()
			continue
		}
		if meta.PendingPermission != nil {
			meta.PendingPermission.Expired = true
			if err := st.SaveMeta(meta); err != nil {
				log.Printf("init: save expired permission state %s: %v", id, err)
			}
		}

		// Load display history; clear any streaming-in-progress flags.
		rawDisplay, err := st.LoadDisplayMessages()
		if err != nil {
			log.Printf("init: load display %s: %v", id, err)
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
			log.Printf("init: load api msgs %s: %v", id, err)
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
			log.Printf("init: get model for %s: %v", id, err)
			st.Close()
			continue
		}
		if meta.ModelID != resolvedModelID || meta.ModelName != modelEntry.Name {
			meta.ModelID = resolvedModelID
			meta.ModelName = modelEntry.Name
			if err := st.SaveMeta(meta); err != nil {
				log.Printf("init: save normalized model state %s: %v", id, err)
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
				log.Printf("init: save thinking state %s: %v", id, err)
			}
		}
		provider := newProvider(modelEntry)
		registry := builtins.RegistryForWorkDir(workDir)
		checker := webperm.NewChecker(perms)
		maxTokens := maxTokensFromConfig(m.cfg)
		registry.Register(builtins.NewListAgentsTool(m, id))
		registry.Register(builtins.NewSendToAgentTool(m, id))

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
			activeSkills:      append([]string(nil), meta.ActiveSkills...),
			maxTokens:         maxTokens,
			apiMessages:       apiMsgs,
			history:           history,
			pendingPermission: clonePermissionPayload(meta.PendingPermission),
			persistedCount:    len(history),
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
		checker.OnPermissionNeeded = func(req webperm.PermissionRequest) {
			payload := sess.setPendingPermission(req)
			sess.send(makeFrame("permission_request", payload))
		}
		checker.OnPermissionSettled = sess.clearPendingPermission
		checker.OnPermissionsChanged = sess.persistPermissions

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
	maxTokens := maxTokensFromConfig(m.cfg)

	registry.Register(builtins.NewListAgentsTool(m, id))
	registry.Register(builtins.NewSendToAgentTool(m, id))

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
		maxTokens:         maxTokens,
		apiMessages:       []api.Message{},
		history:           []DisplayMessage{},
		pendingPermission: nil,
		subscribers:       make(map[int]chan Frame),
	}
	registry.Register(builtins.NewUseSkillTool(sess))
	sess.systemPrompt = prompt.Build(registry, workDir, profile, prompt.AgentIdentity{
		ID:      id,
		Name:    name,
		WorkDir: workDir,
	}, nil)
	sess.onMetaChanged = m.emitSessionUpdated
	checker.OnPermissionNeeded = func(req webperm.PermissionRequest) {
		payload := sess.setPendingPermission(req)
		sess.send(makeFrame("permission_request", payload))
	}
	checker.OnPermissionSettled = sess.clearPendingPermission
	checker.OnPermissionsChanged = sess.persistPermissions

	storeDir := filepath.Join(workDir, ".biene")
	if st, err := store.Open(storeDir); err != nil {
		log.Printf("open store for %s: %v", id, err)
	} else {
		sess.store = st
		sess.mu.Lock()
		initialMeta := sess.persistentMetaLocked()
		sess.mu.Unlock()
		if err := st.SaveMeta(initialMeta); err != nil {
			log.Printf("save initial meta for %s: %v", id, err)
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

	attachments, err := copyFilesBetweenWorkspaces(ctx, fromSess.WorkDir, toSess.WorkDir, "inbox", fromSess.ID, req.FilePaths)
	if err != nil {
		return tools.DeliveryResult{}, err
	}

	toSess.enqueueAgentInput(fromSess.ID, fromSess.Name, req.Message, attachments, messageMeta)
	return tools.DeliveryResult{
		TargetID:    toSess.ID,
		TargetName:  toSess.Name,
		StoredPaths: attachmentPaths(attachments),
		MessageMeta: messageMeta,
	}, nil
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
		m.emitSessionDeleted(id)
		sess.close()
		if sess.store != nil {
			sess.store.Close()
		}
		os.RemoveAll(sess.WorkDir)
	}
	return ok
}
