package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"biene/internal/api"
	"biene/internal/permission/webperm"
	"biene/internal/processes"
	"biene/internal/prompt"
	"biene/internal/store"
	"biene/internal/tools"
)

// ── Core types ────────────────────────────────────────────────────────────

// SessionStatus reflects the current activity state of an agent.
type SessionStatus string

const (
	StatusIdle    SessionStatus = "idle"
	StatusRunning SessionStatus = "running"
	StatusError   SessionStatus = "error"
)

const (
	authorTypeUser  = "user"
	authorTypeAgent = "agent"
)

// DisplayAttachment is a file rendered alongside a chat message. Kind
// distinguishes regular file uploads (routed to inbox/) from inline images
// stored under .biene/assets/ and rendered as thumbnails.
type DisplayAttachment struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Kind      string `json:"kind,omitempty"` // "image" or "" (file)
	MediaType string `json:"media_type,omitempty"`
}

// DisplayReasoning stores a persisted reasoning trace for one assistant message.
type DisplayReasoning struct {
	Text       string    `json:"text"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms,omitempty"`
}

// DisplayTool mirrors a tool call in the display layer.
type DisplayTool struct {
	ToolID      string          `json:"tool_id,omitempty"`
	ToolName    string          `json:"tool_name"`
	ToolSummary string          `json:"tool_summary"`
	ToolInput   json.RawMessage `json:"tool_input,omitempty"`
	Status      string          `json:"status"` // composing|pending|done|error|denied|cancelled
	Result      string          `json:"result,omitempty"`
}

// DisplayMessage is the server-side render state of a single message.
type DisplayMessage struct {
	ID            string                  `json:"id"`
	Role          string                  `json:"role"` // user | assistant
	AuthorType    string                  `json:"author_type,omitempty"`
	AuthorID      string                  `json:"author_id,omitempty"`
	AuthorName    string                  `json:"author_name,omitempty"`
	AgentMeta     *tools.AgentMessageMeta `json:"agent_meta,omitempty"`
	Text          string                  `json:"text"`
	Streaming     bool                    `json:"streaming,omitempty"` // true while a response is in progress
	ToolCalls     []DisplayTool           `json:"tool_calls,omitempty"`
	Attachments   []DisplayAttachment     `json:"attachments,omitempty"`
	Reasoning     *DisplayReasoning       `json:"reasoning,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
}

type queuedInput struct {
	apiMessage api.Message
}

type ToolMode string

const (
	ToolModeAnswerOnly      ToolMode = "answer_only"
	ToolModeWorkspaceChange ToolMode = "workspace_change"
)

// GrantedCowork records one outgoing cowork relationship from this agent.
// The actual symlink lives in the target agent's workspace at
// cowork/<this-agent-id>/<basename(source_path)>; this record is the
// sender's side of the bookkeeping used for end_cowork and cleanup.
type GrantedCowork struct {
	TargetAgentID string    `json:"target_agent_id"`
	SourcePath    string    `json:"source_path"`
	CreatedAt     time.Time `json:"created_at"`
}

// Session is one agent instance with its own workspace and conversation history.
type Session struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	WorkDir    string        `json:"work_dir"` // absolute path
	Status     SessionStatus `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	LastActive time.Time     `json:"last_active"`

	provider          api.Provider
	registry          *tools.Registry
	checker           *webperm.Checker
	systemPrompt      string
	permissions       tools.PermissionSet
	profile           prompt.AgentProfile
	toolMode          ToolMode
	modelID           string
	modelName         string
	thinkingAvailable bool
	thinkingEnabled   bool
	thinkingOn        map[string]any
	thinkingOff       map[string]any

	// activeSkills tracks skills that have been loaded via use_skill during
	// this session. Names are unique and kept in activation order.
	activeSkills []string

	// installedSkillIDs is a cache of skill directory IDs present under
	// <WorkDir>/.biene/skills. It is refreshed from disk on install/uninstall
	// so the frontend can detect drag-and-drop name collisions locally.
	installedSkillIDs []string

	// coworksGranted tracks outgoing cowork relationships this agent has
	// created. The slice is persisted via SessionMeta so coworks survive
	// restarts.
	coworksGranted []GrantedCowork

	// pendingSystemNotes queues "something happened outside the agent's
	// loop that the agent should know next turn" strings. Today this is
	// only populated when the user manually stops the background process
	// via the UI button — the agent otherwise has no way to hear about
	// state changes it didn't cause. Drained (and cleared) when the
	// next user-authored input is enqueued.
	pendingSystemNotes []string

	// apiMessages is the canonical conversation passed into the next model turn.
	apiMessages []api.Message

	// pendingInputs are appended while a run is in flight, then processed as the next turn.
	pendingInputs []queuedInput

	// history is the server-side render state returned by GET /history.
	history []DisplayMessage
	// pendingPermission tracks the current unresolved permission prompt, if any.
	pendingPermission *PermissionRequestPayload

	// processes manages the single background process slot for this session.
	// nil until Session initialization wires it up.
	processes *processes.Controller

	// subscribers fan out live realtime event frames to all connected clients.
	subscribers      map[int]chan Frame
	nextSubscriberID int

	// store persists messages and meta to disk. May be nil if persistence is unavailable.
	store *store.SessionStore
	// persistedCount tracks how many history entries have been written to the store.
	persistedCount int

	subscribersMu  sync.RWMutex
	cancelQuery    context.CancelFunc
	currentRunDone chan struct{}
	closed         bool
	onMetaChanged         func(SessionMeta)
	onProcessStateChanged func(sessionID string, active bool, command string, args []string)
	mu                    sync.Mutex
}

// SessionMeta is the public view of a Session returned by the list endpoint.
type SessionMeta struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	WorkDir           string                    `json:"work_dir"`
	Status            SessionStatus             `json:"status"`
	ModelID           string                    `json:"model_id"`
	ModelName         string                    `json:"model_name"`
	ThinkingAvailable bool                      `json:"thinking_available,omitempty"`
	ThinkingEnabled   bool                      `json:"thinking_enabled,omitempty"`
	Permissions       tools.PermissionSet       `json:"permissions"`
	Profile           prompt.AgentProfile       `json:"profile"`
	PendingPermission *PermissionRequestPayload `json:"pending_permission,omitempty"`
	ActiveSkills      []string                  `json:"active_skills,omitempty"`
	InstalledSkillIDs []string                  `json:"installed_skill_ids"`
	CoworksGranted    []GrantedCowork           `json:"coworks_granted,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
	LastActive        time.Time                 `json:"last_active"`
}

func (s *Session) Meta() SessionMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metaLocked()
}

func (s *Session) metaLocked() SessionMeta {
	return SessionMeta{
		ID:                s.ID,
		Name:              s.Name,
		WorkDir:           s.WorkDir,
		Status:            s.Status,
		ModelID:           s.modelID,
		ModelName:         s.modelName,
		ThinkingAvailable: s.thinkingAvailable,
		ThinkingEnabled:   s.thinkingEnabled,
		Permissions:       s.permissions,
		Profile:           s.profile,
		PendingPermission: clonePermissionPayload(s.pendingPermission),
		ActiveSkills:      append([]string(nil), s.activeSkills...),
		InstalledSkillIDs: append([]string(nil), s.installedSkillIDs...),
		CoworksGranted:    append([]GrantedCowork(nil), s.coworksGranted...),
		CreatedAt:         s.CreatedAt,
		LastActive:        s.LastActive,
	}
}

func (s *Session) persistentMetaLocked() SessionMeta {
	return s.metaLocked()
}

func normalizeProfile(profile prompt.AgentProfile) prompt.AgentProfile {
	return prompt.NormalizeProfile(profile)
}

func normalizeToolMode(mode ToolMode) ToolMode {
	if mode == ToolModeAnswerOnly {
		return ToolModeAnswerOnly
	}
	return ToolModeWorkspaceChange
}

func normalizeAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func clonePermissionPayload(in *PermissionRequestPayload) *PermissionRequestPayload {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func (s *Session) notifyMetaChanged(meta SessionMeta) {
	if s.onMetaChanged != nil {
		s.onMetaChanged(meta)
	}
}

// ── ID helpers ────────────────────────────────────────────────────────────

func newSessionID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}

func newMsgID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "msg_" + hex.EncodeToString(b)
}

func newThreadID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "thread_" + hex.EncodeToString(b)
}

func newAgentMessageID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "agentmsg_" + hex.EncodeToString(b)
}
