package session

import (
	"context"
	"fmt"
	"time"

	"biene/internal/agentloop"
	"biene/internal/api"
	"biene/internal/permission"
	"biene/internal/tools"
)

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

// EnqueueUserInput appends a user-authored message and triggers the next run when idle.
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

func (s *Session) prepareRunLocked(shouldStart bool) (context.Context, *agentloop.Config) {
	if !shouldStart {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelQuery = cancel
	s.Status = StatusRunning
	s.send(makeFrame("status", statusPayload{Status: s.Status}))

	messages := append([]api.Message(nil), s.apiMessages...)
	cfg := &agentloop.Config{
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

func (s *Session) runQuery(ctx context.Context, cfg *agentloop.Config) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[session %s] panic in runQuery: %v\n", s.ID, r)
			s.mu.Lock()
			s.cancelQuery = nil
			s.Status = StatusError
			s.LastActive = time.Now()
			s.mu.Unlock()
			s.send(makeFrame("error", errorPayload{Message: fmt.Sprintf("internal error: %v", r)}))
			s.send(makeFrame("done", donePayload{}))
			s.send(makeFrame("status", statusPayload{Status: StatusError}))
		}
	}()

	hadError := false
	wasInterrupted := false

	for ev := range agentloop.Run(ctx, cfg) {
		if ev.Kind == agentloop.KindError {
			hadError = true
		} else if ev.Kind == agentloop.KindInterrupted {
			wasInterrupted = true
		}

		s.applyEvent(ev)
		switch ev.Kind {
		case agentloop.KindTextDelta:
			s.send(makeFrame("text_delta", textDeltaPayload{Text: ev.Text}))
		case agentloop.KindToolCompose:
			s.send(makeFrame("tool_compose", toolStartPayload{
				ToolID:      ev.ToolID,
				ToolName:    ev.ToolName,
				ToolSummary: ev.ToolSummary,
				ToolInput:   ev.ToolInput,
			}))
		case agentloop.KindToolStart:
			s.send(makeFrame("tool_start", toolStartPayload{
				ToolID:      ev.ToolID,
				ToolName:    ev.ToolName,
				ToolSummary: ev.ToolSummary,
				ToolInput:   ev.ToolInput,
			}))
		case agentloop.KindToolResult:
			s.send(makeFrame("tool_result", toolResultPayload{
				ToolID:   ev.ToolID,
				ToolName: ev.ToolName,
				Text:     ev.Text,
				IsError:  ev.IsError,
			}))
		case agentloop.KindToolDenied:
			s.send(makeFrame("tool_denied", toolDeniedPayload{
				ToolID:   ev.ToolID,
				ToolName: ev.ToolName,
			}))
		case agentloop.KindError:
			s.send(makeFrame("error", errorPayload{Message: ev.Text}))
			s.send(makeFrame("done", donePayload{}))
		case agentloop.KindInterrupted:
			s.send(makeFrame("done", donePayload{}))
		case agentloop.KindDone:
			s.send(makeFrame("done", donePayload{}))
		}
	}

	s.finishRun(cfg, hadError, wasInterrupted)
}

func (s *Session) finishRun(cfg *agentloop.Config, hadError bool, interrupted bool) {
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
