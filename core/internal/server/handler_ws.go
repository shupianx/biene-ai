package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"biene/internal/session"
	"github.com/gorilla/websocket"
)

var chatUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type websocketEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// handleChatWebSocket serves the realtime event stream for a session.
// GET /api/sessions/{id}/ws
func (s *Server) handleChatWebSocket(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	conn, err := chatUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade websocket for %s: %v", r.PathValue("id"), err)
		return
	}
	defer conn.Close()

	subID, events := sess.SubscribeEvents()
	defer sess.UnsubscribeEvents(subID)

	done := make(chan struct{})
	var doneOnce sync.Once
	closeDone := func() {
		doneOnce.Do(func() {
			close(done)
		})
	}

	go func() {
		defer closeDone()
		conn.SetReadLimit(512)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case frame, ok := <-events:
			if !ok {
				return
			}
			if err := writeWebSocketEvent(conn, frame); err != nil {
				closeDone()
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteControl(
				websocket.PingMessage,
				[]byte("ping"),
				time.Now().Add(5*time.Second),
			); err != nil {
				closeDone()
				return
			}
		}
	}
}

func writeWebSocketEvent(conn *websocket.Conn, frame session.Frame) error {
	payload, err := json.Marshal(websocketEvent{
		Type: frame.EventType,
		Data: frame.Data,
	})
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, payload)
}
