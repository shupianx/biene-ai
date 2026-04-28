package compaction

import (
	"strings"
	"testing"

	"biene/internal/api"
)

func userText(text string) api.Message {
	return api.Message{
		Role:    api.RoleUser,
		Content: []api.ContentBlock{api.TextBlock{Text: text}},
	}
}

func userToolResult(id, content string) api.Message {
	return api.Message{
		Role: api.RoleUser,
		Content: []api.ContentBlock{
			api.ToolResultBlock{ToolUseID: id, Content: content},
		},
	}
}

func assistantText(text string) api.Message {
	return api.Message{
		Role:    api.RoleAssistant,
		Content: []api.ContentBlock{api.TextBlock{Text: text}},
	}
}

func assistantToolUse(id, name string, input string) api.Message {
	return api.Message{
		Role: api.RoleAssistant,
		Content: []api.ContentBlock{
			api.TextBlock{Text: ""},
			api.ToolUseBlock{ID: id, Name: name, Input: []byte(input)},
		},
	}
}

func TestFindCutPoint_EntireFitsReturnsZero(t *testing.T) {
	msgs := []api.Message{
		userText("hi"),
		assistantText("hello"),
	}
	if got := FindCutPoint(msgs, 100000); got != 0 {
		t.Fatalf("expected 0 (no compaction needed), got %d", got)
	}
}

func TestFindCutPoint_PrefersFreshUserBoundary(t *testing.T) {
	// keepRecent budget is small; cut should land on a clean user-text turn,
	// never inside a tool_use/tool_result pair.
	bigText := strings.Repeat("x", 4000) // ~1000 tokens
	msgs := []api.Message{
		userText("first task"),
		assistantToolUse("tu1", "read_file", `{"path":"a.go"}`),
		userToolResult("tu1", bigText),
		assistantText("done"),
		userText("next task"),
		assistantToolUse("tu2", "read_file", `{"path":"b.go"}`),
		userToolResult("tu2", bigText),
		assistantText("ok"),
	}
	cut := FindCutPoint(msgs, 100)
	if cut == 0 {
		t.Fatalf("expected cut, got 0")
	}
	if msgs[cut].Role != api.RoleUser {
		t.Fatalf("cut must land on user message, got %s at %d", msgs[cut].Role, cut)
	}
	for _, b := range msgs[cut].Content {
		if _, ok := b.(api.ToolResultBlock); ok {
			t.Fatalf("cut landed on a user message containing tool_result at index %d", cut)
		}
	}
}

func TestFindCutPoint_NoSafeBoundary(t *testing.T) {
	// All user messages are tool_result wrappers; no clean cut exists.
	msgs := []api.Message{
		assistantToolUse("tu1", "x", `{}`),
		userToolResult("tu1", "r1"),
		assistantToolUse("tu2", "x", `{}`),
		userToolResult("tu2", "r2"),
	}
	if got := FindCutPoint(msgs, 1); got != 0 {
		t.Fatalf("expected 0 (no safe boundary), got %d", got)
	}
}

func TestSyntheticHeadDetection(t *testing.T) {
	wrapped := SummaryOpenTag + "\nbody\n" + SummaryCloseTag
	msgs := []api.Message{
		{Role: api.RoleUser, Content: []api.ContentBlock{api.TextBlock{Text: wrapped}}},
		{Role: api.RoleAssistant, Content: []api.ContentBlock{api.TextBlock{Text: AckText}}},
		userText("real"),
	}
	if got := SyntheticHeadLength(msgs); got != 2 {
		t.Fatalf("expected synthetic head of 2, got %d", got)
	}
	if got := ExtractPreviousSummary(msgs); got != "body" {
		t.Fatalf("expected previous summary 'body', got %q", got)
	}
}

func TestSyntheticHeadDetection_RegularMessages(t *testing.T) {
	msgs := []api.Message{
		userText("hello"),
		assistantText("hi"),
	}
	if got := SyntheticHeadLength(msgs); got != 0 {
		t.Fatalf("expected 0 for regular conversation, got %d", got)
	}
}

func TestShouldCompact(t *testing.T) {
	settings := Settings{
		Enabled:          true,
		ReserveTokens:    16384,
		KeepRecentTokens: 32000,
		ContextWindow:    200000,
	}
	if ShouldCompact(50000, settings) {
		t.Fatalf("50K of 200K should not trigger")
	}
	if !ShouldCompact(190000, settings) {
		t.Fatalf("190K of 200K (reserve 16K) should trigger")
	}
	settings.Enabled = false
	if ShouldCompact(190000, settings) {
		t.Fatalf("disabled compaction must never trigger")
	}
}
