package session

import "encoding/json"

// ManagerFrame is a pre-serialized realtime payload for list-level subscribers.
type ManagerFrame struct {
	EventType string
	Data      []byte
}

type sessionMetaPayload struct {
	Session SessionMeta `json:"session"`
}

type sessionDeletedPayload struct {
	ID string `json:"id"`
}

// sessionProcessStatePayload mirrors the runtime "is a background process
// running" snapshot for one session. Carried only on the wire — not in
// SessionMeta, not on disk. The frontend grid card subscribes to this so
// it can render a "running" notice without opening per-session sockets.
type sessionProcessStatePayload struct {
	SessionID string   `json:"session_id"`
	Active    bool     `json:"active"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
}

func makeManagerFrame(eventType string, payload any) ManagerFrame {
	data, _ := json.Marshal(payload)
	return ManagerFrame{EventType: eventType, Data: data}
}

func (m *SessionManager) SubscribeEvents() (int, <-chan ManagerFrame) {
	m.subscribersMu.Lock()
	defer m.subscribersMu.Unlock()

	id := m.nextSubscriberID
	m.nextSubscriberID++

	ch := make(chan ManagerFrame, 64)
	if m.subscribers == nil {
		m.subscribers = make(map[int]chan ManagerFrame)
	}
	m.subscribers[id] = ch
	return id, ch
}

func (m *SessionManager) UnsubscribeEvents(id int) {
	m.subscribersMu.Lock()
	defer m.subscribersMu.Unlock()

	ch, ok := m.subscribers[id]
	if !ok {
		return
	}
	delete(m.subscribers, id)
	close(ch)
}

func (m *SessionManager) send(frame ManagerFrame) {
	m.subscribersMu.RLock()
	defer m.subscribersMu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (m *SessionManager) emitSessionCreated(meta SessionMeta) {
	m.send(makeManagerFrame("session_created", sessionMetaPayload{Session: meta}))
}

func (m *SessionManager) emitSessionUpdated(meta SessionMeta) {
	m.send(makeManagerFrame("session_updated", sessionMetaPayload{Session: meta}))
}

func (m *SessionManager) emitSessionDeleted(id string) {
	m.send(makeManagerFrame("session_deleted", sessionDeletedPayload{ID: id}))
}

func (m *SessionManager) emitSessionProcessState(sessionID string, active bool, command string, args []string) {
	m.send(makeManagerFrame("session_process_state", sessionProcessStatePayload{
		SessionID: sessionID,
		Active:    active,
		Command:   command,
		Args:      append([]string(nil), args...),
	}))
}
