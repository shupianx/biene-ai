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
	ctx, cfg, meta, activatedSkillName := s.prepareRunLocked(!running)
	s.mu.Unlock()
	if meta != nil {
		s.notifyMetaChanged(*meta)
	}

	s.send(makeFrame("message_added", messageAddedPayload{Message: display}))
	if activatedSkillName != "" {
		s.send(makeFrame("skill_activated", skillActivatedPayload{SkillName: activatedSkillName}))
	}

	// Persist the user display message outside the lock.
	s.persistDisplayMessage(display)

	if cfg != nil {
		go s.runQuery(ctx, cfg)
	}
}

func (s *Session) prepareRunLocked(shouldStart bool) (context.Context, *agentloop.Config, *SessionMeta, string) {
	if !shouldStart {
		return nil, nil, nil, ""
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelQuery = cancel
	s.Status = StatusRunning
	s.send(makeFrame("status", statusPayload{Status: s.Status}))
	meta := s.metaLocked()

	messages := append([]api.Message(nil), s.apiMessages...)
	registry, _ := registryForToolMode(s.registry, s.toolMode)
	installedSkills, activatedSkills := resolveSkillsForPrompt(s.WorkDir, s.history)
	systemPrompt := prompt.Build(registry, s.WorkDir, s.profile, installedSkills, activatedSkills)
	activatedSkillName := ""
	if len(activatedSkills) > 0 {
		activatedSkillName = activatedSkills[0].Name
	}
	s.currentSkillName = activatedSkillName
	cfg := &agentloop.Config{
		Provider:     s.provider,
		Registry:     registry,
		Checker:      s.checker,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		MaxTokens:    s.maxTokens,
	}
	return ctx, cfg, &meta, activatedSkillName
}

func resolveSkillsForPrompt(workDir string, history []DisplayMessage) ([]skills.Metadata, []skills.Definition) {
	installedSkills, err := skills.ScanForWorkDir(workDir)
	if err != nil || len(installedSkills) == 0 {
		return nil, nil
	}

	userText := latestUserText(history)
	if userText == "" {
		return installedSkills, nil
	}

	match := skills.SelectForText(userText, installedSkills)
	if match == nil {
		return installedSkills, nil
	}

	def, err := skills.LoadDefinition(*match)
	if err != nil {
		return installedSkills, nil
	}
	return installedSkills, []skills.Definition{def}
}

func latestUserText(history []DisplayMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role != "user" {
			continue
		}
		if msg.AuthorType != authorTypeUser {
			return ""
		}
		return msg.Text
	}
	return ""
}

// ── Agent loop ────────────────────────────────────────────────────────────

func (s *Session) runQuery(ctx context.Context, cfg *agentloop.Config) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[session %s] panic in runQuery: %v\n", s.ID, r)
			s.mu.Lock()
			s.cancelQuery = nil
			s.currentSkillName = ""
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
			ctxNext, cfgNext, metaNext, activatedSkillName := s.prepareRunLocked(true)
			s.mu.Unlock()
			if metaNext != nil {
				s.notifyMetaChanged(*metaNext)
			}
			if activatedSkillName != "" {
				s.send(makeFrame("skill_activated", skillActivatedPayload{SkillName: activatedSkillName}))
			}
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
	s.currentSkillName = ""
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
