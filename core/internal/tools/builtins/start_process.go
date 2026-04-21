package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/processes"
	"biene/internal/tools"
)

// StartProcessTool launches a long-running background process for the
// current agent session. Only one background process per session is
// allowed; calling this when another process is running will stop the
// previous one first.
type StartProcessTool struct {
	WorkDir    string
	Controller *processes.Controller
}

func NewStartProcessTool(workDir string, ctrl *processes.Controller) *StartProcessTool {
	return &StartProcessTool{WorkDir: workDir, Controller: ctrl}
}

func (t *StartProcessTool) Name() string { return "start_process" }

func (t *StartProcessTool) PermissionKey() tools.PermissionKey { return tools.PermissionExecute }

func (t *StartProcessTool) Description() string {
	return `Start a long-running background process (dev servers, watchers, build daemons).
Each agent session has exactly one background process slot. Starting a new process automatically stops the previous one.
Use read_process_output to inspect logs while it runs, and stop_process to terminate it explicitly.
This tool does not support shell syntax (no pipes, redirects, globs, or variable expansion). Pass environment variables via the env object.
For short one-shot commands whose output you need in-line, use run_command instead.`
}

func (t *StartProcessTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "Executable name or absolute path to run"
			},
			"args": {
				"type": "array",
				"description": "Arguments passed to the command. Do not include shell syntax here.",
				"items": { "type": "string" }
			},
			"cwd": {
				"type": "string",
				"description": "Optional working directory relative to the agent workspace. Defaults to the workspace root."
			},
			"env": {
				"type": "object",
				"description": "Optional environment variables merged onto the agent's own env.",
				"additionalProperties": { "type": "string" }
			}
		},
		"required": ["command"]
	}`)
}

type startProcessInput struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd"`
	Env     map[string]string `json:"env"`
}

func (t *StartProcessTool) Summary(raw json.RawMessage) string {
	var in startProcessInput
	_ = json.Unmarshal(raw, &in)
	cmd := formatCommandPreview(in.Command, in.Args)
	if in.Cwd != "" {
		cmd = "[" + in.Cwd + "] " + cmd
	}
	if len(cmd) > 96 {
		cmd = cmd[:93] + "..."
	}
	return cmd
}

func (t *StartProcessTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in startProcessInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("start_process: invalid input: %w", err)
	}
	if strings.TrimSpace(in.Command) == "" {
		return "", fmt.Errorf("start_process: command is required")
	}
	if t.Controller == nil {
		return "", fmt.Errorf("start_process: no controller registered")
	}

	cwd, err := resolveCommandCwd(t.WorkDir, in.Cwd)
	if err != nil {
		return "", err
	}

	res, err := t.Controller.Start(processes.StartOptions{
		Command: in.Command,
		Args:    in.Args,
		Cwd:     cwd,
		Env:     in.Env,
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	if res.Replaced {
		b.WriteString("[info] Previous background process was stopped.\n")
	}
	fmt.Fprintf(&b, "Started pid=%d: %s", res.State.PID, formatCommandPreview(in.Command, in.Args))
	if in.Cwd != "" {
		fmt.Fprintf(&b, " (cwd: %s)", in.Cwd)
	}
	b.WriteString("\nUse read_process_output to tail logs. Use stop_process to terminate.")
	return b.String(), nil
}
