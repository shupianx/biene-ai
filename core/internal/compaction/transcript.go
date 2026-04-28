package compaction

import (
	"fmt"
	"strings"

	"biene/internal/api"
)

// SerializeTranscript turns a slice of api.Message into a human-readable
// markdown transcript suitable for feeding into the summarizer prompt.
//
// Tool calls and tool results are rendered in their own sections so the
// summarizer sees what actions were taken without having to parse JSON
// envelopes. Image blocks are collapsed to a placeholder line — image
// bytes don't survive serialization to text and would explode the prompt
// if base64-encoded back in.
func SerializeTranscript(msgs []api.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case api.RoleUser:
			writeUserTurn(&sb, m)
		case api.RoleAssistant:
			writeAssistantTurn(&sb, m)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func writeUserTurn(sb *strings.Builder, m api.Message) {
	hasToolResult := false
	for _, b := range m.Content {
		if _, ok := b.(api.ToolResultBlock); ok {
			hasToolResult = true
			break
		}
	}
	if hasToolResult {
		// Pure tool-result turn — render each result as its own section
		// so the summarizer can correlate it with the prior tool_use.
		for _, b := range m.Content {
			if r, ok := b.(api.ToolResultBlock); ok {
				header := "### Tool Result"
				if r.IsError {
					header = "### Tool Result (error)"
				}
				fmt.Fprintf(sb, "%s\n%s\n\n", header, truncate(r.Content, 4000))
			}
		}
		return
	}
	sb.WriteString("## User\n")
	for _, b := range m.Content {
		switch v := b.(type) {
		case api.TextBlock:
			sb.WriteString(v.Text)
			sb.WriteString("\n")
		case api.ImageBlock:
			fmt.Fprintf(sb, "[image: %s]\n", v.Path)
		}
	}
	sb.WriteString("\n")
}

func writeAssistantTurn(sb *strings.Builder, m api.Message) {
	sb.WriteString("## Assistant\n")
	for _, b := range m.Content {
		switch v := b.(type) {
		case api.TextBlock:
			if v.Text != "" {
				sb.WriteString(v.Text)
				sb.WriteString("\n")
			}
		case api.ReasoningBlock:
			// Reasoning text is internal-only; the summarizer doesn't
			// need it to capture the user-visible decisions and we
			// don't want it leaking into the saved summary.
			continue
		case api.ToolUseBlock:
			fmt.Fprintf(sb, "\n### Tool Use: %s\nInput: %s\n", v.Name, string(v.Input))
		}
	}
	sb.WriteString("\n")
}

// truncate caps very long tool outputs so a single mega-result doesn't
// blow the summarizer's context. The summarizer is told (via prompt
// rules) not to invent details, so a "truncated" marker is enough
// signal that the original was longer.
func truncate(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s
	}
	return s[:maxRunes] + "\n... (truncated)"
}
