package session

import (
	"fmt"
	"strings"
	"time"

	"tinte/internal/processes"
)

// ProcessState returns a snapshot of the session's background process.
func (s *Session) ProcessState() processes.State {
	if s.processes == nil {
		return processes.State{Status: processes.StatusIdle}
	}
	return s.processes.State()
}

// StopProcess terminates the session's background process. No-op if nothing
// is running.
func (s *Session) StopProcess() error {
	if s.processes == nil {
		return fmt.Errorf("process controller unavailable")
	}
	return s.processes.Stop()
}

// WriteProcessInput forwards bytes to the background process's PTY master
// so interactive CLIs receive them as if the user typed them into a real
// terminal.
func (s *Session) WriteProcessInput(data []byte) error {
	if s.processes == nil {
		return fmt.Errorf("process controller unavailable")
	}
	return s.processes.WriteInput(data)
}

// ResizeProcess updates the background process's PTY window size so
// curses-style UIs redraw to the visible panel dimensions.
func (s *Session) ResizeProcess(cols, rows uint16) error {
	if s.processes == nil {
		return fmt.Errorf("process controller unavailable")
	}
	return s.processes.Resize(cols, rows)
}

// StopProcessByUser is the HTTP-side entry point for the capsule's stop
// button. It differs from StopProcess only in that it also queues a
// system note so the agent learns about the manual interruption on its
// next turn — otherwise the agent would keep believing the process is
// still running and, for example, reply "your dev server is already
// live at http://localhost:5173" the next time it is asked to start
// one. The agent-initiated stop_process tool deliberately does not go
// through here since the agent already knows from the tool's return.
func (s *Session) StopProcessByUser() error {
	if s.processes == nil {
		return fmt.Errorf("process controller unavailable")
	}
	pre := s.processes.State()
	if err := s.processes.Stop(); err != nil {
		return err
	}
	if pre.Active {
		s.appendSystemNote(describeManualProcessStop(pre, s.processes.State()))
	}
	return nil
}

// appendSystemNote queues a one-shot note for the next user turn.
func (s *Session) appendSystemNote(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.mu.Lock()
	s.pendingSystemNotes = append(s.pendingSystemNotes, text)
	s.mu.Unlock()
}

// drainSystemNotesLocked returns and clears the queued notes. Caller
// must hold s.mu.
func (s *Session) drainSystemNotesLocked() []string {
	if len(s.pendingSystemNotes) == 0 {
		return nil
	}
	notes := s.pendingSystemNotes
	s.pendingSystemNotes = nil
	return notes
}

func describeManualProcessStop(pre, post processes.State) string {
	cmd := pre.Command
	if len(pre.Args) > 0 {
		cmd = fmt.Sprintf("%s %s", pre.Command, strings.Join(pre.Args, " "))
	}
	when := time.Now().Format("15:04:05")
	if post.ExitCode != nil {
		return fmt.Sprintf(
			"The background process `%s` was stopped by the user at %s (exit code %d). It is no longer running; start it again if you need to.",
			cmd, when, *post.ExitCode,
		)
	}
	return fmt.Sprintf(
		"The background process `%s` was stopped by the user at %s. It is no longer running; start it again if you need to.",
		cmd, when,
	)
}

// SubscribeProcessLogs returns a channel that receives live log lines from
// the session's background process. Output events deliver line data;
// started/stopped events reflect state transitions. The caller must invoke
// unsubscribe when done.
func (s *Session) SubscribeProcessLogs() (<-chan processes.Event, func(), error) {
	if s.processes == nil {
		return nil, nil, fmt.Errorf("process controller unavailable")
	}
	ch, unsub := s.processes.Subscribe()
	return ch, unsub, nil
}

// SubscribeProcessLogsWithBacklog returns the log written so far plus a
// live event channel such that no line is delivered twice. Use this when the
// consumer wants to see prior output before it attached.
func (s *Session) SubscribeProcessLogsWithBacklog(maxBacklog int) ([]byte, <-chan processes.Event, func(), error) {
	if s.processes == nil {
		return nil, nil, nil, fmt.Errorf("process controller unavailable")
	}
	backlog, ch, unsub := s.processes.SubscribeWithBacklog(maxBacklog)
	return backlog, ch, unsub, nil
}

// startProcessWatcher forwards the controller's started/stopped events onto
// the session's own realtime channel so the frontend can update the capsule
// UI without opening the logs websocket. Output events are ignored here;
// consumers that want live lines must subscribe to the logs WS directly.
func (s *Session) startProcessWatcher() {
	if s.processes == nil {
		return
	}
	ch, _ := s.processes.Subscribe()
	go func() {
		for ev := range ch {
			if ev.Kind != "started" && ev.Kind != "stopped" {
				continue
			}
			s.send(makeFrame("process_state", ev.State))
		}
	}()
}
