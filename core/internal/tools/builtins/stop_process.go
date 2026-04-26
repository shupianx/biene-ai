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

Call this only when (a) the user explicitly asked you to stop, or (b) the process is genuinely stuck — silent for a long time AND not waiting for user input (e.g., a known infinite loop, a deadlock, a build that has clearly hung with no progress).

DO NOT call this just because the process is paused on an interactive prompt. The user types directly into the process via the process panel — "Project name?" or "Select a framework:" is for the user to answer, not for you to abandon. A scaffolder waiting on a prompt is NOT stuck.

DO NOT call this to "clean up" after a task looks done. Long-running processes — dev servers (npm run dev, vite, webpack-serve), file watchers, build daemons — are supposed to stay running so the user can interact with them. Stopping them defeats their purpose; the user wants to open the URL in their browser or keep watching the terminal. A dev server printed its URL and is now idle waiting for requests is NOT stuck.

DO NOT call this before starting a replacement process. start_process automatically replaces any previous background process in the same session, so stopping first is redundant.

DO NOT call this just because a short interactive command (npm create, git commit, etc.) finished. Those exit on their own and the process slot is already empty.

Returns the final status and exit code. No-op if nothing is running.`
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
