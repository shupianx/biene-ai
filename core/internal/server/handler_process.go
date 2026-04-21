package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"biene/internal/processes"

	"github.com/gorilla/websocket"
)

// handleProcessState returns the current background process snapshot.
// GET /api/sessions/{id}/process
func (s *Server) handleProcessState(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}
	writeJSON(w, http.StatusOK, sess.ProcessState())
}

// handleActiveProcesses returns every session that currently has a running
// background process. Used by the Electron quit confirmation.
// GET /api/processes/active
func (s *Server) handleActiveProcesses(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"processes": s.mgr.ActiveBackgroundProcesses(),
	})
}

// handleProcessStop terminates the session's background process.
// POST /api/sessions/{id}/process/stop
func (s *Server) handleProcessStop(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}
	if err := sess.StopProcess(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess.ProcessState())
}

// handleProcessLogsWebSocket streams live log lines for the session's
// background process. Lines are sent as JSON `{"line":"..."}` frames.
// State transitions arrive as `{"state":{...}}`. Closed when the client
// disconnects.
//
// GET /api/sessions/{id}/process/logs/ws
func (s *Server) handleProcessLogsWebSocket(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	conn, err := chatUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade process logs websocket: %v", err)
		return
	}
	defer conn.Close()

	backlog, events, unsubscribe, err := sess.SubscribeProcessLogsWithBacklog(64 * 1024)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"process controller unavailable"}`))
		return
	}
	defer unsubscribe()

	// Deliver the current state first so the client can render immediately.
	if err := writeProcessLogFrame(conn, processLogFrame{State: ptrState(sess.ProcessState())}); err != nil {
		return
	}

	// Replay the log file content so the client sees output that arrived
	// before it subscribed. Each line is sent as its own frame so the
	// frontend can append without splitting.
	if len(backlog) > 0 {
		for _, line := range strings.Split(strings.TrimRight(string(backlog), "\n"), "\n") {
			if err := writeProcessLogFrame(conn, processLogFrame{Line: line}); err != nil {
				return
			}
		}
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	closeDone := func() {
		doneOnce.Do(func() { close(done) })
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
		case ev, ok := <-events:
			if !ok {
				return
			}
			frame := processLogFrame{}
			switch ev.Kind {
			case "output":
				frame.Line = ev.Line
			case "started", "stopped":
				st := ev.State
				frame.State = &st
			default:
				continue
			}
			if err := writeProcessLogFrame(conn, frame); err != nil {
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

type processLogFrame struct {
	Line  string           `json:"line,omitempty"`
	State *processes.State `json:"state,omitempty"`
}

func writeProcessLogFrame(conn *websocket.Conn, frame processLogFrame) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func ptrState(s processes.State) *processes.State {
	return &s
}
