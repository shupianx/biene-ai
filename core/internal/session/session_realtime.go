package session

import (
	"biene/internal/permission/webperm"
)

// ── Permission pending/clear ──────────────────────────────────────────────

func (s *Session) setPendingPermission(req webperm.PermissionRequest) *PermissionRequestPayload {
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

// ── Realtime subscriptions ────────────────────────────────────────────────

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

// send broadcasts a frame to all connected subscribers (non-blocking).
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
