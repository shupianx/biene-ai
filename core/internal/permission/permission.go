package permission

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

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
func (c *Checker) Check(_ context.Context, tool tools.Tool, input json.RawMessage) (bool, error) {
	if c.autoMode || c.alwaysAllow[tool.Name()] {
		return true, nil
	}
	if tool.IsReadOnly() {
		return true, nil
	}
	type readOnlyChecker interface {
		ReadOnlyForInput(json.RawMessage) bool
	}
	if roc, ok := tool.(readOnlyChecker); ok && roc.ReadOnlyForInput(input) {
		return true, nil
	}

	summary := tool.Summary(input)
	fmt.Printf("\n\x1b[33m⚠  %s\x1b[0m  %s\n", tool.Name(), summary)
	fmt.Print("Allow? [\x1b[32my\x1b[0m=yes / \x1b[31mn\x1b[0m=no / \x1b[36ma\x1b[0m=always] ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading permission response: %w", err)
	}
	line = strings.TrimSpace(strings.ToLower(line))

	switch line {
	case "y", "yes", "":
		return true, nil
	case "a", "always":
		c.alwaysAllow[tool.Name()] = true
		return true, nil
	default:
		fmt.Println("Denied.")
		return false, nil
	}
}

// ─── HTTP Checker (Web mode) ──────────────────────────────────────────────

// PermissionRequest carries the data pushed to the frontend via SSE.
type PermissionRequest struct {
	RequestID   string
	Permission  tools.PermissionKey
	ToolName    string
	ToolSummary string
	ToolInput   json.RawMessage
}

// HTTPChecker implements async permission confirmation via HTTP callbacks.
// When a tool needs approval, it pushes a SSE event and blocks until the
// frontend responds via POST /api/permission.
type HTTPChecker struct {
	allowed tools.PermissionSet
	mu      sync.Mutex
	pending map[string]pendingDecision // requestID → decision channel

	// OnPermissionNeeded is set by the server layer. It is called (non-blocking)
	// to push a permission_request SSE event to the connected frontend.
	OnPermissionNeeded func(PermissionRequest)
	// OnPermissionSettled is called when a pending permission request is cleared
	// either by resolution or by context cancellation.
	OnPermissionSettled func(requestID string)
	// OnPermissionsChanged persists the current permission set after updates.
	OnPermissionsChanged func(tools.PermissionSet)
}

type pendingDecision struct {
	permission tools.PermissionKey
	ch         chan Decision
}

// NewHTTPChecker creates an HTTPChecker for Web server mode.
func NewHTTPChecker(allowed tools.PermissionSet) *HTTPChecker {
	return &HTTPChecker{
		allowed: allowed,
		pending: make(map[string]pendingDecision),
	}
}

// Check blocks until the frontend resolves the request (or ctx is cancelled).
func (c *HTTPChecker) Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, error) {
	if tool.IsReadOnly() {
		return true, nil
	}
	type readOnlyChecker interface {
		ReadOnlyForInput(json.RawMessage) bool
	}
	if roc, ok := tool.(readOnlyChecker); ok && roc.ReadOnlyForInput(input) {
		return true, nil
	}
	key := tool.PermissionKey()
	if key == tools.PermissionNone {
		return true, nil
	}

	c.mu.Lock()
	allowed := c.allowed.Allows(key)
	c.mu.Unlock()
	if allowed {
		return true, nil
	}

	requestID := newRequestID()
	ch := make(chan Decision, 1)

	c.mu.Lock()
	c.pending[requestID] = pendingDecision{
		permission: key,
		ch:         ch,
	}
	c.mu.Unlock()

	if c.OnPermissionNeeded != nil {
		c.OnPermissionNeeded(PermissionRequest{
			RequestID:   requestID,
			Permission:  key,
			ToolName:    tool.Name(),
			ToolSummary: tool.Summary(input),
			ToolInput:   input,
		})
	}

	// Block until frontend responds or context is cancelled.
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, requestID)
		c.mu.Unlock()
		if c.OnPermissionSettled != nil {
			c.OnPermissionSettled(requestID)
		}
		return false, ctx.Err()
	case decision := <-ch:
		switch decision {
		case DecisionAllow, DecisionAlwaysAllow:
			return true, nil
		default:
			return false, nil
		}
	}
}

// Resolve is called by the HTTP permission handler to unblock a pending Check.
func (c *HTTPChecker) Resolve(requestID string, decision Decision) error {
	c.mu.Lock()
	pending, ok := c.pending[requestID]
	delete(c.pending, requestID)
	var changed tools.PermissionSet
	shouldPersist := false
	if ok && decision == DecisionAlwaysAllow {
		c.allowed = c.allowed.With(pending.permission, true)
		changed = c.allowed
		shouldPersist = true
	}
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown or expired request_id: %q", requestID)
	}
	if shouldPersist && c.OnPermissionsChanged != nil {
		c.OnPermissionsChanged(changed)
	}
	if c.OnPermissionSettled != nil {
		c.OnPermissionSettled(requestID)
	}
	pending.ch <- decision
	return nil
}

func (c *HTTPChecker) SetPermissions(allowed tools.PermissionSet) {
	c.mu.Lock()
	c.allowed = allowed
	c.mu.Unlock()
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "perm_" + hex.EncodeToString(b)
}
