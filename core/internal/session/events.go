package session

import (
	"encoding/json"
)

// Frame is a pre-serialized realtime event payload.
type Frame struct {
	EventType string
	Data      []byte
}

// makeFrame serialises the payload and returns an event frame.
func makeFrame(eventType string, payload any) Frame {
	data, _ := json.Marshal(payload)
	return Frame{EventType: eventType, Data: data}
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

type PermissionRequestPayload struct {
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
