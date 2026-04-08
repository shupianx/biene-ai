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
	"biene/internal/permission"
	"biene/internal/prompt"
	"biene/internal/query"
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
	checker      *permission.HTTPChecker
	systemPrompt string
	maxTokens    int
	permissions  tools.PermissionSet
	profile      prompt.AgentProfile

	// apiMessages is the canonical conversation passed into the next model turn.
	apiMessages []api.Message

	// pendingInputs are appended while a run is in flight, then processed as the next turn.
	pendingInputs []queuedInput

	// history is the server-side render state returned by GET /history.
	history []DisplayMessage
	// pendingPermission tracks the current unresolved permission prompt, if any.
	pendingPermission *PermissionRequestPayload

	// subscribers fan out live SSE frames to all connected clients.
	subscribers      map[int]chan Frame
	nextSubscriberID int

	// store persists messages and meta to disk. May be nil if persistence is unavailable.
	store *store.SessionStore
	// persistedCount tracks how many history entries have been written to the store.
	persistedCount int

	subscribersMu sync.RWMutex
	cancelQuery   context.CancelFunc
	closed        bool
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

// ── Permission pending/clear ──────────────────────────────────────────────

func (s *Session) setPendingPermission(req permission.PermissionRequest) *PermissionRequestPayload {
	payload := &PermissionRequestPayload{
		RequestID:   req.RequestID,
		Permission:  string(req.Permission),
		ToolName:    req.ToolName,
		ToolSummary: req.ToolSummary,
		ToolInput:   req.ToolInput,
	}

	s.mu.Lock()
	s.pendingPermission = payload
	s.mu.Unlock()
	return clonePermissionPayload(payload)
}

func (s *Session) clearPendingPermission(requestID string) {
	cleared := false

	s.mu.Lock()
	if s.pendingPermission == nil {
		s.mu.Unlock()
		return
	}
	if requestID != "" && s.pendingPermission.RequestID != requestID {
		s.mu.Unlock()
		return
	}
	s.pendingPermission = nil
	cleared = true
	s.mu.Unlock()

	if cleared {
		s.send(makeFrame("permission_cleared", permissionClearedPayload{RequestID: requestID}))
	}
}

// ── SSE subscription ──────────────────────────────────────────────────────

func (s *Session) SubscribeEvents() (int, <-chan Frame) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	id := s.nextSubscriberID
	s.nextSubscriberID++

	ch := make(chan Frame, 256)
	if s.subscribers == nil {
		s.subscribers = make(map[int]chan Frame)
	}
	s.subscribers[id] = ch
	return id, ch
}

func (s *Session) UnsubscribeEvents(id int) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	ch, ok := s.subscribers[id]
	if !ok {
		return
	}
	delete(s.subscribers, id)
	close(ch)
}

// send broadcasts a frame to all connected SSE subscribers (non-blocking).
func (s *Session) send(frame Frame) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

// ── Lifecycle ─────────────────────────────────────────────────────────────

// cancelCurrentQuery cancels any in-flight query goroutine.
func (s *Session) CancelCurrentQuery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelQuery != nil {
		s.cancelQuery()
		s.cancelQuery = nil
	}
}

// ResolvePermission resolves a pending permission request and returns fresh session metadata.
func (s *Session) ResolvePermission(requestID string, decision permission.Decision) (SessionMeta, error) {
	if err := s.checker.Resolve(requestID, decision); err != nil {
		return SessionMeta{}, err
	}
	return s.Meta(), nil
}

func (s *Session) close() {
	s.mu.Lock()
	s.closed = true
	if s.cancelQuery != nil {
		s.cancelQuery()
		s.cancelQuery = nil
	}
	s.mu.Unlock()

	s.subscribersMu.Lock()
	for id, ch := range s.subscribers {
		delete(s.subscribers, id)
		close(ch)
	}
	s.subscribersMu.Unlock()
}

// ── Input enqueueing ──────────────────────────────────────────────────────

// enqueueUserInput appends a user-authored message and triggers the next run when idle.
func (s *Session) EnqueueUserInput(text string, attachments []DisplayAttachment, messageID string) {
	if messageID == "" {
		messageID = newMsgID()
	}

	display := DisplayMessage{
		ID:          messageID,
		Role:        "user",
		AuthorType:  authorTypeUser,
		AuthorName:  "You",
		Text:        displayTextForInput(authorTypeUser, text, attachments),
		Attachments: cloneAttachments(attachments),
		CreatedAt:   time.Now(),
	}

	modelText := buildInputText(authorTypeUser, "", "", text, attachments, nil)
	s.enqueueInput(display, api.Message{
		Role:    api.RoleUser,
		Content: []api.ContentBlock{api.TextBlock{Text: modelText}},
	})
}

// enqueueAgentInput appends a peer-agent-authored message and triggers the next run when idle.
func (s *Session) enqueueAgentInput(sourceID, sourceName, text string, attachments []DisplayAttachment, meta tools.AgentMessageMeta) {
	display := DisplayMessage{
		ID:          newMsgID(),
		Role:        "user",
		AuthorType:  authorTypeAgent,
		AuthorID:    sourceID,
		AuthorName:  sourceName,
		AgentMeta:   cloneAgentMessageMeta(&meta),
		Text:        displayTextForInput(authorTypeAgent, text, attachments),
		Attachments: cloneAttachments(attachments),
		CreatedAt:   time.Now(),
	}

	modelText := buildInputText(authorTypeAgent, sourceID, sourceName, text, attachments, &meta)
	s.enqueueInput(display, api.Message{
		Role:    api.RoleUser,
		Content: []api.ContentBlock{api.TextBlock{Text: modelText}},
	})
}

func (s *Session) enqueueInput(display DisplayMessage, apiMessage api.Message) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	s.history = append(s.history, display)
	s.LastActive = time.Now()
	running := s.Status == StatusRunning
	if running {
		s.pendingInputs = append(s.pendingInputs, queuedInput{apiMessage: apiMessage})
	} else {
		s.apiMessages = append(s.apiMessages, apiMessage)
	}
	ctx, cfg := s.prepareRunLocked(!running)
	s.mu.Unlock()

	s.send(makeFrame("message_added", messageAddedPayload{Message: display}))

	// Persist the user display message outside the lock.
	s.persistDisplayMessage(display)

	if cfg != nil {
		go s.runQuery(ctx, cfg)
	}
}

func (s *Session) prepareRunLocked(shouldStart bool) (context.Context, *query.Config) {
	if !shouldStart {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelQuery = cancel
	s.Status = StatusRunning
	s.send(makeFrame("status", statusPayload{Status: s.Status}))

	messages := append([]api.Message(nil), s.apiMessages...)
	cfg := &query.Config{
		Provider:     s.provider,
		Registry:     s.registry,
		Checker:      s.checker,
		SystemPrompt: s.systemPrompt,
		Messages:     messages,
		MaxTokens:    s.maxTokens,
	}
	return ctx, cfg
}

// ── Agent loop ────────────────────────────────────────────────────────────

func (s *Session) runQuery(ctx context.Context, cfg *query.Config) {
	hadError := false
	wasInterrupted := false

	for ev := range query.Run(ctx, cfg) {
		if ev.Kind == query.KindError {
			hadError = true
		} else if ev.Kind == query.KindInterrupted {
			wasInterrupted = true
		}

		s.applyEvent(ev)
		switch ev.Kind {
		case query.KindTextDelta:
			s.send(makeFrame("text_delta", textDeltaPayload{Text: ev.Text}))
		case query.KindToolCompose:
			s.send(makeFrame("tool_compose", toolStartPayload{
				ToolID:      ev.ToolID,
				ToolName:    ev.ToolName,
				ToolSummary: ev.ToolSummary,
				ToolInput:   ev.ToolInput,
			}))
		case query.KindToolStart:
			s.send(makeFrame("tool_start", toolStartPayload{
				ToolID:      ev.ToolID,
				ToolName:    ev.ToolName,
				ToolSummary: ev.ToolSummary,
				ToolInput:   ev.ToolInput,
			}))
		case query.KindToolResult:
			s.send(makeFrame("tool_result", toolResultPayload{
				ToolID:   ev.ToolID,
				ToolName: ev.ToolName,
				Text:     ev.Text,
				IsError:  ev.IsError,
			}))
		case query.KindToolDenied:
			s.send(makeFrame("tool_denied", toolDeniedPayload{
				ToolID:   ev.ToolID,
				ToolName: ev.ToolName,
			}))
		case query.KindError:
			s.send(makeFrame("error", errorPayload{Message: ev.Text}))
			s.send(makeFrame("done", donePayload{}))
		case query.KindInterrupted:
			s.send(makeFrame("done", donePayload{}))
		case query.KindDone:
			s.send(makeFrame("done", donePayload{}))
		}
	}

	s.finishRun(cfg, hadError, wasInterrupted)
}

func (s *Session) finishRun(cfg *query.Config, hadError bool, interrupted bool) {
	s.mu.Lock()
	if s.closed {
		s.cancelQuery = nil
		s.mu.Unlock()
		return
	}

	if !interrupted {
		s.apiMessages = cfg.Messages
	}
	s.cancelQuery = nil

	if interrupted {
		s.markInterruptedAssistantSegmentsLocked()
	}

	if len(s.pendingInputs) > 0 {
		for _, pending := range s.pendingInputs {
			s.apiMessages = append(s.apiMessages, pending.apiMessage)
		}
		if !interrupted {
			s.pendingInputs = nil
			ctxNext, cfgNext := s.prepareRunLocked(true)
			s.mu.Unlock()
			go s.runQuery(ctxNext, cfgNext)
			return
		}
	}

	s.pendingInputs = nil
	if interrupted {
		s.Status = StatusIdle
	} else if hadError {
		s.Status = StatusError
	} else {
		s.Status = StatusIdle
	}
	s.LastActive = time.Now()
	status := s.Status
	// Snapshot what needs persisting while still holding the lock.
	newDisplay := append([]DisplayMessage(nil), s.history[s.persistedCount:]...)
	s.persistedCount = len(s.history)
	apiMsgs := append([]api.Message(nil), s.apiMessages...)
	metaSnap := s.persistentMetaLocked()
	s.mu.Unlock()

	s.send(makeFrame("status", statusPayload{Status: status}))
	s.persistAfterRun(newDisplay, apiMsgs, metaSnap)
}

// ── Agent delivery helpers ────────────────────────────────────────────────

type outboundAgentDelivery struct {
	MessageMeta          tools.AgentMessageMeta
	IsReply              bool
	ReplySourceDisplayID string
}

func (s *Session) prepareOutboundAgentDelivery(targetAgentID string, requestedRequiresReply bool) outboundAgentDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()

	if replyMsg := s.latestPendingReplyRequestLocked(targetAgentID); replyMsg != nil && replyMsg.AgentMeta != nil {
		return outboundAgentDelivery{
			MessageMeta: tools.AgentMessageMeta{
				ThreadID:      replyMsg.AgentMeta.ThreadID,
				MessageID:     newAgentMessageID(),
				InReplyTo:     replyMsg.AgentMeta.MessageID,
				RequiresReply: false,
			},
			IsReply:              true,
			ReplySourceDisplayID: replyMsg.ID,
		}
	}

	if latestMsg := s.latestIncomingAgentMessageLocked(targetAgentID); latestMsg != nil && latestMsg.AgentMeta != nil {
		return outboundAgentDelivery{
			MessageMeta: tools.AgentMessageMeta{
				ThreadID:      latestMsg.AgentMeta.ThreadID,
				MessageID:     newAgentMessageID(),
				InReplyTo:     latestMsg.AgentMeta.MessageID,
				RequiresReply: requestedRequiresReply,
			},
		}
	}

	return outboundAgentDelivery{
		MessageMeta: tools.AgentMessageMeta{
			ThreadID:      newThreadID(),
			MessageID:     newAgentMessageID(),
			RequiresReply: requestedRequiresReply,
		},
	}
}

func (s *Session) latestIncomingAgentMessageLocked(sourceAgentID string) *DisplayMessage {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role == "user" && msg.AuthorType == authorTypeAgent && msg.AuthorID == sourceAgentID && msg.AgentMeta != nil {
			return msg
		}
	}
	return nil
}

func (s *Session) latestPendingReplyRequestLocked(sourceAgentID string) *DisplayMessage {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role != "user" || msg.AuthorType != authorTypeAgent || msg.AuthorID != sourceAgentID || msg.AgentMeta == nil {
			continue
		}
		if msg.AgentMeta.RequiresReply && !msg.AgentMeta.ReplySent {
			return msg
		}
	}
	return nil
}

func (s *Session) markAgentReplySent(displayMessageID string) {
	if displayMessageID == "" {
		return
	}

	var updated DisplayMessage
	changed := false

	s.mu.Lock()
	for i := range s.history {
		msg := &s.history[i]
		if msg.ID != displayMessageID || msg.AgentMeta == nil || msg.AgentMeta.ReplySent {
			continue
		}
		msg.AgentMeta.ReplySent = true
		updated = *msg
		updated.AgentMeta = cloneAgentMessageMeta(msg.AgentMeta)
		changed = true
		break
	}
	s.mu.Unlock()

	if changed {
		s.updatePersistedDisplayMessage(updated)
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
