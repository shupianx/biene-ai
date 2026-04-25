package session

import (
	"testing"

	"biene/internal/agentloop"
)

func TestReasoningPersistsOnAssistantMessage(t *testing.T) {
	sess := &Session{ID: "sess_test", Name: "Test"}

	sess.applyEvent(agentloop.Event{Kind: agentloop.KindReasoningDelta, Text: "thinking"})
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindTextDelta, Text: "answer"})
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindDone})

	history := sess.SnapshotHistory()
	if len(history) != 1 {
		t.Fatalf("expected one assistant message, got %d", len(history))
	}
	msg := history[0]
	if msg.Reasoning == nil {
		t.Fatalf("expected reasoning to be present")
	}
	if msg.Reasoning.Text != "thinking" {
		t.Fatalf("unexpected reasoning text: %q", msg.Reasoning.Text)
	}
	if msg.Reasoning.DurationMS <= 0 {
		t.Fatalf("expected reasoning duration to be finalized, got %d", msg.Reasoning.DurationMS)
	}
	if msg.Text != "answer" {
		t.Fatalf("unexpected assistant text: %q", msg.Text)
	}
	if msg.Streaming {
		t.Fatalf("expected message to be finalized")
	}
}

// TestReasoningAfterToolCallStartsNewSegment guards against a regression where
// a second round of reasoning (after a tool call) was appended to the first
// round's Reasoning block instead of starting a new assistant segment. The
// symptom was: first round's timer froze early, second round's text got
// concatenated onto the first, and no new "thinking…" block rendered.
func TestReasoningAfterToolCallStartsNewSegment(t *testing.T) {
	sess := &Session{ID: "sess_test", Name: "Test"}

	// Round 1: reasoning → tool call → tool result
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindReasoningDelta, Text: "plan "})
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindReasoningDelta, Text: "it out"})
	sess.applyEvent(agentloop.Event{
		Kind:     agentloop.KindToolStart,
		ToolID:   "t1",
		ToolName: "read_file",
	})
	sess.applyEvent(agentloop.Event{
		Kind:     agentloop.KindToolResult,
		ToolID:   "t1",
		ToolName: "read_file",
		Text:     "ok",
	})

	// Round 2: reasoning after the tool returns, then final text.
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindReasoningDelta, Text: "now I know"})
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindTextDelta, Text: "done"})
	sess.applyEvent(agentloop.Event{Kind: agentloop.KindDone})

	history := sess.SnapshotHistory()
	if len(history) != 2 {
		t.Fatalf("expected two assistant segments (pre/post tool), got %d", len(history))
	}

	first := history[0]
	if first.Reasoning == nil || first.Reasoning.Text != "plan it out" {
		t.Fatalf("unexpected first reasoning: %+v", first.Reasoning)
	}
	if first.Reasoning.DurationMS <= 0 {
		t.Fatalf("first reasoning should be finalized, got %d", first.Reasoning.DurationMS)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("first segment should carry the tool call, got %d", len(first.ToolCalls))
	}

	second := history[1]
	if second.Reasoning == nil {
		t.Fatalf("expected second reasoning block to exist (regression: was appended to first)")
	}
	if second.Reasoning.Text != "now I know" {
		t.Fatalf("second reasoning text leaked from first round: %q", second.Reasoning.Text)
	}
	if second.Reasoning.StartedAt.Equal(first.Reasoning.StartedAt) {
		t.Fatalf("second reasoning should have its own StartedAt, got identical timestamp")
	}
	if second.Text != "done" {
		t.Fatalf("unexpected second text: %q", second.Text)
	}
}
