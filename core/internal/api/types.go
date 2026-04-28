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

// ReasoningBlock holds an assistant turn's chain-of-thought content that must
// be echoed back to the provider on subsequent turns (OpenAI-compatible
// `reasoning_content`, Anthropic `thinking` block). Signature is non-empty
// only for Anthropic, where the server-issued signature must be preserved
// verbatim.
type ReasoningBlock struct {
	Text      string
	Signature string
}

func (ReasoningBlock) blockType() string { return "reasoning" }

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

// ImageBlock carries an inline image attached to a user turn. Path is
// relative to the owning session's workspace and is what gets persisted;
// Data is the raw image bytes and is populated transiently by the caller
// just before handing the message to a provider, so DB rows stay small.
type ImageBlock struct {
	Path      string
	MediaType string
	Data      []byte
}

func (ImageBlock) blockType() string { return "image" }

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
	EventReasoningDelta  EventType = "reasoning_delta"
	EventSignatureDelta  EventType = "signature_delta"
	EventTextDelta       EventType = "text_delta"
	EventToolUseStart    EventType = "tool_use_start"
	EventInputJSONDelta  EventType = "input_json_delta" // partial JSON chunk for an in-flight tool_use
	EventToolUse         EventType = "tool_use"         // complete tool_use block ready
	EventUsage           EventType = "usage"            // accumulated token usage reported by provider
	EventDone            EventType = "done"
	EventError           EventType = "error"
)

// Usage reports token consumption for one provider call. InputTokens is
// the count the API charged for inputs (system + messages + tools schema)
// and is what compaction triggers compare against the model's
// context window. OutputTokens covers everything the model produced.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// StreamEvent is emitted by Provider.Stream for each incremental update.
type StreamEvent struct {
	Type      EventType
	Text      string        // EventReasoningDelta / EventTextDelta
	ToolUseID string        // EventInputJSONDelta — identifies which in-flight tool_use
	InputJSON string        // EventInputJSONDelta — partial JSON chunk
	ToolUse   *ToolUseBlock // EventToolUseStart / EventToolUse
	Usage     Usage         // EventUsage
	Err       error         // EventError
}

// RequestOptions carries provider-specific runtime controls expressed in
// provider-agnostic terms.
//
// ThinkingExtra is a JSON fragment that the provider shallow-merges into
// the top-level of the chat request body. The fragment's shape is defined
// by the provider (Qwen: {"enable_thinking": true}; Kimi: {"thinking":
// {"type": "enabled"}}) and supplied by config, so the core doesn't need
// to know dialect specifics.
type RequestOptions struct {
	ThinkingExtra map[string]any
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
		opts RequestOptions,
	) (<-chan StreamEvent, error)

	// Name returns a human-readable identifier for logging.
	Name() string
}
