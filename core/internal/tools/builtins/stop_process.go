package builtins

import (
	"context"
	"encoding/json"
	"fmt"

	"biene/internal/processes"
	"biene/internal/tools"
)

// StopProcessTool terminates the session's background process.
type StopProcessTool struct {
	Controller *processes.Controller
}

func NewStopProcessTool(ctrl *processes.Controller) *StopProcessTool {
	return &StopProcessTool{Controller: ctrl}
}

func (t *StopProcessTool) Name() string { return "stop_process" }

func (t *StopProcessTool) PermissionKey() tools.PermissionKey { return tools.PermissionExecute }

func (t *StopProcessTool) Description() string {
	return `Stop the session's background process, if one is running.
Returns the process's final status and exit code. No-op if nothing is running.`
}

func (t *StopProcessTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{ "type": "object", "properties": {} }`)
}

func (t *StopProcessTool) Summary(_ json.RawMessage) string {
	return "stop background process"
}

func (t *StopProcessTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	if t.Controller == nil {
		return "", fmt.Errorf("stop_process: no controller registered")
	}
	before := t.Controller.State()
	if !before.Active {
		return "no background process is running", nil
	}
	if err := t.Controller.Stop(); err != nil {
		return "", err
	}
	after := t.Controller.State()
	msg := fmt.Sprintf("Stopped pid=%d (%s)", before.PID, after.Status)
	if after.ExitCode != nil {
		msg += fmt.Sprintf(", exit=%d", *after.ExitCode)
	}
	return msg, nil
}
