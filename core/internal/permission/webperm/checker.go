package webperm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"biene/internal/permission"
	"biene/internal/tools"
)

// PermissionRequest carries the data pushed to the frontend via the realtime channel.
type PermissionRequest struct {
	RequestID   string
	Permission  tools.PermissionKey
	ToolName    string
	ToolSummary string
	ToolInput   json.RawMessage
}

// Checker implements async permission confirmation via web callbacks.
type Checker struct {
	allowed tools.PermissionSet
	mu      sync.Mutex
	pending map[string]pendingDecision

	OnPermissionNeeded   func(PermissionRequest)
	OnPermissionSettled  func(requestID string)
	OnPermissionsChanged func(tools.PermissionSet)
}

type pendingDecision struct {
	permission tools.PermissionKey
	ch         chan permission.Decision
}

// NewChecker creates a permission checker for the web application.
func NewChecker(allowed tools.PermissionSet) *Checker {
	return &Checker{
		allowed: allowed,
		pending: make(map[string]pendingDecision),
	}
}

// Check blocks until the frontend resolves the request or ctx is cancelled.
func (c *Checker) Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, error) {
	key := tool.PermissionKey()
	if key == tools.PermissionNone {
		return true, nil
	}
	type readOnlyChecker interface {
		ReadOnlyForInput(json.RawMessage) bool
	}
	if roc, ok := tool.(readOnlyChecker); ok && roc.ReadOnlyForInput(input) {
		return true, nil
	}

	c.mu.Lock()
	allowed := c.allowed.Allows(key)
	c.mu.Unlock()
	if allowed {
		return true, nil
	}

	requestID := newRequestID()
	ch := make(chan permission.Decision, 1)

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
		case permission.DecisionAllow, permission.DecisionAlwaysAllow:
			return true, nil
		default:
			return false, nil
		}
	}
}

// Resolve is called by the permission handler to unblock a pending Check.
func (c *Checker) Resolve(requestID string, decision permission.Decision) error {
	c.mu.Lock()
	pending, ok := c.pending[requestID]
	delete(c.pending, requestID)
	var changed tools.PermissionSet
	shouldPersist := false
	if ok && decision == permission.DecisionAlwaysAllow {
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

func (c *Checker) SetPermissions(allowed tools.PermissionSet) {
	c.mu.Lock()
	c.allowed = allowed
	c.mu.Unlock()
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "perm_" + hex.EncodeToString(b)
}
