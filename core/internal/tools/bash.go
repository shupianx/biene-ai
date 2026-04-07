package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// readOnlyPrefixes lists command prefixes that are considered safe (read-only).
// Bash commands starting with any of these prefixes will skip the permission prompt.
var readOnlyPrefixes = []string{
	"cat ", "cat\t", "head ", "tail ", "less ", "more ",
	"ls ", "ls\t", "ls\n", "ls", // bare "ls" is safe
	"pwd", "echo ", "printf ",
	"grep ", "rg ", "ripgrep ",
	"find ", "fd ",
	"wc ", "sort ", "uniq ", "cut ", "awk ", "sed ",
	"diff ", "git diff", "git log", "git status", "git show",
	"which ", "type ", "file ",
	"env", "printenv",
	"ps ", "ps\n", "ps",
	"du ", "df ",
	"tree ",
	"jq ",
	"curl ", "wget ", // treat as read-only for permission (may still be risky)
}

const defaultBashTimeout = 120 * time.Second

// BashTool executes shell commands.
type BashTool struct {
	WorkDir string // if set, commands run with this as the working directory
}

func NewBashTool() *BashTool                { return &BashTool{} }
func NewBashToolInDir(dir string) *BashTool { return &BashTool{WorkDir: dir} }

func (t *BashTool) Name() string { return "Bash" }

func (t *BashTool) PermissionKey() PermissionKey { return PermissionWrite }

func (t *BashTool) Description() string {
	return `Execute a shell command in bash and return its stdout and stderr.
Use this for running tests, building projects, searching files, and any system operation.
Prefer specific tools (Read, Edit, Write) for file operations when possible.
Commands run in the user's current working directory.`
}

func (t *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The bash command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Optional timeout in seconds (default 120)"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) IsReadOnly() bool { return false } // decided per-invocation in Summary/Execute

// isReadOnlyCommand returns true when the command is heuristically safe to run
// without a permission prompt.
func isReadOnlyCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	for _, prefix := range readOnlyPrefixes {
		if trimmed == strings.TrimRight(prefix, " \t\n") || strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func parseBashInput(raw json.RawMessage) (bashInput, error) {
	var in bashInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return in, fmt.Errorf("invalid Bash input: %w", err)
	}
	if in.Command == "" {
		return in, fmt.Errorf("Bash: command is required")
	}
	return in, nil
}

func (t *BashTool) Summary(raw json.RawMessage) string {
	in, err := parseBashInput(raw)
	if err != nil {
		return "<invalid input>"
	}
	cmd := in.Command
	if len(cmd) > 80 {
		cmd = cmd[:77] + "..."
	}
	return cmd
}

// ReadOnly returns whether this specific invocation is read-only.
// Used by the permission layer to decide whether to prompt.
func (t *BashTool) ReadOnlyForInput(raw json.RawMessage) bool {
	in, err := parseBashInput(raw)
	if err != nil {
		return false
	}
	return isReadOnlyCommand(in.Command)
}

func (t *BashTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	in, err := parseBashInput(raw)
	if err != nil {
		return "", err
	}

	timeout := defaultBashTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	if t.WorkDir != "" {
		cmd.Dir = t.WorkDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // we include stderr in the result regardless of exit code

	var sb strings.Builder
	if stdout.Len() > 0 {
		sb.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr]\n")
		sb.WriteString(stderr.String())
	}
	if sb.Len() == 0 {
		sb.WriteString("(no output)")
	}

	result := sb.String()
	// Truncate very large outputs to prevent context bloat
	const maxOutput = 50_000
	if len(result) > maxOutput {
		result = result[:maxOutput] + fmt.Sprintf("\n... [output truncated at %d chars]", maxOutput)
	}
	return result, nil
}
