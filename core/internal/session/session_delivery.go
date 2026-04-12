package session

import (
	"biene/internal/tools"
)

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
