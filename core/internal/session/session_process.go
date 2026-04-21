package session

import (
	"fmt"

	"biene/internal/processes"
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
