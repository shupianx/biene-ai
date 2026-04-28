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

type reasoningDeltaPayload struct {
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
	ToolID      string          `json:"tool_id,omitempty"`
	Context     json.RawMessage `json:"context,omitempty"`
	Expired     bool            `json:"expired,omitempty"`
}

type toolComposeProgressPayload struct {
	ToolID        string `json:"tool_id"`
	ToolName      string `json:"tool_name,omitempty"`
	FilePath      string `json:"file_path,omitempty"`
	FileTextBytes int    `json:"file_text_bytes,omitempty"`
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

type skillActivatedPayload struct {
	SkillName string `json:"skill_name"`
}

// compactionStartPayload is broadcast right before the summarizer LLM
// call, so the UI can show a "compressing context…" indicator.
type compactionStartPayload struct {
	TokensBefore int `json:"tokens_before"`
}

// compactionDonePayload is broadcast after a successful compaction.
// MessageID points at the CompactionMarker DisplayMessage that was
// appended; the UI reads its `compaction` block for the summary text.
type compactionDonePayload struct {
	MessageID    string `json:"message_id"`
	TokensBefore int    `json:"tokens_before"`
	TokensAfter  int    `json:"tokens_after"`
	Replaced     int    `json:"replaced"`
}

// compactionFailedPayload signals that compaction was attempted but
// errored out. The agent loop continues with the un-compressed history;
// the UI surfaces the message as a non-fatal toast.
type compactionFailedPayload struct {
	Reason string `json:"reason"`
}
