package agentloop

import (
	"context"
	"testing"

	"biene/internal/api"
)

func TestCollectStreamKeepsReasoningOutOfFinalMessage(t *testing.T) {
	stream := make(chan api.StreamEvent, 3)
	events := make(chan Event, 3)

	stream <- api.StreamEvent{Type: api.EventReasoningDelta, Text: "thinking..."}
	stream <- api.StreamEvent{Type: api.EventTextDelta, Text: "final answer"}
	stream <- api.StreamEvent{Type: api.EventDone}
	close(stream)

	msg, _, err := collectStream(context.Background(), stream, events, nil)
	if err != nil {
		t.Fatalf("collectStream returned error: %v", err)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("expected exactly one content block, got %d", len(msg.Content))
	}

	textBlock, ok := msg.Content[0].(api.TextBlock)
	if !ok {
		t.Fatalf("expected text block, got %T", msg.Content[0])
	}
	if textBlock.Text != "final answer" {
		t.Fatalf("expected final text only, got %q", textBlock.Text)
	}

	first := <-events
	if first.Kind != KindReasoningDelta || first.Text != "thinking..." {
		t.Fatalf("unexpected first event: %#v", first)
	}
	second := <-events
	if second.Kind != KindTextDelta || second.Text != "final answer" {
		t.Fatalf("unexpected second event: %#v", second)
	}
}
