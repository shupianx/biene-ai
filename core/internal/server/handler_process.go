package server

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
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
// Uses StopProcessByUser (not StopProcess) so the session records a
// one-shot system note — the agent's next turn is informed that the
// user interrupted the process, preventing stale assumptions like
// "your dev server is still running on port 5173".
// POST /api/sessions/{id}/process/stop
func (s *Server) handleProcessStop(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}
	if err := sess.StopProcessByUser(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess.ProcessState())
}

// handleProcessLogsWebSocket is a bidirectional PTY bridge between the
// session's background process and the renderer's xterm.js terminal.
//
// Server → client frames:
//
//	{"output":"<base64>"}   — PTY byte chunk (may contain ANSI escapes)
//	{"state":{...}}         — process lifecycle transitions
//
// Client → server frames:
//
//	{"input":"<base64>"}    — keystrokes / raw bytes to write into PTY
//	{"resize":{"cols":N,"rows":M}}
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

	// A larger backlog than the line-based version since raw PTY output
	// includes escape sequences; 256 KiB keeps a typical interactive
	// redraw history intact on reconnect.
	backlog, events, unsubscribe, err := sess.SubscribeProcessLogsWithBacklog(256 * 1024)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"process controller unavailable"}`))
		return
	}
	defer unsubscribe()

	var writeMu sync.Mutex
	writeFrame := func(frame processLogFrame) error {
		payload, err := json.Marshal(frame)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, payload)
	}

	// Current state first so the renderer can render the capsule shell
	// before any bytes arrive.
	state := sess.ProcessState()
	if err := writeFrame(processLogFrame{State: &state}); err != nil {
		return
	}

	if len(backlog) > 0 {
		if err := writeFrame(processLogFrame{Output: base64.StdEncoding.EncodeToString(backlog)}); err != nil {
			return
		}
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	closeDone := func() {
		doneOnce.Do(func() { close(done) })
	}

	// Reader goroutine: parse client frames and route them into the
	// session's PTY. A failure here (connection drop / bad message)
	// tears down the whole bridge.
	go func() {
		defer closeDone()
		// 64 KiB allows the renderer to paste whole buffers without
		// tripping gorilla's default limit of 512 bytes.
		conn.SetReadLimit(64 * 1024)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var frame inboundProcessFrame
			if err := json.Unmarshal(data, &frame); err != nil {
				continue
			}
			if frame.Input != "" {
				decoded, err := base64.StdEncoding.DecodeString(frame.Input)
				if err != nil {
					continue
				}
				_ = sess.WriteProcessInput(decoded)
			}
			if frame.Resize != nil {
				_ = sess.ResizeProcess(frame.Resize.Cols, frame.Resize.Rows)
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
			var frame processLogFrame
			switch ev.Kind {
			case "output":
				if len(ev.Bytes) == 0 {
					continue
				}
				frame.Output = base64.StdEncoding.EncodeToString(ev.Bytes)
			case "started", "stopped":
				st := ev.State
				frame.State = &st
			default:
				continue
			}
			if err := writeFrame(frame); err != nil {
				closeDone()
				return
			}
		case <-pingTicker.C:
			writeMu.Lock()
			err := conn.WriteControl(
				websocket.PingMessage,
				[]byte("ping"),
				time.Now().Add(5*time.Second),
			)
			writeMu.Unlock()
			if err != nil {
				closeDone()
				return
			}
		}
	}
}

type processLogFrame struct {
	Output string           `json:"output,omitempty"`
	State  *processes.State `json:"state,omitempty"`
}

type inboundProcessFrame struct {
	Input  string            `json:"input,omitempty"`
	Resize *processResizeMsg `json:"resize,omitempty"`
}

type processResizeMsg struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}
