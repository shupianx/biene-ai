package builtins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"biene/internal/tools"
)

const defaultCommandTimeout = 120 * time.Second
const maxCommandOutput = 50_000

// RunCommandTool executes a single command inside the agent workspace.
type RunCommandTool struct {
	WorkDir string
}

func NewRunCommandTool() *RunCommandTool                { return &RunCommandTool{} }
func NewRunCommandToolInDir(dir string) *RunCommandTool { return &RunCommandTool{WorkDir: dir} }

func (t *RunCommandTool) Name() string { return "run_command" }

func (t *RunCommandTool) PermissionKey() tools.PermissionKey { return tools.PermissionExecute }

func (t *RunCommandTool) Description() string {
	return `Run a short, non-interactive workspace command and return its stdout and stderr in-line.
Appropriate for builds, tests, linters, formatters, and CLIs that complete on their own without reading from stdin or redrawing their UI.

DO NOT use this for interactive commands. Without a TTY they hang waiting for input or bail out with "stdin is not a TTY". This includes scaffolders and wizards (npm create, pnpm create, yarn create, create-next-app without --yes, most npm init <generator>), commit editors (git commit without -m, git rebase -i), and full-screen terminal programs (vim, nano, less, top, htop). Use start_process for any of those — even when they are short.

DO NOT use this for long-running processes (dev servers, watchers, build daemons) — they never return, the call just hangs. Use start_process.

This tool runs a single executable plus argument list in the agent workspace. It does not support shell syntax: no pipes, redirects, glob expansion, command chaining, or environment variable assignments.

Quick decision:
  interactive / prompts the user   → start_process
  long-running (dev server, watch) → start_process
  short + quiet + need stdout      → run_command

Prefer list_files/read_file for inspection and write_file/edit_file for direct file changes.`
}

func (t *RunCommandTool) InputSchema() json.RawMessage {
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
				"description": "Optional working directory for the command, relative to the agent workspace (e.g. \"frontend\"). Must stay inside the workspace. Defaults to the workspace root."
			},
			"timeout": {
				"type": "integer",
				"description": "Optional timeout in seconds (default 120)"
			}
		},
		"required": ["command"]
	}`)
}

type runCommandInput struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
	Timeout int      `json:"timeout"`
}

func parseRunCommandInput(raw json.RawMessage) (runCommandInput, error) {
	var in runCommandInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return in, fmt.Errorf("invalid run_command input: %w", err)
	}
	if in.Command == "" {
		return in, fmt.Errorf("run_command: command is required")
	}
	for i, arg := range in.Args {
		if arg == "" {
			return in, fmt.Errorf("run_command: args[%d] must not be empty", i)
		}
	}
	return in, nil
}

func (t *RunCommandTool) Summary(raw json.RawMessage) string {
	in, err := parseRunCommandInput(raw)
	if err != nil {
		return "<invalid input>"
	}
	cmd := formatCommandPreview(in.Command, in.Args)
	if in.Cwd != "" {
		cmd = "[" + in.Cwd + "] " + cmd
	}
	if len(cmd) > 96 {
		cmd = cmd[:93] + "..."
	}
	return cmd
}

func (t *RunCommandTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	in, err := parseRunCommandInput(raw)
	if err != nil {
		return "", err
	}

	timeout := defaultCommandTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Second
	}

	workDir, err := resolveCommandCwd(t.WorkDir, in.Cwd)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, in.Command, in.Args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := formatCommandOutput(stdout.String(), stderr.String())

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return truncateCommandOutput(result + fmt.Sprintf("\n\n[timeout]\nCommand timed out after %ds.", int(timeout/time.Second))), nil
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(runErr, &exitErr):
			if result == "(no output)" {
				result = ""
			}
			if result != "" {
				result += "\n\n"
			}
			result += fmt.Sprintf("[exit code]\n%d", exitErr.ExitCode())
			return truncateCommandOutput(result), nil
		default:
			if result == "(no output)" {
				result = ""
			}
			if result != "" {
				result += "\n\n"
			}
			result += fmt.Sprintf("[error]\n%s", runErr)
			return truncateCommandOutput(result), nil
		}
	}
	return truncateCommandOutput(result), nil
}

// resolveCommandCwd returns the directory to run the command in.
// If cwd is empty, it falls back to workDir. If cwd is provided, it is
// resolved relative to workDir and must stay inside the workspace.
func resolveCommandCwd(workDir, cwd string) (string, error) {
	if cwd == "" {
		return workDir, nil
	}
	if workDir == "" {
		return "", fmt.Errorf("run_command: cwd not supported without a workspace root")
	}
	abs, _, err := resolvePath(workDir, cwd)
	if err != nil {
		return "", fmt.Errorf("run_command: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("run_command: cwd %q: %w", cwd, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("run_command: cwd %q is not a directory", cwd)
	}
	return abs, nil
}

func formatCommandPreview(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteCommandPart(command))
	for _, arg := range args {
		parts = append(parts, quoteCommandPart(arg))
	}
	return strings.Join(parts, " ")
}

func quoteCommandPart(part string) string {
	if part == "" {
		return `""`
	}
	if strings.ContainsAny(part, " \t\n\"'") {
		return strconv.Quote(part)
	}
	return part
}

func formatCommandOutput(stdout, stderr string) string {
	var sb strings.Builder
	if strings.TrimSpace(stdout) != "" {
		sb.WriteString(stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr]\n")
		sb.WriteString(stderr)
	}
	if sb.Len() == 0 {
		return "(no output)"
	}
	return sb.String()
}

func truncateCommandOutput(result string) string {
	if len(result) <= maxCommandOutput {
		return result
	}
	return result[:maxCommandOutput] + fmt.Sprintf("\n... [output truncated at %d chars]", maxCommandOutput)
}
