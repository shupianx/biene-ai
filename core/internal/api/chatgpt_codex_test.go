package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

// ── Test helpers ─────────────────────────────────────────────────────────

// drainTracker creates a buffered tracker, plays the given event sequence
// through handle(), then calls flush() and returns every emitted
// StreamEvent. The buffer is sized large enough that no producer will
// block in any test case.
func drainTracker(events []responses.ResponseStreamEventUnion) []StreamEvent {
	out := make(chan StreamEvent, 256)
	tracker := newCodexStreamTracker(out)
	for _, ev := range events {
		tracker.handle(ev)
	}
	tracker.flush()
	close(out)
	collected := make([]StreamEvent, 0)
	for ev := range out {
		collected = append(collected, ev)
	}
	return collected
}

// ── Reasoning + text deltas ──────────────────────────────────────────────

func TestCodexTracker_ReasoningSummary_EmitsDeltasAndParagraphBreak(t *testing.T) {
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{Type: "response.output_item.added", Item: responses.ResponseOutputItemUnion{Type: "reasoning"}},
		{Type: "response.reasoning_summary_text.delta", Delta: "first part"},
		// Per onItemAdded comments, between summary parts we insert
		// a paragraph break so the rendered thought reads as one
		// continuous block instead of two glued fragments.
		{Type: "response.reasoning_summary_part.done"},
		{Type: "response.reasoning_summary_text.delta", Delta: "second part"},
		{Type: "response.output_item.done", Item: responses.ResponseOutputItemUnion{Type: "reasoning"}},
	})

	want := []StreamEvent{
		{Type: EventReasoningDelta, Text: "first part"},
		{Type: EventReasoningDelta, Text: "\n\n"},
		{Type: EventReasoningDelta, Text: "second part"},
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected event count: got %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Text != want[i].Text {
			t.Errorf("event %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCodexTracker_TextDelta_AndRefusalSurfacesAsText(t *testing.T) {
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{Type: "response.output_item.added", Item: responses.ResponseOutputItemUnion{Type: "message"}},
		{Type: "response.output_text.delta", Delta: "hello "},
		// Refusals are intentionally surfaced through the same text
		// channel so the user sees the model's apology rather than a
		// silent empty turn (see chatgpt_codex.go).
		{Type: "response.refusal.delta", Delta: "I cannot help with that."},
		{Type: "response.output_text.delta", Delta: ""}, // empty deltas should be dropped
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 text events (empty delta should be dropped), got %d: %+v", len(got), got)
	}
	if got[0].Type != EventTextDelta || got[0].Text != "hello " {
		t.Errorf("first event mismatch: %+v", got[0])
	}
	if got[1].Type != EventTextDelta || got[1].Text != "I cannot help with that." {
		t.Errorf("second event mismatch: %+v", got[1])
	}
}

// ── Function call lifecycle ──────────────────────────────────────────────

func TestCodexTracker_FunctionCall_FullLifecycle(t *testing.T) {
	events := []responses.ResponseStreamEventUnion{
		{
			Type: "response.output_item.added",
			Item: responses.ResponseOutputItemUnion{
				Type:   "function_call",
				ID:     "fc_item_42",
				CallID: "call_abc",
				Name:   "list_files",
			},
		},
		{Type: "response.function_call_arguments.delta", Delta: `{"path":`},
		{Type: "response.function_call_arguments.delta", Delta: ` "/tmp"}`},
		// The .done event carries the canonical full args. The tracker
		// is supposed to overwrite its accumulated buffer with this so
		// downstream parsers don't see e.g. truncated/duplicated text
		// if one of the deltas was lost or re-fragmented.
		{Type: "response.function_call_arguments.done", Arguments: `{"path": "/tmp"}`},
		{
			Type: "response.output_item.done",
			Item: responses.ResponseOutputItemUnion{Type: "function_call"},
		},
	}
	got := drainTracker(events)

	// Expected sequence:
	//   tool_use_start, input_json_delta×2, tool_use(final)
	if len(got) != 4 {
		t.Fatalf("unexpected event count: %d (%+v)", len(got), got)
	}
	wantID := "call_abc|fc_item_42"

	if got[0].Type != EventToolUseStart || got[0].ToolUse == nil {
		t.Fatalf("event[0] should be tool_use_start, got %+v", got[0])
	}
	if got[0].ToolUse.ID != wantID {
		t.Errorf("composite id mismatch: got %q, want %q", got[0].ToolUse.ID, wantID)
	}
	if got[0].ToolUse.Name != "list_files" {
		t.Errorf("name mismatch: %q", got[0].ToolUse.Name)
	}

	if got[1].Type != EventInputJSONDelta || got[1].InputJSON != `{"path":` || got[1].ToolUseID != wantID {
		t.Errorf("event[1] (delta1) mismatch: %+v", got[1])
	}
	if got[2].Type != EventInputJSONDelta || got[2].InputJSON != ` "/tmp"}` || got[2].ToolUseID != wantID {
		t.Errorf("event[2] (delta2) mismatch: %+v", got[2])
	}

	if got[3].Type != EventToolUse || got[3].ToolUse == nil {
		t.Fatalf("event[3] should be tool_use final, got %+v", got[3])
	}
	if got[3].ToolUse.ID != wantID {
		t.Errorf("final id mismatch: %q", got[3].ToolUse.ID)
	}
	// Args must be valid JSON and match the .done payload, not the
	// concatenated deltas (in practice they're the same; the test
	// guards against future drift between the two paths).
	if string(got[3].ToolUse.Input) != `{"path": "/tmp"}` {
		t.Errorf("final args mismatch: got %s", got[3].ToolUse.Input)
	}
	if !json.Valid(got[3].ToolUse.Input) {
		t.Errorf("final args not valid JSON: %s", got[3].ToolUse.Input)
	}
}

func TestCodexTracker_FunctionCall_MalformedArgsBecomeEmptyObject(t *testing.T) {
	// No .done event ever fires with a clean payload — the deltas land
	// as half-formed JSON. The tracker must still emit a parseable
	// EventToolUse so the downstream agentloop doesn't crash on a
	// fragment.
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{
			Type: "response.output_item.added",
			Item: responses.ResponseOutputItemUnion{
				Type: "function_call", ID: "i1", CallID: "c1", Name: "broken",
			},
		},
		{Type: "response.function_call_arguments.delta", Delta: `{"oops"`},
		{Type: "response.output_item.done", Item: responses.ResponseOutputItemUnion{Type: "function_call"}},
	})
	// tool_use_start, one input_json_delta, then tool_use with "{}"
	if len(got) != 3 {
		t.Fatalf("unexpected event count: %d (%+v)", len(got), got)
	}
	if got[2].Type != EventToolUse || string(got[2].ToolUse.Input) != "{}" {
		t.Errorf("malformed args should fall back to {}: got %+v", got[2])
	}
}

func TestCodexTracker_Flush_TruncatedFunctionCall(t *testing.T) {
	// A truncated stream — output_item.added arrives, deltas come in,
	// but neither .done event ever fires. flush() is the safety net
	// that emits a final tool_use anyway so the outer loop sees a
	// coherent end-of-turn.
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{
			Type: "response.output_item.added",
			Item: responses.ResponseOutputItemUnion{
				Type: "function_call", ID: "i9", CallID: "c9", Name: "interrupted",
			},
		},
		{Type: "response.function_call_arguments.delta", Delta: `{"x":1}`},
	})
	// tool_use_start, delta, then flush() emits the final tool_use.
	if len(got) != 3 {
		t.Fatalf("expected 3 events (start + delta + flushed final), got %d (%+v)", len(got), got)
	}
	if got[2].Type != EventToolUse {
		t.Fatalf("flushed event is not EventToolUse: %+v", got[2])
	}
	if got[2].ToolUse.ID != "c9|i9" || string(got[2].ToolUse.Input) != `{"x":1}` {
		t.Errorf("flushed tool_use payload mismatch: %+v", got[2].ToolUse)
	}
}

// ── Usage + error events ─────────────────────────────────────────────────

func TestCodexTracker_Completed_EmitsUsage_SubtractsCachedTokens(t *testing.T) {
	// InputTokens reported by the API includes cached prefix tokens,
	// but Biene's compaction logic compares against fresh-input cost
	// only. The tracker subtracts CachedTokens before emitting.
	usage := responses.ResponseUsage{
		InputTokens:        1200,
		OutputTokens:       300,
		InputTokensDetails: responses.ResponseUsageInputTokensDetails{CachedTokens: 1000},
	}
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{Type: "response.completed", Response: responses.Response{Usage: usage}},
	})
	if len(got) != 1 || got[0].Type != EventUsage {
		t.Fatalf("expected single usage event, got %+v", got)
	}
	if got[0].Usage.InputTokens != 200 {
		t.Errorf("input tokens should be net-of-cache (1200-1000=200), got %d", got[0].Usage.InputTokens)
	}
	if got[0].Usage.OutputTokens != 300 {
		t.Errorf("output tokens passthrough mismatch, got %d", got[0].Usage.OutputTokens)
	}
}

func TestCodexTracker_Error_EmitsEventError(t *testing.T) {
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{Type: "error", Message: "rate limit exceeded"},
	})
	if len(got) != 1 || got[0].Type != EventError {
		t.Fatalf("expected single error event, got %+v", got)
	}
	if got[0].Err == nil || got[0].Err.Error() != "rate limit exceeded" {
		t.Errorf("error message mismatch: %v", got[0].Err)
	}
}

func TestCodexTracker_Failed_EmptyMessage_FallsBackToGenericText(t *testing.T) {
	got := drainTracker([]responses.ResponseStreamEventUnion{
		{Type: "response.failed"},
	})
	if len(got) != 1 || got[0].Type != EventError {
		t.Fatalf("expected error event, got %+v", got)
	}
	if got[0].Err == nil || got[0].Err.Error() != "codex stream error" {
		t.Errorf("expected fallback message, got %v", got[0].Err)
	}
}

// ── Composite tool_use ID round-trip ─────────────────────────────────────

func TestSplitCodexToolUseID_Composite(t *testing.T) {
	callID, itemID := splitCodexToolUseID("call_abc|fc_item_42")
	if callID != "call_abc" || itemID != "fc_item_42" {
		t.Errorf("split mismatch: got (%q, %q)", callID, itemID)
	}
}

func TestSplitCodexToolUseID_LegacyIDWithoutPipe(t *testing.T) {
	// Tool-use blocks that originated from another provider during a
	// session migrated mid-conversation won't have the pipe form. They
	// should round-trip as call_id with an empty itemID.
	callID, itemID := splitCodexToolUseID("legacy_id_42")
	if callID != "legacy_id_42" || itemID != "" {
		t.Errorf("legacy id should round-trip as call_id only: got (%q, %q)", callID, itemID)
	}
}

// ── Message conversion ───────────────────────────────────────────────────

func TestConvertMessagesToResponsesInput_TextOnly_BecomesSingleMessageItem(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: []ContentBlock{TextBlock{Text: "hello"}, TextBlock{Text: "world"}}},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 input item (collapsed text), got %d", len(items))
	}
}

func TestConvertMessagesToResponsesInput_ToolUseAndResult_PreserveOrder(t *testing.T) {
	// One assistant turn that emits a tool_use, then a user turn that
	// returns the tool_result. The composite ID built by the tracker
	// must round-trip back into a Codex call_id (the Codex backend
	// keys results by call_id, not Biene's internal id).
	msgs := []Message{
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				TextBlock{Text: "let me check"},
				ToolUseBlock{
					ID:    "call_abc|fc_item_42",
					Name:  "list_files",
					Input: json.RawMessage(`{"path":"/tmp"}`),
				},
			},
		},
		{
			Role: RoleUser,
			Content: []ContentBlock{
				ToolResultBlock{ToolUseID: "call_abc|fc_item_42", Content: "[]"},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	// Expected items: (1) assistant message text, (2) function_call,
	// (3) function_call_output. Order matters — Responses API rejects
	// dangling outputs whose call_id hasn't been declared yet.
	if len(items) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(items))
	}
	if items[0].OfMessage == nil {
		t.Errorf("item[0] should be a message: %+v", items[0])
	}
	if items[1].OfFunctionCall == nil {
		t.Fatalf("item[1] should be a function_call: %+v", items[1])
	}
	if items[1].OfFunctionCall.CallID != "call_abc" {
		t.Errorf("function_call should use the call_id half of the composite, got %q",
			items[1].OfFunctionCall.CallID)
	}
	// Item-level ID is the OpenAI item handle from the stream — without
	// it the backend rejects the second-turn input on stateless mode.
	if items[1].OfFunctionCall.ID.Or("") != "fc_item_42" {
		t.Errorf("function_call should preserve item id: got %q",
			items[1].OfFunctionCall.ID.Or(""))
	}
	if items[2].OfFunctionCallOutput == nil {
		t.Fatalf("item[2] should be a function_call_output: %+v", items[2])
	}
	if items[2].OfFunctionCallOutput.CallID != "call_abc" {
		t.Errorf("function_call_output should also use call_id only, got %q",
			items[2].OfFunctionCallOutput.CallID)
	}
}

func TestConvertMessagesToResponsesInput_ReasoningWithoutSignatureIsDropped(t *testing.T) {
	// EncryptedContent is the only thing the Responses API can re-use
	// from a prior reasoning block on a stateless call. A reasoning
	// block with an empty signature carries no information the backend
	// can act on, so we drop it rather than send `OfReasoning` items
	// that would waste tokens / confuse the backend.
	msgs := []Message{
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				TextBlock{Text: "thought through it"},
				ReasoningBlock{Text: "internal", Signature: ""},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	for _, item := range items {
		if item.OfReasoning != nil {
			t.Errorf("reasoning without signature should not be emitted: %+v", item)
		}
	}
}

func TestConvertMessagesToResponsesInput_ReasoningWithSignatureRoundTrips(t *testing.T) {
	msgs := []Message{
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				ReasoningBlock{Text: "doesn't matter for codex", Signature: "ENCRYPTED_BLOB"},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(items) != 1 || items[0].OfReasoning == nil {
		t.Fatalf("expected single reasoning item, got %+v", items)
	}
	if items[0].OfReasoning.EncryptedContent.Or("") != "ENCRYPTED_BLOB" {
		t.Errorf("signature should round-trip into EncryptedContent, got %q",
			items[0].OfReasoning.EncryptedContent.Or(""))
	}
	// Silence unused-import lint when the SDK helper isn't otherwise
	// referenced in this test (param.NewOpt is only used in the
	// production code path we're exercising via reflection).
	_ = openai.NewClient
	_ = param.NewOpt[string]
}

// ── Tool conversion ─────────────────────────────────────────────────────

func TestConvertToolsToResponses_PassesThroughDescription(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file from disk",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
	}
	out := convertToolsToResponses(tools)
	if len(out) != 1 || out[0].OfFunction == nil {
		t.Fatalf("expected single function tool, got %+v", out)
	}
	if out[0].OfFunction.Name != "read_file" {
		t.Errorf("name mismatch: %q", out[0].OfFunction.Name)
	}
	if out[0].OfFunction.Description.Or("") != "Read a file from disk" {
		t.Errorf("description not propagated: %q", out[0].OfFunction.Description.Or(""))
	}
}

func TestConvertToolsToResponses_NilSchemaFallsBackToEmptyObject(t *testing.T) {
	// The Responses API rejects function tools whose parameters block
	// is missing entirely. When a tool ships without an input schema
	// the conversion synthesizes an empty {type:"object"} so the
	// request still validates upstream.
	tools := []ToolDefinition{{Name: "noop"}}
	out := convertToolsToResponses(tools)
	if len(out) != 1 || out[0].OfFunction == nil {
		t.Fatalf("expected fallback function tool, got %+v", out)
	}
	params := out[0].OfFunction.Parameters
	if params == nil {
		t.Fatalf("parameters should not be nil")
	}
	if got, _ := params["type"].(string); got != "object" {
		t.Errorf("synthesized parameters.type should be 'object', got %v", params["type"])
	}
}

// ── Image input ──────────────────────────────────────────────────────────

func TestBuildImageDataURI_DefaultsToPNG(t *testing.T) {
	// Empty MediaType → image/png, since screenshot pastes (the most
	// common attachment in Biene) default to PNG. Caller should still
	// set MediaType when known; this is just the safety net.
	got := buildImageDataURI("", []byte{0x89, 'P', 'N', 'G'})
	const wantPrefix = "data:image/png;base64,"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("expected prefix %q, got %q", wantPrefix, got)
	}
	encoded := strings.TrimPrefix(got, wantPrefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("data URI payload should be valid base64: %v", err)
	}
	if string(decoded) != "\x89PNG" {
		t.Errorf("decoded payload mismatch: %q", decoded)
	}
}

func TestBuildImageDataURI_PreservesExplicitMediaType(t *testing.T) {
	got := buildImageDataURI("image/jpeg", []byte{0xff, 0xd8, 0xff})
	if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
		t.Errorf("explicit media type should win: %q", got)
	}
}

func TestConvertMessagesToResponsesInput_UserTurnWithImage_UsesContentList(t *testing.T) {
	// A user turn carrying both text and an image must take the
	// structured `OfInputMessage` path — the easy-input fast path
	// can't represent images. The text and image both end up in the
	// content list, in source order, with the image as a base64 data
	// URI.
	imgBytes := []byte("FAKE-PNG-BYTES")
	msgs := []Message{
		{
			Role: RoleUser,
			Content: []ContentBlock{
				TextBlock{Text: "look at this"},
				ImageBlock{
					Path:      "inbox/user/screenshot.png",
					MediaType: "image/png",
					Data:      imgBytes,
				},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected single message item, got %d (%+v)", len(items), items)
	}
	if items[0].OfInputMessage == nil {
		t.Fatalf("image-bearing message must use OfInputMessage path: %+v", items[0])
	}
	msg := items[0].OfInputMessage
	if msg.Role != "user" {
		t.Errorf("structured message role should be user, got %q", msg.Role)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected text + image content parts, got %d", len(msg.Content))
	}
	if msg.Content[0].OfInputText == nil || msg.Content[0].OfInputText.Text != "look at this" {
		t.Errorf("first content part should be input_text 'look at this': %+v", msg.Content[0])
	}
	if msg.Content[1].OfInputImage == nil {
		t.Fatalf("second content part should be input_image: %+v", msg.Content[1])
	}
	url := msg.Content[1].OfInputImage.ImageURL.Or("")
	const wantPrefix = "data:image/png;base64,"
	if !strings.HasPrefix(url, wantPrefix) {
		t.Errorf("image URL should be a data URI, got %q", url)
	}
	encoded := strings.TrimPrefix(url, wantPrefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("image data URI payload not valid base64: %v", err)
	}
	if string(decoded) != string(imgBytes) {
		t.Errorf("image bytes should round-trip through base64, got %q", decoded)
	}
}

func TestConvertMessagesToResponsesInput_ImageOnlyMessage_OmitsTextPart(t *testing.T) {
	// User turn with only an image (no caption). Content list must
	// contain just the image — no empty input_text part, which would
	// be wasted tokens at best and rejected at worst.
	msgs := []Message{
		{
			Role: RoleUser,
			Content: []ContentBlock{
				ImageBlock{MediaType: "image/png", Data: []byte("a")},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(items) != 1 || items[0].OfInputMessage == nil {
		t.Fatalf("expected one structured message item, got %+v", items)
	}
	content := items[0].OfInputMessage.Content
	if len(content) != 1 {
		t.Fatalf("image-only message should have one content part, got %d", len(content))
	}
	if content[0].OfInputImage == nil {
		t.Errorf("sole content part should be input_image: %+v", content[0])
	}
}

func TestConvertMessagesToResponsesInput_ImageWithoutData_IsDroppedSilently(t *testing.T) {
	// ImageBlock.Data is populated transiently by the session layer
	// just before the provider call; if it's absent at this point
	// (e.g. replay path forgot to rehydrate), there's nothing to send.
	// We must not emit a degenerate message item — and we must not
	// emit an empty input_image part either, since OpenAI rejects
	// `image_url` without a payload.
	msgs := []Message{
		{
			Role: RoleUser,
			Content: []ContentBlock{
				TextBlock{Text: "did you see it?"},
				ImageBlock{Path: "inbox/user/missing.png", MediaType: "image/png" /* no Data */},
			},
		},
	}
	items, err := convertMessagesToResponsesInput(msgs)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// Falls back to the easy-input fast path: text-only.
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d (%+v)", len(items), items)
	}
	if items[0].OfInputMessage != nil {
		t.Errorf("no image data should mean we don't take the structured path: %+v", items[0])
	}
	if items[0].OfMessage == nil {
		t.Errorf("text-only fallback should emit OfMessage easy-input form: %+v", items[0])
	}
}

// ── Friendly error translation ───────────────────────────────────────────

func TestFriendlyCodexMessage_UsageLimit_WithPlanAndResetsAt(t *testing.T) {
	// resets_at is unix seconds, ~30 minutes in the future. We round
	// to whole minutes inside friendlyCodexMessage so the assertion
	// uses a tolerant range rather than an exact match.
	resetsAt := time.Now().Add(30 * time.Minute).Unix()
	body := []byte(`{"error":{"code":"usage_limit_reached","plan_type":"Pro","resets_at":` +
		strconvI(resetsAt) + `,"message":"upstream raw"}}`)
	got := friendlyCodexMessage(string(body), 429)

	if !strings.Contains(got, "usage limit") {
		t.Errorf("missing usage-limit phrase: %q", got)
	}
	if !strings.Contains(got, "(pro plan)") {
		t.Errorf("plan_type should appear lowercased: %q", got)
	}
	if !strings.Contains(got, "min.") {
		t.Errorf("resets_at should produce a 'try again in ~X min.' tail: %q", got)
	}
}

func TestFriendlyCodexMessage_429StatusFallback_NoBody(t *testing.T) {
	// Some Codex errors come back with an empty body (e.g. an edge
	// proxy rejecting before the origin can serialize JSON). Status
	// alone is enough to tell the user it's a quota issue.
	got := friendlyCodexMessage("", 429)
	if !strings.Contains(got, "usage limit") {
		t.Errorf("status-only fallback should still mention usage limit: %q", got)
	}
}

func TestFriendlyCodexMessage_NonUsage_FallsThrough(t *testing.T) {
	// A generic 400 with a real message should surface the message
	// verbatim — we don't want to mistranslate it as a quota hit.
	body := `{"error":{"code":"invalid_request","message":"input_text required"}}`
	got := friendlyCodexMessage(body, 400)
	if got != "input_text required" {
		t.Errorf("non-usage error should pass through message: %q", got)
	}
}

func TestFriendlyCodexMessage_UnknownEnvelope_ReturnsEmpty(t *testing.T) {
	// Garbage / unrelated JSON returns "" so translateCodexError can
	// fall back to apiErr.Message rather than masking the real error.
	if got := friendlyCodexMessage(`{"foo":"bar"}`, 200); got != "" {
		t.Errorf("unknown envelope should return empty, got %q", got)
	}
	if got := friendlyCodexMessage(`not json`, 0); got != "" {
		t.Errorf("non-JSON should return empty, got %q", got)
	}
}

// strconvI is a tiny helper so the test body stays readable; we don't
// pull in strconv at the file level just for this case.
func strconvI(n int64) string { return fmt.Sprintf("%d", n) }

// ── Reasoning effort/summary translation ─────────────────────────────────

func TestBuildCodexReasoning_PassesThroughEffortAndSummary(t *testing.T) {
	r := buildCodexReasoning("gpt-5", map[string]any{
		"reasoning": map[string]any{
			"effort":  "high",
			"summary": "concise",
		},
	})
	if string(r.Effort) != "high" {
		t.Errorf("effort: got %q want %q", r.Effort, "high")
	}
	if string(r.Summary) != "concise" {
		t.Errorf("summary: got %q want %q", r.Summary, "concise")
	}
}

func TestBuildCodexReasoning_ClampsMinimalForGPT55(t *testing.T) {
	// gpt-5.2 / 5.3 / 5.4 / 5.5 reject "minimal" — pi-coding-agent
	// demotes it to "low" so the request doesn't 400. Spot-check the
	// boundary cases we actually ship today.
	for _, model := range []string{"gpt-5.5", "gpt-5.4"} {
		r := buildCodexReasoning(model, map[string]any{
			"reasoning": map[string]any{"effort": "minimal"},
		})
		if string(r.Effort) != "low" {
			t.Errorf("%s: minimal should clamp to low, got %q", model, r.Effort)
		}
	}
}

func TestBuildCodexReasoning_NoFragment_ReturnsZero(t *testing.T) {
	// Empty extras → zero ReasoningParam → SDK omits the block →
	// model uses its own default. Matches pre-thinking behaviour, so
	// existing sessions don't regress.
	r := buildCodexReasoning("gpt-5.5", nil)
	if string(r.Effort) != "" || string(r.Summary) != "" {
		t.Errorf("nil extras should produce zero ReasoningParam, got %+v", r)
	}
}

func TestClampCodexReasoningEffort_GPT51AndCodexMini(t *testing.T) {
	cases := []struct {
		model, in, want string
	}{
		{"gpt-5.1", "xhigh", "high"},
		{"gpt-5.1", "high", "high"},
		{"gpt-5.1-codex-mini", "low", "medium"},
		{"gpt-5.1-codex-mini", "medium", "medium"},
		{"gpt-5.1-codex-mini", "high", "high"},
		{"gpt-5.1-codex-mini", "xhigh", "high"},
		// Unknown model → pass through. Future models we haven't
		// taught the clamp about should still receive the user's
		// requested effort verbatim.
		{"gpt-99-future", "xhigh", "xhigh"},
	}
	for _, c := range cases {
		got := clampCodexReasoningEffort(c.model, c.in)
		if got != c.want {
			t.Errorf("%s + %q: got %q want %q", c.model, c.in, got, c.want)
		}
	}
}
