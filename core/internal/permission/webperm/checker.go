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
	// ToolID identifies the originating tool_use block. Pre-warmed write
	// permission checks fire before the call's input is fully streamed, so
	// the UI uses this to correlate incoming tool_compose_progress events
	// with the pending dialog.
	ToolID string
	// Context is an optional tool-provided payload (marshalled as JSON) shown
	// to the user alongside the default dialog — for example, file collisions.
	Context json.RawMessage
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

// decisionEnvelope carries both the user's allow/deny choice and any
// resolution data the UI submitted alongside it.
type decisionEnvelope struct {
	decision   permission.Decision
	resolution json.RawMessage
}

type pendingDecision struct {
	permission tools.PermissionKey
	ch         chan decisionEnvelope
}

// NewChecker creates a permission checker for the web application.
func NewChecker(allowed tools.PermissionSet) *Checker {
	return &Checker{
		allowed: allowed,
		pending: make(map[string]pendingDecision),
	}
}

// Check blocks until the frontend resolves the request or ctx is cancelled.
// The second return value is resolution data the UI supplied with the decision.
func (c *Checker) Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, json.RawMessage, error) {
	key := tool.PermissionKey()
	if key == tools.PermissionNone {
		return true, nil, nil
	}
	type readOnlyChecker interface {
		ReadOnlyForInput(json.RawMessage) bool
	}
	if roc, ok := tool.(readOnlyChecker); ok && roc.ReadOnlyForInput(input) {
		return true, nil, nil
	}

	c.mu.Lock()
	allowed := c.allowed.Allows(key)
	c.mu.Unlock()
	if allowed {
		return true, nil, nil
	}

	var extraCtx json.RawMessage
	if provider, ok := tool.(tools.PermissionContextProvider); ok {
		if v, err := provider.PermissionContext(ctx, input); err != nil {
			return false, nil, fmt.Errorf("permission context for %s: %w", tool.Name(), err)
		} else if v != nil {
			if data, err := json.Marshal(v); err != nil {
				return false, nil, fmt.Errorf("marshal permission context for %s: %w", tool.Name(), err)
			} else {
				extraCtx = data
			}
		}
	}

	requestID := newRequestID()
	ch := make(chan decisionEnvelope, 1)

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
			ToolID:      tools.ToolIDFromContext(ctx),
			Context:     extraCtx,
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
		return false, nil, ctx.Err()
	case env := <-ch:
		switch env.decision {
		case permission.DecisionAllow, permission.DecisionAlwaysAllow:
			return true, env.resolution, nil
		default:
			return false, nil, nil
		}
	}
}

// Resolve is called by the permission handler to unblock a pending Check.
// `resolution` is an optional UI-supplied payload that is forwarded into
// the tool's Execute context (see tools.WithPermissionResolution).
func (c *Checker) Resolve(requestID string, decision permission.Decision, resolution json.RawMessage) error {
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
	pending.ch <- decisionEnvelope{decision: decision, resolution: resolution}
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
