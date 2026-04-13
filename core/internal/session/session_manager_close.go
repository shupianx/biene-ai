package session

// Close stops all live session activity and closes realtime subscribers.
func (m *SessionManager) Close() {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	for _, sess := range sessions {
		sess.close()
	}

	m.subscribersMu.Lock()
	subscribers := m.subscribers
	m.subscribers = make(map[int]chan ManagerFrame)
	m.subscribersMu.Unlock()

	for _, ch := range subscribers {
		close(ch)
	}
}
