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
//
// CacheReadTokens / CacheWriteTokens are the prompt-cache-hit / write
// portions in the same dimension as InputTokens. Providers that expose
// prompt caching (Anthropic native, OpenAI Codex via prompt_cache_key)
// fill them in; others leave them at zero. They are *informational only*
// — compaction still keys off InputTokens, which is already the
// non-cached portion (see chatgpt_codex.go for the subtraction). Surface
// these fields in the UI to show the user how much of their context is
// served from cache.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
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
//
// SessionID is the stable identifier of the originating Biene session.
// Providers that support prompt-cache affinity (Codex backend's
// `prompt_cache_key` + `session_id` header) use it to keep consecutive
// turns of one conversation on the same cache shard. Empty string means
// "don't use cache affinity" — the request still works, just colder.
//
// ServiceTier is the OpenAI Codex `service_tier` knob. Empty string =
// upstream default; recognised values are "default", "flex" (≈0.5×
// pricing, slower), "priority" (≈2×–2.5×, faster), "scale", "auto".
// Only the ChatGPTCodex provider currently consumes it; other providers
// ignore the field.
type RequestOptions struct {
	ThinkingExtra map[string]any
	SessionID     string
	ServiceTier   string
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
