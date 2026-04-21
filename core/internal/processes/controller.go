// Package processes manages a single long-running background process
// per agent session. It captures interleaved stdout/stderr to a log
// file, streams live lines to subscribers, and enforces a 1:1 process
// slot with automatic replacement on restart.
package processes

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Status is the lifecycle state of the current or most recent process.
type Status string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
	StatusKilled  Status = "killed"
	StatusFailed  Status = "failed"
)

// State is a JSON-serializable snapshot of the controller's current process.
type State struct {
	Active    bool       `json:"active"`
	Status    Status     `json:"status"`
	Command   string     `json:"command,omitempty"`
	Args      []string   `json:"args,omitempty"`
	Cwd       string     `json:"cwd,omitempty"`
	PID       int        `json:"pid,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	ExitedAt  *time.Time `json:"exited_at,omitempty"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	LogFile   string     `json:"log_file,omitempty"`
}

// StartOptions describes a process to launch.
type StartOptions struct {
	Command string
	Args    []string
	Cwd     string // absolute path
	Env     map[string]string
}

// StartResult is returned from Start.
type StartResult struct {
	State    State
	Replaced bool // true if an existing process was auto-stopped
}

// ReadOptions controls ReadOutput.
type ReadOptions struct {
	TailLines  int   // if >0, return only the last N lines
	SinceBytes int64 // if >0, start reading from this byte offset in the log
	MaxBytes   int   // hard cap on bytes returned (default 32KiB)
}

// ReadResult bundles log content with cursors for incremental reads.
type ReadResult struct {
	State        State
	Content      string
	StartOffset  int64
	EndOffset    int64
	Truncated    bool
}

// Event is a live stream notification.
type Event struct {
	Kind  string // "output" | "started" | "stopped"
	Line  string // present when Kind=="output"; bare line, no trailing newline
	State State  // present for "started"/"stopped"
}

const (
	logsSubdir     = ".biene/logs"
	logFileName    = "current.log"
	defaultMaxRead = 32 * 1024
	subscriberBuf  = 128
)

// Controller manages one background process for one agent session.
type Controller struct {
	workDir string
	logPath string

	mu          sync.Mutex
	cmd         *exec.Cmd
	state       State
	logFile     *os.File
	subscribers map[int]chan Event
	nextSubID   int
	doneCh      chan struct{} // closed when current process fully exits
}

// New creates a controller rooted at workDir. The log directory is lazily
// created on first start.
func New(workDir string) *Controller {
	return &Controller{
		workDir:     workDir,
		logPath:     filepath.Join(workDir, logsSubdir, logFileName),
		state:       State{Status: StatusIdle},
		subscribers: make(map[int]chan Event),
	}
}

// State returns a snapshot of the current process state.
func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Start launches a new process. If one is already running, it is stopped first
// and StartResult.Replaced is set to true.
func (c *Controller) Start(opts StartOptions) (StartResult, error) {
	if strings.TrimSpace(opts.Command) == "" {
		return StartResult{}, errors.New("start_process: command is required")
	}
	if opts.Cwd == "" {
		return StartResult{}, errors.New("start_process: cwd is required")
	}

	replaced := false
	if c.isRunning() {
		if err := c.Stop(); err != nil {
			return StartResult{}, fmt.Errorf("stopping previous process: %w", err)
		}
		replaced = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(c.logPath), 0o755); err != nil {
		return StartResult{}, fmt.Errorf("preparing log dir: %w", err)
	}
	logFile, err := os.OpenFile(c.logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return StartResult{}, fmt.Errorf("opening log file: %w", err)
	}

	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range opts.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	setProcessGroup(cmd)

	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		logFile.Close()
		return StartResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		logFile.Close()
		return StartResult{}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		c.state = State{
			Status:  StatusFailed,
			Command: opts.Command,
			Args:    append([]string(nil), opts.Args...),
			Cwd:     opts.Cwd,
			LogFile: c.logPath,
		}
		return StartResult{}, fmt.Errorf("start: %w", err)
	}

	started := time.Now()
	c.cmd = cmd
	c.logFile = logFile
	c.doneCh = make(chan struct{})
	c.state = State{
		Active:    true,
		Status:    StatusRunning,
		Command:   opts.Command,
		Args:      append([]string(nil), opts.Args...),
		Cwd:       opts.Cwd,
		PID:       cmd.Process.Pid,
		StartedAt: &started,
		LogFile:   c.logPath,
	}

	// Fan-out stdout + stderr line-by-line into the log file and subscribers.
	var drain sync.WaitGroup
	drain.Add(2)
	go c.pump(&drain, stdoutR)
	go c.pump(&drain, stderrR)

	go c.wait(cmd, &drain)

	c.broadcastLocked(Event{Kind: "started", State: c.state})

	return StartResult{State: c.state, Replaced: replaced}, nil
}

// Stop terminates the current process group. It is safe to call when nothing
// is running.
func (c *Controller) Stop() error {
	c.mu.Lock()
	if c.cmd == nil || c.state.Status != StatusRunning {
		c.mu.Unlock()
		return nil
	}
	cmd := c.cmd
	doneCh := c.doneCh
	c.mu.Unlock()

	if err := killProcessGroup(cmd); err != nil {
		return fmt.Errorf("kill process group: %w", err)
	}

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		// Fall through; wait() will mark state whenever it fires.
	}
	return nil
}

// Close stops any running process and closes all subscriber channels.
// Safe to call multiple times.
func (c *Controller) Close() {
	_ = c.Stop()

	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.subscribers {
		close(ch)
		delete(c.subscribers, id)
	}
}

// Subscribe returns a channel that receives live events for the running
// process. Call unsubscribe() to stop. The channel is closed when the caller
// unsubscribes or when the controller is closed.
func (c *Controller) Subscribe() (<-chan Event, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subscribeLocked()
}

// SubscribeWithBacklog atomically returns the log-file contents written so
// far alongside a fresh live channel. Because registration and the file read
// happen under the controller mutex, no line appears in both the backlog and
// the channel. If maxBacklog > 0 and the file is larger, only the tail is
// returned, trimmed forward to a line boundary.
func (c *Controller) SubscribeWithBacklog(maxBacklog int) ([]byte, <-chan Event, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, unsubscribe := c.subscribeLocked()

	var backlog []byte
	if c.state.LogFile != "" {
		if data, err := os.ReadFile(c.state.LogFile); err == nil {
			if maxBacklog > 0 && len(data) > maxBacklog {
				data = trimToLineBoundary(data[len(data)-maxBacklog:])
			}
			backlog = data
		}
	}

	return backlog, ch, unsubscribe
}

// subscribeLocked registers a new subscriber. c.mu must be held.
func (c *Controller) subscribeLocked() (<-chan Event, func()) {
	id := c.nextSubID
	c.nextSubID++
	ch := make(chan Event, subscriberBuf)
	c.subscribers[id] = ch

	unsubscribe := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if existing, ok := c.subscribers[id]; ok {
			delete(c.subscribers, id)
			close(existing)
		}
	}
	return ch, unsubscribe
}

// trimToLineBoundary drops bytes before the first newline so the returned
// slice starts at a fresh line. Returns nil if no newline is present.
func trimToLineBoundary(b []byte) []byte {
	for i, ch := range b {
		if ch == '\n' {
			return b[i+1:]
		}
	}
	return nil
}

// ReadOutput returns a slice of the log file. If TailLines > 0, returns the
// last N lines. Otherwise, returns bytes starting at SinceBytes up to
// MaxBytes.
func (c *Controller) ReadOutput(opts ReadOptions) (ReadResult, error) {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = defaultMaxRead
	}

	c.mu.Lock()
	state := c.state
	c.mu.Unlock()

	if state.LogFile == "" {
		return ReadResult{State: state}, nil
	}

	f, err := os.Open(state.LogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadResult{State: state}, nil
		}
		return ReadResult{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ReadResult{}, err
	}
	size := info.Size()

	start := opts.SinceBytes
	if start < 0 || start > size {
		start = 0
	}

	// Tail mode: read the last MaxBytes, then keep only the final TailLines.
	if opts.TailLines > 0 {
		tailSize := int64(opts.MaxBytes)
		if tailSize > size {
			tailSize = size
		}
		start = size - tailSize
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return ReadResult{}, err
	}

	buf := make([]byte, opts.MaxBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return ReadResult{}, err
	}
	content := string(buf[:n])
	endOffset := start + int64(n)
	truncated := endOffset < size

	if opts.TailLines > 0 {
		content = lastLines(content, opts.TailLines)
	}

	return ReadResult{
		State:       state,
		Content:     content,
		StartOffset: start,
		EndOffset:   endOffset,
		Truncated:   truncated,
	}, nil
}

func (c *Controller) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Status == StatusRunning
}

func (c *Controller) pump(wg *sync.WaitGroup, r io.ReadCloser) {
	defer wg.Done()
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		text := scanner.Text()
		c.mu.Lock()
		if c.logFile != nil {
			_, _ = c.logFile.WriteString(text + "\n")
		}
		c.broadcastLocked(Event{Kind: "output", Line: text})
		c.mu.Unlock()
	}
}

func (c *Controller) wait(cmd *exec.Cmd, drain *sync.WaitGroup) {
	runErr := cmd.Wait()
	drain.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	exited := time.Now()
	c.state.Active = false
	c.state.ExitedAt = &exited

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	c.state.ExitCode = &exitCode

	switch {
	case cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1:
		c.state.Status = StatusKilled
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			if exitErr.ProcessState != nil && !exitErr.ProcessState.Success() {
				c.state.Status = StatusExited
			} else {
				c.state.Status = StatusFailed
			}
		} else {
			c.state.Status = StatusFailed
		}
	default:
		c.state.Status = StatusExited
	}

	if c.logFile != nil {
		_ = c.logFile.Sync()
		_ = c.logFile.Close()
		c.logFile = nil
	}

	c.broadcastLocked(Event{Kind: "stopped", State: c.state})
	if c.doneCh != nil {
		close(c.doneCh)
		c.doneCh = nil
	}
}

// broadcastLocked must be called with c.mu held.
func (c *Controller) broadcastLocked(ev Event) {
	for id, ch := range c.subscribers {
		select {
		case ch <- ev:
		default:
			// Slow subscriber: drop to avoid blocking the pump. The client
			// should reconnect and re-read the log file if it needs catch-up.
			_ = id
		}
	}
}

// lastLines returns the final n lines of s, joined with their original
// newlines. Trailing empty fragment after the final "\n" is dropped.
func lastLines(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return strings.Join(lines, "")
	}
	return strings.Join(lines[len(lines)-n:], "")
}
