package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConvertMessagesAlwaysEmitsAssistantReasoningContent(t *testing.T) {
	// DeepSeek's thinking-mode validator rejects requests where any prior
	// assistant turn lacks the reasoning_content field, even if that turn
	// was generated with thinking off. Verify every assistant message
	// carries the key (empty string when unknown), and user/tool messages
	// do not.
	msgs := []Message{
		{
			Role:    RoleUser,
			Content: []ContentBlock{TextBlock{Text: "find me the readme"}},
		},
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				ToolUseBlock{ID: "call_1", Name: "list_files", Input: json.RawMessage(`{}`)},
			},
		},
		{
			Role: RoleUser,
			Content: []ContentBlock{
				ToolResultBlock{ToolUseID: "call_1", Content: "README.md"},
			},
		},
		{
			Role: RoleAssistant,
			Content: []ContentBlock{
				ReasoningBlock{Text: "the file exists"},
				TextBlock{Text: "found it"},
			},
		},
		{
			Role:    RoleUser,
			Content: []ContentBlock{TextBlock{Text: "now read it"}},
		},
	}

	out, err := convertMessagesToOpenAI(msgs, "system")
	if err != nil {
		t.Fatalf("convertMessagesToOpenAI: %v", err)
	}

	// Marshal then unmarshal into a generic shape so we can check field
	// presence (nil pointer with omitempty disappears; non-nil stays).
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	type want struct {
		role         string
		hasReasoning bool   // field key present in JSON
		reasoning    string // expected value when hasReasoning
	}
	wants := []want{
		{role: "system"},                                              // 0
		{role: "user"},                                                // 1
		{role: "assistant", hasReasoning: true, reasoning: ""},        // 2 — tool-call turn, no reasoning generated
		{role: "tool"},                                                // 3
		{role: "assistant", hasReasoning: true, reasoning: "the file exists"}, // 4
		{role: "user"},                                                // 5
	}
	if len(wire) != len(wants) {
		t.Fatalf("expected %d messages on wire, got %d:\n%s", len(wants), len(wire), string(raw))
	}
	for i, w := range wants {
		var role string
		if err := json.Unmarshal(wire[i]["role"], &role); err != nil {
			t.Fatalf("msg %d role unmarshal: %v", i, err)
		}
		if role != w.role {
			t.Fatalf("msg %d role: want %q got %q", i, w.role, role)
		}
		rcRaw, present := wire[i]["reasoning_content"]
		if w.hasReasoning {
			if !present {
				t.Fatalf("msg %d (%s): reasoning_content must be present on assistant", i, role)
			}
			var rc string
			if err := json.Unmarshal(rcRaw, &rc); err != nil {
				t.Fatalf("msg %d reasoning_content unmarshal: %v", i, err)
			}
			if rc != w.reasoning {
				t.Fatalf("msg %d reasoning_content: want %q got %q", i, w.reasoning, rc)
			}
		} else if present {
			t.Fatalf("msg %d (%s): reasoning_content must NOT appear on non-assistant, got %s", i, role, string(rcRaw))
		}
	}
}

func TestComputeReasoningRetention(t *testing.T) {
	user := func(text string) Message {
		return Message{Role: RoleUser, Content: []ContentBlock{TextBlock{Text: text}}}
	}
	toolResult := func(id, content string) Message {
		return Message{Role: RoleUser, Content: []ContentBlock{ToolResultBlock{ToolUseID: id, Content: content}}}
	}
	assistantText := func(reasoning, text string) Message {
		blocks := []ContentBlock{}
		if reasoning != "" {
			blocks = append(blocks, ReasoningBlock{Text: reasoning})
		}
		if text != "" {
			blocks = append(blocks, TextBlock{Text: text})
		}
		return Message{Role: RoleAssistant, Content: blocks}
	}
	assistantTool := func(reasoning, callID, name string) Message {
		blocks := []ContentBlock{}
		if reasoning != "" {
			blocks = append(blocks, ReasoningBlock{Text: reasoning})
		}
		blocks = append(blocks, ToolUseBlock{ID: callID, Name: name, Input: json.RawMessage(`{}`)})
		return Message{Role: RoleAssistant, Content: blocks}
	}

	tests := []struct {
		name string
		msgs []Message
		// keep[i]==true means reasoning at message index i must be retained.
		// Non-assistant indices are always expected false.
		want []bool
	}{
		{
			name: "empty",
			msgs: nil,
			want: nil,
		},
		{
			name: "single no-tool turn (open interval, no tool)",
			msgs: []Message{
				user("hi"),
				assistantText("internal musing", "hello"),
			},
			want: []bool{false, false},
		},
		{
			name: "single tool turn (open interval, has tool)",
			msgs: []Message{
				user("read file"),
				assistantTool("plan to call list_files", "c1", "list_files"),
				toolResult("c1", "README.md"),
			},
			want: []bool{false, true, false},
		},
		{
			name: "multi-turn no tools — both intervals dropped",
			msgs: []Message{
				user("q1"),
				assistantText("r1", "a1"),
				user("q2"),
				assistantText("r2", "a2"),
			},
			want: []bool{false, false, false, false},
		},
		{
			name: "first interval has tools, second is plain — first kept whole, second dropped",
			msgs: []Message{
				user("q1"),
				assistantTool("r1a", "c1", "list_files"),
				toolResult("c1", "ok"),
				assistantText("r1b", "a1"),
				user("q2"),
				assistantText("r2", "a2"),
			},
			want: []bool{false, true, false, true, false, false},
		},
		{
			name: "tool-result-only user message does not split interval",
			msgs: []Message{
				user("read"),
				assistantTool("r1", "c1", "list_files"),
				toolResult("c1", "ok"),
				assistantTool("r2", "c2", "read_file"),
				toolResult("c2", "contents"),
				assistantText("r3", "done"),
			},
			want: []bool{false, true, false, true, false, true},
		},
		{
			name: "image input closes interval (treated as user input)",
			msgs: []Message{
				user("q1"),
				assistantText("r1", "a1"),
				{Role: RoleUser, Content: []ContentBlock{ImageBlock{Path: "x.png", MediaType: "image/png", Data: []byte{1}}}},
				assistantTool("r2", "c1", "describe"),
				toolResult("c1", "a cat"),
			},
			want: []bool{false, false, false, true, false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeReasoningRetention(tc.msgs)
			if len(got) != len(tc.want) {
				t.Fatalf("len(keep): want %d got %d (%v)", len(tc.want), len(got), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("keep[%d]: want %v got %v\nfull: %v", i, tc.want[i], got[i], got)
				}
			}
		})
	}
}

func TestConvertMessagesDropsReasoningInNoToolInterval(t *testing.T) {
	// End-to-end: the no-tool interval has reasoning, but it should not
	// reach the wire because DeepSeek would ignore it anyway. The field
	// stays present (empty string) for thinking-mode safety.
	msgs := []Message{
		{Role: RoleUser, Content: []ContentBlock{TextBlock{Text: "hi"}}},
		{Role: RoleAssistant, Content: []ContentBlock{
			ReasoningBlock{Text: "user said hi, respond casually"},
			TextBlock{Text: "hello!"},
		}},
		{Role: RoleUser, Content: []ContentBlock{TextBlock{Text: "bye"}}},
	}
	out, err := convertMessagesToOpenAI(msgs, "system")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// out[0]=system, out[1]=user, out[2]=assistant, out[3]=user
	if got := out[2].ReasoningContent; got == nil || *got != "" {
		t.Fatalf("expected empty reasoning_content on assistant, got %v", got)
	}
}

func TestConvertMessagesKeepsReasoningInToolInterval(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: []ContentBlock{TextBlock{Text: "list"}}},
		{Role: RoleAssistant, Content: []ContentBlock{
			ReasoningBlock{Text: "need to call list_files"},
			ToolUseBlock{ID: "c1", Name: "list_files", Input: json.RawMessage(`{}`)},
		}},
		{Role: RoleUser, Content: []ContentBlock{ToolResultBlock{ToolUseID: "c1", Content: "ok"}}},
	}
	out, err := convertMessagesToOpenAI(msgs, "system")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// out[0]=system, out[1]=user, out[2]=assistant(tool_call), out[3]=tool
	if got := out[2].ReasoningContent; got == nil || *got != "need to call list_files" {
		t.Fatalf("expected reasoning to be retained in tool interval, got %v", got)
	}
}

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
