package session

import (
	"context"
	"fmt"
	"time"

	"biene/internal/agentloop"
	"biene/internal/api"
	"biene/internal/permission"
	"biene/internal/prompt"
	"biene/internal/skills"
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
	s.mu.Lock()
	expired := s.pendingPermission != nil &&
		s.pendingPermission.RequestID == requestID &&
		s.pendingPermission.Expired
	s.mu.Unlock()
	if expired {
		s.clearPendingPermission(requestID)
		return s.Meta(), nil
	}
	if err := s.checker.Resolve(requestID, decision); err != nil {
		return SessionMeta{}, err
	}
	return s.Meta(), nil
}

func (s *Session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	cancel := s.cancelQuery
	runDone := s.currentRunDone
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if runDone != nil {
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
		}
	}

	s.mu.Lock()
	s.closed = true
	s.cancelQuery = nil
	s.currentRunDone = nil
	s.mu.Unlock()

	s.subscribersMu.Lock()
	for id, ch := range s.subscribers {
		delete(s.subscribers, id)
		close(ch)
	}
	s.subscribersMu.Unlock()

	if s.processes != nil {
		s.processes.Close()
	}
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
	ctx, cfg, meta, runDone := s.prepareRunLocked(!running)
	s.mu.Unlock()
	if meta != nil {
		s.notifyMetaChanged(*meta)
	}

	s.send(makeFrame("message_added", messageAddedPayload{Message: display}))

	// Persist the user display message outside the lock.
	s.persistDisplayMessage(display)

	if cfg != nil {
		go s.runQuery(ctx, cfg, runDone)
	}
}

func (s *Session) prepareRunLocked(shouldStart bool) (context.Context, *agentloop.Config, *SessionMeta, chan struct{}) {
	if !shouldStart {
		return nil, nil, nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	s.cancelQuery = cancel
	s.currentRunDone = runDone
	s.Status = StatusRunning
	s.send(makeFrame("status", statusPayload{Status: s.Status}))
	meta := s.metaLocked()

	messages := append([]api.Message(nil), s.apiMessages...)
	registry, _ := registryForToolMode(s.registry, s.toolMode)
	installedSkills := resolveSkillsForPrompt(s.WorkDir)
	systemPrompt := prompt.Build(registry, s.WorkDir, s.profile, prompt.AgentIdentity{
		ID:      s.ID,
		Name:    s.Name,
		WorkDir: s.WorkDir,
	}, installedSkills)
	cfg := &agentloop.Config{
		Provider:     s.provider,
		Registry:     registry,
		Checker:      s.checker,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		RequestOpts: api.RequestOptions{
			ThinkingExtra: thinkingExtra(s.thinkingAvailable, s.thinkingEnabled, s.thinkingOn, s.thinkingOff),
		},
	}
	return ctx, cfg, &meta, runDone
}

// thinkingExtra picks the JSON fragment to splat into the request body
// based on whether the provider supports thinking and whether the user
// has it toggled on for this session. Returns nil when the provider has
// no fragment for the chosen state (e.g. a template that only declares
// the "on" fragment and relies on the backend's default-off behavior).
func thinkingExtra(available, enabled bool, on, off map[string]any) map[string]any {
	if !available {
		return nil
	}
	if enabled {
		return on
	}
	return off
}

func resolveSkillsForPrompt(workDir string) []skills.Metadata {
	installedSkills, err := skills.ScanForWorkDir(workDir)
	if err != nil {
		return nil
	}
	return installedSkills
}

// ── Agent loop ────────────────────────────────────────────────────────────

func (s *Session) runQuery(ctx context.Context, cfg *agentloop.Config, runDone chan struct{}) {
	defer close(runDone)
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
		case agentloop.KindReasoningDelta:
			s.send(makeFrame("reasoning_delta", reasoningDeltaPayload{Text: ev.Text}))
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
			ctxNext, cfgNext, metaNext, nextRunDone := s.prepareRunLocked(true)
			s.mu.Unlock()
			if metaNext != nil {
				s.notifyMetaChanged(*metaNext)
			}
			go s.runQuery(ctxNext, cfgNext, nextRunDone)
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
	s.notifyMetaChanged(metaSnap)
	s.persistAfterRun(newDisplay, apiMsgs, metaSnap)
}
