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

// DisplayAttachment is a file rendered alongside a chat message.
type DisplayAttachment struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
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
	ID          string                  `json:"id"`
	Role        string                  `json:"role"` // user | assistant
	AuthorType  string                  `json:"author_type,omitempty"`
	AuthorID    string                  `json:"author_id,omitempty"`
	AuthorName  string                  `json:"author_name,omitempty"`
	AgentMeta   *tools.AgentMessageMeta `json:"agent_meta,omitempty"`
	Text        string                  `json:"text"`
	Streaming   bool                    `json:"streaming,omitempty"` // true while a response is in progress
	ToolCalls   []DisplayTool           `json:"tool_calls,omitempty"`
	Attachments []DisplayAttachment     `json:"attachments,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
}

type queuedInput struct {
	apiMessage api.Message
}

type ToolMode string

const (
	ToolModeAnswerOnly      ToolMode = "answer_only"
	ToolModeWorkspaceChange ToolMode = "workspace_change"
)

// Session is one agent instance with its own workspace and conversation history.
type Session struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	WorkDir    string        `json:"work_dir"` // absolute path
	Status     SessionStatus `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	LastActive time.Time     `json:"last_active"`

	provider     api.Provider
	registry     *tools.Registry
	checker      *webperm.Checker
	systemPrompt string
	maxTokens    int
	permissions  tools.PermissionSet
	profile      prompt.AgentProfile
	toolMode     ToolMode

	// apiMessages is the canonical conversation passed into the next model turn.
	apiMessages []api.Message

	// pendingInputs are appended while a run is in flight, then processed as the next turn.
	pendingInputs []queuedInput

	// history is the server-side render state returned by GET /history.
	history []DisplayMessage
	// pendingPermission tracks the current unresolved permission prompt, if any.
	pendingPermission *PermissionRequestPayload

	// subscribers fan out live realtime event frames to all connected clients.
	subscribers      map[int]chan Frame
	nextSubscriberID int

	// store persists messages and meta to disk. May be nil if persistence is unavailable.
	store *store.SessionStore
	// persistedCount tracks how many history entries have been written to the store.
	persistedCount int

	subscribersMu sync.RWMutex
	cancelQuery   context.CancelFunc
	closed        bool
	onMetaChanged func(SessionMeta)
	mu            sync.Mutex
}

// SessionMeta is the public view of a Session returned by the list endpoint.
type SessionMeta struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	WorkDir           string                    `json:"work_dir"`
	Status            SessionStatus             `json:"status"`
	Permissions       tools.PermissionSet       `json:"permissions"`
	Profile           prompt.AgentProfile       `json:"profile"`
	PendingPermission *PermissionRequestPayload `json:"pending_permission,omitempty"`
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
		Permissions:       s.permissions,
		Profile:           s.profile,
		PendingPermission: clonePermissionPayload(s.pendingPermission),
		CreatedAt:         s.CreatedAt,
		LastActive:        s.LastActive,
	}
}

func (s *Session) persistentMetaLocked() SessionMeta {
	meta := s.metaLocked()
	meta.PendingPermission = nil
	return meta
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
