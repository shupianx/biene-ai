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
