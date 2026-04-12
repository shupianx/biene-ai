package builtins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"biene/internal/tools"
)

var readOnlyPrefixes = []string{
	"cat ", "cat\t", "head ", "tail ", "less ", "more ",
	"ls ", "ls\t", "ls\n", "ls",
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
	"curl ", "wget ",
}

const defaultBashTimeout = 120 * time.Second

// BashTool executes shell commands.
type BashTool struct {
	WorkDir string
}

func NewBashTool() *BashTool                { return &BashTool{} }
func NewBashToolInDir(dir string) *BashTool { return &BashTool{WorkDir: dir} }

func (t *BashTool) Name() string { return "Bash" }

func (t *BashTool) PermissionKey() tools.PermissionKey { return tools.PermissionWrite }

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

	_ = cmd.Run()

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
	const maxOutput = 50_000
	if len(result) > maxOutput {
		result = result[:maxOutput] + fmt.Sprintf("\n... [output truncated at %d chars]", maxOutput)
	}
	return result, nil
}
