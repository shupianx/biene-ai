package builtins

import (
	"context"
	"encoding/json"
	"fmt"

	"tinte/internal/processes"
	"tinte/internal/tools"
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

Call this only when either (a) the user explicitly asked you to stop it, or (b) the process had a one-shot goal that is now clearly complete AND the process is actually still running (e.g., a scaffolder hanging on a prompt you need to abandon).

DO NOT call this to "clean up" after a task looks done. Long-running processes — dev servers (npm run dev, vite, webpack-serve), file watchers, build daemons — are supposed to stay running so the user can interact with them. Stopping them defeats their purpose; the user wants to open the URL in their browser or keep watching the terminal. If you just scaffolded a project and started a dev server, leave it running and tell the user about the URL.

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
