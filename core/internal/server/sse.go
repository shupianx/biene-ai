package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseFrame is an internal unit ready to be flushed to an SSE response.
type sseFrame struct {
	eventType string
	data      []byte // pre-serialized JSON
}

// writeSSE writes one SSE frame to w and flushes if possible.
func writeSSE(w http.ResponseWriter, frame sseFrame) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", frame.eventType, frame.data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// makeFrame serialises the payload and returns a sseFrame.
func makeFrame(eventType string, payload any) sseFrame {
	data, _ := json.Marshal(payload)
	return sseFrame{eventType: eventType, data: data}
}

// ─── Payload types ────────────────────────────────────────────────────────

type textDeltaPayload struct {
	Text string `json:"text"`
}

type toolStartPayload struct {
	ToolID      string          `json:"tool_id,omitempty"`
	ToolName    string          `json:"tool_name"`
	ToolSummary string          `json:"tool_summary"`
	ToolInput   json.RawMessage `json:"tool_input"`
}

type toolResultPayload struct {
	ToolID   string `json:"tool_id,omitempty"`
	ToolName string `json:"tool_name"`
	Text     string `json:"text"`
	IsError  bool   `json:"is_error"`
}

type toolDeniedPayload struct {
	ToolID   string `json:"tool_id,omitempty"`
	ToolName string `json:"tool_name"`
}

type permissionRequestPayload struct {
	RequestID   string          `json:"request_id"`
	Permission  string          `json:"permission"`
	ToolName    string          `json:"tool_name"`
	ToolSummary string          `json:"tool_summary"`
	ToolInput   json.RawMessage `json:"tool_input"`
}

type permissionClearedPayload struct {
	RequestID string `json:"request_id,omitempty"`
}

type messageAddedPayload struct {
	Message DisplayMessage `json:"message"`
}

type statusPayload struct {
	Status SessionStatus `json:"status"`
}

type errorPayload struct {
	Message string `json:"message"`
}

type donePayload struct{}
