package session

import (
	"biene/internal/agentloop"
	"biene/internal/tools"
	"time"
)

// ── Display history helpers ───────────────────────────────────────────────

func (s *Session) appendAssistantSegmentLocked() *DisplayMessage {
	usedSkillName := s.currentTurnSkillLabelLocked()
	s.history = append(s.history, DisplayMessage{
		ID:            newMsgID(),
		Role:          "assistant",
		AuthorType:    authorTypeAgent,
		AuthorID:      s.ID,
		AuthorName:    s.Name,
		UsedSkillName: usedSkillName,
		Streaming:     true,
		CreatedAt:     time.Now(),
	})
	return &s.history[len(s.history)-1]
}

func (s *Session) currentTurnSkillLabelLocked() string {
	if s.currentSkillName == "" {
		return ""
	}
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := s.history[i]
		if msg.Role != "assistant" || !msg.Streaming {
			break
		}
		if msg.UsedSkillName != "" {
			return ""
		}
	}
	return s.currentSkillName
}

func (s *Session) latestStreamingAssistantLocked() *DisplayMessage {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role == "assistant" && msg.Streaming {
			return msg
		}
	}
	return nil
}

func (s *Session) currentAssistantTextSegmentLocked() *DisplayMessage {
	if msg := s.latestStreamingAssistantLocked(); msg != nil && len(msg.ToolCalls) == 0 {
		return msg
	}
	return s.appendAssistantSegmentLocked()
}

func (s *Session) currentAssistantReasoningSegmentLocked() *DisplayMessage {
	if msg := s.latestStreamingAssistantLocked(); msg != nil {
		return msg
	}
	return s.appendAssistantSegmentLocked()
}

func (s *Session) currentAssistantToolSegmentLocked() *DisplayMessage {
	if msg := s.latestStreamingAssistantLocked(); msg != nil && msg.Text == "" {
		return msg
	}
	return s.appendAssistantSegmentLocked()
}

func (s *Session) latestActiveToolLocked(toolID, toolName string, statuses ...string) *DisplayTool {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role != "assistant" || !msg.Streaming {
			break
		}
		for j := len(msg.ToolCalls) - 1; j >= 0; j-- {
			toolCall := &msg.ToolCalls[j]
			if len(statuses) > 0 {
				matched := false
				for _, status := range statuses {
					if toolCall.Status == status {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			if toolID != "" && toolCall.ToolID == toolID {
				return toolCall
			}
			if toolID == "" && toolCall.ToolName == toolName {
				return toolCall
			}
		}
	}
	return nil
}

func (s *Session) finishStreamingAssistantSegmentsLocked() {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role != "assistant" || !msg.Streaming {
			break
		}
		finalizeReasoningLocked(msg)
		msg.Streaming = false
	}
}

func (s *Session) markInterruptedAssistantSegmentsLocked() {
	for i := len(s.history) - 1; i >= 0; i-- {
		msg := &s.history[i]
		if msg.Role != "assistant" {
			break
		}
		for j := range msg.ToolCalls {
			toolCall := &msg.ToolCalls[j]
			if toolCall.Status != "pending" && toolCall.Status != "composing" {
				continue
			}
			toolCall.Status = "cancelled"
			if toolCall.Result == "" {
				toolCall.Result = "Interrupted."
			}
		}
		finalizeReasoningLocked(msg)
		msg.Streaming = false
	}
}

func finalizeReasoningLocked(msg *DisplayMessage) {
	if msg == nil || msg.Reasoning == nil || msg.Reasoning.DurationMS > 0 {
		return
	}
	durationMS := time.Since(msg.Reasoning.StartedAt).Milliseconds()
	if durationMS <= 0 {
		durationMS = 1
	}
	msg.Reasoning.DurationMS = durationMS
}

// applyEvent updates the display history for an agent loop event.
func (s *Session) applyEvent(ev agentloop.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Kind {
	case agentloop.KindReasoningDelta:
		msg := s.currentAssistantReasoningSegmentLocked()
		if msg.Reasoning == nil {
			msg.Reasoning = &DisplayReasoning{StartedAt: time.Now()}
		}
		msg.Reasoning.Text += ev.Text
	case agentloop.KindTextDelta:
		if msg := s.latestStreamingAssistantLocked(); msg != nil {
			finalizeReasoningLocked(msg)
		}
		msg := s.currentAssistantTextSegmentLocked()
		msg.Text += ev.Text
	case agentloop.KindToolCompose:
		if msg := s.latestStreamingAssistantLocked(); msg != nil {
			finalizeReasoningLocked(msg)
		}
		msg := s.currentAssistantToolSegmentLocked()
		msg.ToolCalls = append(msg.ToolCalls, DisplayTool{
			ToolID:      ev.ToolID,
			ToolName:    ev.ToolName,
			ToolSummary: ev.ToolSummary,
			ToolInput:   ev.ToolInput,
			Status:      "composing",
		})
	case agentloop.KindToolStart:
		if msg := s.latestStreamingAssistantLocked(); msg != nil {
			finalizeReasoningLocked(msg)
		}
		if toolCall := s.latestActiveToolLocked(ev.ToolID, ev.ToolName, "composing"); toolCall != nil {
			toolCall.ToolSummary = ev.ToolSummary
			toolCall.ToolInput = ev.ToolInput
			toolCall.Status = "pending"
			break
		}
		msg := s.currentAssistantToolSegmentLocked()
		msg.ToolCalls = append(msg.ToolCalls, DisplayTool{
			ToolID:      ev.ToolID,
			ToolName:    ev.ToolName,
			ToolSummary: ev.ToolSummary,
			ToolInput:   ev.ToolInput,
			Status:      "pending",
		})
	case agentloop.KindToolResult:
		if msg := s.latestStreamingAssistantLocked(); msg != nil {
			finalizeReasoningLocked(msg)
		}
		if toolCall := s.latestActiveToolLocked(ev.ToolID, ev.ToolName, "pending", "composing"); toolCall != nil {
			if ev.IsError {
				toolCall.Status = "error"
			} else {
				toolCall.Status = "done"
			}
			toolCall.Result = ev.Text
		}
	case agentloop.KindToolDenied:
		if msg := s.latestStreamingAssistantLocked(); msg != nil {
			finalizeReasoningLocked(msg)
		}
		if toolCall := s.latestActiveToolLocked(ev.ToolID, ev.ToolName, "pending", "composing"); toolCall != nil {
			toolCall.Status = "denied"
		}
	case agentloop.KindError:
		msg := s.currentAssistantTextSegmentLocked()
		finalizeReasoningLocked(msg)
		if msg.Text != "" {
			msg.Text += "\n\n"
		}
		msg.Text += "**Error:** " + ev.Text
		s.finishStreamingAssistantSegmentsLocked()
	case agentloop.KindInterrupted:
		s.markInterruptedAssistantSegmentsLocked()
	case agentloop.KindDone:
		s.finishStreamingAssistantSegmentsLocked()
	}
}

// SnapshotHistory returns a copy of the current history (safe for JSON encoding).
func (s *Session) SnapshotHistory() []DisplayMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]DisplayMessage, len(s.history))
	copy(out, s.history)
	for i := range out {
		if out[i].AgentMeta != nil {
			out[i].AgentMeta = cloneAgentMessageMeta(out[i].AgentMeta)
		}
		if len(out[i].ToolCalls) > 0 {
			tc := make([]DisplayTool, len(out[i].ToolCalls))
			copy(tc, out[i].ToolCalls)
			out[i].ToolCalls = tc
		}
		if len(out[i].Attachments) > 0 {
			atts := make([]DisplayAttachment, len(out[i].Attachments))
			copy(atts, out[i].Attachments)
			out[i].Attachments = atts
		}
		if out[i].Reasoning != nil {
			out[i].Reasoning = cloneDisplayReasoning(out[i].Reasoning)
		}
	}
	return out
}

// cloneAgentMessageMeta returns a shallow copy of meta, or nil.
func cloneAgentMessageMeta(in *tools.AgentMessageMeta) *tools.AgentMessageMeta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneDisplayReasoning(in *DisplayReasoning) *DisplayReasoning {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
