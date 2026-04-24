package session

import "tinte/internal/tools"

// ── Agent delivery helpers ────────────────────────────────────────────────

func (s *Session) prepareOutboundAgentDelivery(targetAgentID string) tools.AgentMessageMeta {
	s.mu.Lock()
	defer s.mu.Unlock()

	if latestMsg := s.latestIncomingAgentMessageLocked(targetAgentID); latestMsg != nil && latestMsg.AgentMeta != nil {
		return tools.AgentMessageMeta{
			ThreadID:  latestMsg.AgentMeta.ThreadID,
			MessageID: newAgentMessageID(),
			InReplyTo: latestMsg.AgentMeta.MessageID,
		}
	}

	return tools.AgentMessageMeta{
		ThreadID:  newThreadID(),
		MessageID: newAgentMessageID(),
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
