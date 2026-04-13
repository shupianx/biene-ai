package agentloop

import (
	"context"
	"encoding/json"

	"biene/internal/api"
	"biene/internal/tools"
)

// EventKind classifies an Event.
type EventKind string

const (
	KindTextDelta   EventKind = "text_delta"
	KindToolCompose EventKind = "tool_compose"
	KindToolStart   EventKind = "tool_start"
	KindToolResult  EventKind = "tool_result"
	KindToolDenied  EventKind = "tool_denied"
	KindInterrupted EventKind = "interrupted"
	KindDone        EventKind = "done"
	KindError       EventKind = "error"
)

// Event is a single update emitted to the caller.
type Event struct {
	Kind        EventKind
	Text        string
	ToolID      string
	ToolName    string
	ToolSummary string
	ToolInput   json.RawMessage
	IsError     bool
}

// PermissionChecker decides whether a tool call is allowed to proceed.
// Both permission.Checker (CLI) and webperm.Checker (Web) implement this.
type PermissionChecker interface {
	Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, error)
}

// Config holds everything needed for a single conversational turn.
type Config struct {
	Provider     api.Provider
	Registry     *tools.Registry
	Checker      PermissionChecker
	SystemPrompt string
	Messages     []api.Message
	MaxTokens    int
}
