package session

import (
	"biene/internal/query"
	"biene/internal/tools"
	"time"
)

// ── Display history helpers ───────────────────────────────────────────────

func (s *Session) appendAssistantSegmentLocked() *DisplayMessage {
	s.history = append(s.history, DisplayMessage{
		ID:         newMsgID(),
		Role:       "assistant",
		AuthorType: authorTypeAgent,
		AuthorID:   s.ID,
		AuthorName: s.Name,
		Streaming:  true,
		CreatedAt:  time.Now(),
	})
	return &s.history[len(s.history)-1]
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
		msg.Streaming = false
	}
}

// applyEvent updates the display history for a query event.
func (s *Session) applyEvent(ev query.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Kind {
	case query.KindTextDelta:
		msg := s.currentAssistantTextSegmentLocked()
		msg.Text += ev.Text
	case query.KindToolCompose:
		msg := s.currentAssistantToolSegmentLocked()
		msg.ToolCalls = append(msg.ToolCalls, DisplayTool{
			ToolID:      ev.ToolID,
			ToolName:    ev.ToolName,
			ToolSummary: ev.ToolSummary,
			ToolInput:   ev.ToolInput,
			Status:      "composing",
		})
	case query.KindToolStart:
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
	case query.KindToolResult:
		if toolCall := s.latestActiveToolLocked(ev.ToolID, ev.ToolName, "pending", "composing"); toolCall != nil {
			if ev.IsError {
				toolCall.Status = "error"
			} else {
				toolCall.Status = "done"
			}
			toolCall.Result = ev.Text
		}
	case query.KindToolDenied:
		if toolCall := s.latestActiveToolLocked(ev.ToolID, ev.ToolName, "pending", "composing"); toolCall != nil {
			toolCall.Status = "denied"
		}
	case query.KindError:
		msg := s.currentAssistantTextSegmentLocked()
		if msg.Text != "" {
			msg.Text += "\n\n"
		}
		msg.Text += "**Error:** " + ev.Text
		s.finishStreamingAssistantSegmentsLocked()
	case query.KindInterrupted:
		s.markInterruptedAssistantSegmentsLocked()
	case query.KindDone:
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
