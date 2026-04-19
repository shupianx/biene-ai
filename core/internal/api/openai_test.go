package api

import (
	"encoding/json"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestMarshalChatCompletionRequestAddsTopLevelExtraFields(t *testing.T) {
	body, err := marshalChatCompletionRequest(openai.ChatCompletionRequest{
		Model: "qwen3.6-plus",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
		},
		Stream: true,
	}, map[string]any{
		"enable_thinking": true,
	})
	if err != nil {
		t.Fatalf("marshalChatCompletionRequest returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["enable_thinking"]; got != true {
		t.Fatalf("expected top-level enable_thinking=true, got %#v", got)
	}
	if got := payload["model"]; got != "qwen3.6-plus" {
		t.Fatalf("expected model to be preserved, got %#v", got)
	}
}
