package agentloop

import (
	"context"
	"testing"

	"tinte/internal/api"
)

func TestCollectStreamPutsReasoningAtHead(t *testing.T) {
	stream := make(chan api.StreamEvent, 4)
	events := make(chan Event, 3)

	stream <- api.StreamEvent{Type: api.EventReasoningDelta, Text: "thinking..."}
	stream <- api.StreamEvent{Type: api.EventSignatureDelta, Text: "sig"}
	stream <- api.StreamEvent{Type: api.EventTextDelta, Text: "final answer"}
	stream <- api.StreamEvent{Type: api.EventDone}
	close(stream)

	msg, _, err := collectStream(context.Background(), stream, events, nil, nil)
	if err != nil {
		t.Fatalf("collectStream returned error: %v", err)
	}

	if len(msg.Content) != 2 {
		t.Fatalf("expected two content blocks, got %d", len(msg.Content))
	}
	reasoningBlock, ok := msg.Content[0].(api.ReasoningBlock)
	if !ok {
		t.Fatalf("expected reasoning block first, got %T", msg.Content[0])
	}
	if reasoningBlock.Text != "thinking..." || reasoningBlock.Signature != "sig" {
		t.Fatalf("unexpected reasoning block: %#v", reasoningBlock)
	}
	textBlock, ok := msg.Content[1].(api.TextBlock)
	if !ok {
		t.Fatalf("expected text block second, got %T", msg.Content[1])
	}
	if textBlock.Text != "final answer" {
		t.Fatalf("expected final text, got %q", textBlock.Text)
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

func TestCollectStreamOmitsReasoningWhenEmpty(t *testing.T) {
	stream := make(chan api.StreamEvent, 2)
	events := make(chan Event, 1)

	stream <- api.StreamEvent{Type: api.EventTextDelta, Text: "hi"}
	stream <- api.StreamEvent{Type: api.EventDone}
	close(stream)

	msg, _, err := collectStream(context.Background(), stream, events, nil, nil)
	if err != nil {
		t.Fatalf("collectStream returned error: %v", err)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected single block, got %d", len(msg.Content))
	}
	if _, ok := msg.Content[0].(api.TextBlock); !ok {
		t.Fatalf("expected text-only content, got %T", msg.Content[0])
	}
}
