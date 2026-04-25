package permission

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"biene/internal/tools"
)

// Decision is the user's response to a permission prompt.
type Decision int

const (
	DecisionAllow       Decision = iota // allow once
	DecisionAlwaysAllow                 // allow and persist this permission group
	DecisionDeny                        // deny this call
)

// ─── CLI Checker (stdin) ─────────────────────────────────────────────────

// Checker manages per-session always-allow overrides via stdin prompts.
type Checker struct {
	alwaysAllow map[string]bool
	autoMode    bool
}

// NewChecker creates a permission checker for CLI use.
func NewChecker(autoMode bool) *Checker {
	return &Checker{
		alwaysAllow: make(map[string]bool),
		autoMode:    autoMode,
	}
}

// Check decides whether a tool call should proceed, prompting via stdin if needed.
// The CLI checker never returns resolution data.
func (c *Checker) Check(_ context.Context, tool tools.Tool, input json.RawMessage) (bool, json.RawMessage, error) {
	if c.autoMode || c.alwaysAllow[tool.Name()] {
		return true, nil, nil
	}
	if tool.PermissionKey() == tools.PermissionNone {
		return true, nil, nil
	}
	type readOnlyChecker interface {
		ReadOnlyForInput(json.RawMessage) bool
	}
	if roc, ok := tool.(readOnlyChecker); ok && roc.ReadOnlyForInput(input) {
		return true, nil, nil
	}

	summary := tool.Summary(input)
	fmt.Printf("\n\x1b[33m⚠  %s\x1b[0m  %s\n", tool.Name(), summary)
	fmt.Print("Allow? [\x1b[32my\x1b[0m=yes / \x1b[31mn\x1b[0m=no / \x1b[36ma\x1b[0m=always] ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, nil, fmt.Errorf("reading permission response: %w", err)
	}
	line = strings.TrimSpace(strings.ToLower(line))

	switch line {
	case "y", "yes", "":
		return true, nil, nil
	case "a", "always":
		c.alwaysAllow[tool.Name()] = true
		return true, nil, nil
	default:
		fmt.Println("Denied.")
		return false, nil, nil
	}
}
