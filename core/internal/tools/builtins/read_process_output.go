package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/processes"
	"biene/internal/tools"
)

// ReadProcessOutputTool returns the recent log output of the session's
// single background process. Safe to call whether or not a process is
// currently running (it reads the log file, which persists past exit).
type ReadProcessOutputTool struct {
	Controller *processes.Controller
}

func NewReadProcessOutputTool(ctrl *processes.Controller) *ReadProcessOutputTool {
	return &ReadProcessOutputTool{Controller: ctrl}
}

func (t *ReadProcessOutputTool) Name() string { return "read_process_output" }

func (t *ReadProcessOutputTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ReadProcessOutputTool) Description() string {
	return `Read the log output of the current (or most recent) background process in this session.
Returns a status header (command, pid, status, exit code) followed by log content. If tail_lines is set, only the final N lines are returned. Otherwise the most recent chunk (up to max_bytes) is returned.
Call this to check on a dev server's progress, verify a watcher picked up a change, or capture a stack trace after a crash.`
}

func (t *ReadProcessOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"tail_lines": {
				"type": "integer",
				"description": "If set, return only the last N lines. Defaults to 100 when tail_lines and since_bytes are both omitted."
			},
			"since_bytes": {
				"type": "integer",
				"description": "Return log content starting at this byte offset. Use the end_offset from a previous call to tail incrementally."
			},
			"max_bytes": {
				"type": "integer",
				"description": "Maximum bytes to return. Defaults to 32768."
			}
		}
	}`)
}

type readProcessOutputInput struct {
	TailLines  int `json:"tail_lines"`
	SinceBytes int `json:"since_bytes"`
	MaxBytes   int `json:"max_bytes"`
}

func (t *ReadProcessOutputTool) Summary(raw json.RawMessage) string {
	var in readProcessOutputInput
	_ = json.Unmarshal(raw, &in)
	switch {
	case in.TailLines > 0:
		return fmt.Sprintf("tail %d lines", in.TailLines)
	case in.SinceBytes > 0:
		return fmt.Sprintf("since offset %d", in.SinceBytes)
	default:
		return "read output"
	}
}

func (t *ReadProcessOutputTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in readProcessOutputInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("read_process_output: invalid input: %w", err)
	}
	if t.Controller == nil {
		return "", fmt.Errorf("read_process_output: no controller registered")
	}

	if in.TailLines <= 0 && in.SinceBytes <= 0 {
		in.TailLines = 100
	}

	res, err := t.Controller.ReadOutput(processes.ReadOptions{
		TailLines:  in.TailLines,
		SinceBytes: int64(in.SinceBytes),
		MaxBytes:   in.MaxBytes,
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[status]\n")
	if res.State.Command == "" {
		b.WriteString("no process has been started\n")
		return b.String(), nil
	}
	fmt.Fprintf(&b, "command: %s\n", formatCommandPreview(res.State.Command, res.State.Args))
	fmt.Fprintf(&b, "status:  %s\n", res.State.Status)
	if res.State.PID > 0 {
		fmt.Fprintf(&b, "pid:     %d\n", res.State.PID)
	}
	if res.State.ExitCode != nil {
		fmt.Fprintf(&b, "exit:    %d\n", *res.State.ExitCode)
	}
	fmt.Fprintf(&b, "offset:  %d\n", res.EndOffset)

	b.WriteString("\n[output]\n")
	if res.Content == "" {
		b.WriteString("(empty)")
	} else {
		b.WriteString(res.Content)
	}
	if res.Truncated {
		b.WriteString("\n... [output truncated; call again with since_bytes to continue]")
	}
	return b.String(), nil
}
