package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProviderSplatsThinkingExtra(t *testing.T) {
	tests := []struct {
		name   string
		extra  map[string]any
		verify func(t *testing.T, payload map[string]any)
	}{
		{
			name:  "qwen top-level bool",
			extra: map[string]any{"enable_thinking": true},
			verify: func(t *testing.T, payload map[string]any) {
				if got := payload["enable_thinking"]; got != true {
					t.Fatalf("expected top-level enable_thinking=true, got %#v", got)
				}
			},
		},
		{
			name: "kimi nested object",
			extra: map[string]any{
				"thinking": map[string]any{"type": "enabled"},
			},
			verify: func(t *testing.T, payload map[string]any) {
				thinking, ok := payload["thinking"].(map[string]any)
				if !ok {
					t.Fatalf("expected thinking object, got %#v", payload["thinking"])
				}
				if thinking["type"] != "enabled" {
					t.Fatalf("expected thinking.type=enabled, got %#v", thinking["type"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			provider := NewOpenAIProvider("", "test-model", server.URL)
			stream, err := provider.Stream(
				t.Context(),
				"",
				[]Message{{
					Role:    RoleUser,
					Content: []ContentBlock{TextBlock{Text: "hello"}},
				}},
				nil,
				RequestOptions{ThinkingExtra: tc.extra},
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
			tc.verify(t, payload)
			if got := payload["model"]; got != "test-model" {
				t.Fatalf("expected model to be preserved, got %#v", got)
			}
		})
	}
}
