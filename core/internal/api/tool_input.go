package api

import (
	"encoding/json"
	"log/slog"
)

// finalizeToolInput hardens streamed tool-arguments bytes against
// truncation and provider quirks. Some providers cut the stream
// mid-arguments (leaving us with bytes like `{"file_path":`) or emit
// non-JSON fragments — passing those through as the tool input causes
// "unexpected end of JSON input" errors at execute time and breaks
// every subsequent message persistence (json.RawMessage refuses to
// marshal invalid bytes). Fall back to "{}" so the tool surfaces a
// clean missing-field error instead.
func finalizeToolInput(args []byte, toolID, toolName string) json.RawMessage {
	if json.Valid(args) {
		return json.RawMessage(args)
	}
	if len(args) > 0 {
		preview := args
		if len(preview) > 200 {
			preview = preview[:200]
		}
		slog.Warn("provider stream: invalid tool input bytes — falling back to empty object",
			"tool_id", toolID,
			"tool_name", toolName,
			"raw_len", len(args),
			"raw_prefix", string(preview),
		)
	}
	return json.RawMessage("{}")
}
