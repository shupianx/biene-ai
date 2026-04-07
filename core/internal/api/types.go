package api

import (
	"context"
	"encoding/json"
)

// ─── Internal message types (provider-agnostic) ───────────────────────────

// Role values for Message.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ContentBlock is the sealed union of block types in a message.
type ContentBlock interface {
	blockType() string
}

// TextBlock holds plain text content.
type TextBlock struct {
	Text string
}

func (TextBlock) blockType() string { return "text" }

// ToolUseBlock represents a model-requested tool call.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input json.RawMessage
}

func (ToolUseBlock) blockType() string { return "tool_use" }

// ToolResultBlock carries the result of a tool execution back to the model.
type ToolResultBlock struct {
	ToolUseID string
	Content   string
	IsError   bool
}

func (ToolResultBlock) blockType() string { return "tool_result" }

// Message is a single turn in the conversation history.
type Message struct {
	Role    string
	Content []ContentBlock
}

// ToolDefinition describes a tool that the model can call.
type ToolDefinition struct {
	Name        string
	Description string
	// InputSchema is a JSON Schema object describing the tool's input parameters.
	InputSchema json.RawMessage
}

// ─── Streaming event types ────────────────────────────────────────────────

// EventType classifies a StreamEvent.
type EventType string

const (
	EventTextDelta    EventType = "text_delta"
	EventToolUseStart EventType = "tool_use_start"
	EventToolUse      EventType = "tool_use" // complete tool_use block ready
	EventDone         EventType = "done"
	EventError        EventType = "error"
)

// StreamEvent is emitted by Provider.Stream for each incremental update.
type StreamEvent struct {
	Type    EventType
	Text    string        // EventTextDelta
	ToolUse *ToolUseBlock // EventToolUseStart / EventToolUse
	Err     error         // EventError
}

// ─── Provider interface ───────────────────────────────────────────────────

// Provider abstracts over different LLM backends.
// Each implementation converts between the internal types above and the
// backend-specific wire format.
type Provider interface {
	// Stream sends a chat request and returns a channel of streaming events.
	// The channel is closed after EventDone or EventError is sent.
	Stream(
		ctx context.Context,
		systemPrompt string,
		messages []Message,
		tools []ToolDefinition,
		maxTokens int,
	) (<-chan StreamEvent, error)

	// Name returns a human-readable identifier for logging.
	Name() string
}
