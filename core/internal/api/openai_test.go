package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProviderAddsTopLevelThinkingField(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var err error
		requestBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenAIProvider("", "qwen3.6-plus", server.URL)
	enabled := true
	stream, err := provider.Stream(
		t.Context(),
		"",
		[]Message{{
			Role:    RoleUser,
			Content: []ContentBlock{TextBlock{Text: "hello"}},
		}},
		nil,
		1024,
		RequestOptions{EnableThinking: &enabled},
	)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	for range stream {
	}

	var payload map[string]any
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["enable_thinking"]; got != true {
		t.Fatalf("expected top-level enable_thinking=true, got %#v", got)
	}
	if got := payload["model"]; got != "qwen3.6-plus" {
		t.Fatalf("expected model to be preserved, got %#v", got)
	}
}
