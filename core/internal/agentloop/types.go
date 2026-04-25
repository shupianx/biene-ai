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
	KindReasoningDelta      EventKind = "reasoning_delta"
	KindTextDelta           EventKind = "text_delta"
	KindToolCompose         EventKind = "tool_compose"
	KindToolComposeProgress EventKind = "tool_compose_progress"
	KindToolStart           EventKind = "tool_start"
	KindToolResult          EventKind = "tool_result"
	KindToolDenied          EventKind = "tool_denied"
	KindInterrupted         EventKind = "interrupted"
	KindDone                EventKind = "done"
	KindError               EventKind = "error"
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

	// KindToolComposeProgress payload — fields are populated as they become
	// known from the streaming tool_use input JSON.
	FilePath      string // write/edit: path whose value has fully arrived
	FileTextBytes int    // write: approximate bytes of file_text seen so far
}

// PermissionChecker decides whether a tool call is allowed to proceed.
// Both permission.Checker (CLI) and webperm.Checker (Web) implement this.
//
// The second return value carries optional resolution data the user supplied
// alongside the decision (for example, a collision-handling strategy). It is
// forwarded into the tool's Execute via tools.WithPermissionResolution.
type PermissionChecker interface {
	Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, json.RawMessage, error)
}

// Config holds everything needed for a single conversational turn.
type Config struct {
	Provider     api.Provider
	Registry     *tools.Registry
	Checker      PermissionChecker
	SystemPrompt string
	Messages     []api.Message
	RequestOpts  api.RequestOptions

	// SessionID is optional and only used for log enrichment.
	SessionID string
}
