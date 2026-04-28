package compaction

import (
	"strings"

	"biene/internal/api"
)

// SummaryOpenTag and SummaryCloseTag wrap the summary text inside the
// synthetic user message that replaces the discarded prefix. The tag is
// what next-round detection keys off of, so it must remain stable across
// versions; treat it as a wire format.
const (
	SummaryOpenTag  = "<biene:compaction-summary>"
	SummaryCloseTag = "</biene:compaction-summary>"
	// AckText is the synthetic assistant turn that follows the summary.
	// It exists only to keep the user/assistant alternation valid for
	// providers that reject consecutive same-role messages.
	AckText = "Acknowledged. Continuing from the summarized context."
)

// SyntheticHeadLength returns the number of leading messages that form a
// previous compaction's synthetic pair (summary user + ack assistant).
// Returns 0 when the head is genuine conversation.
func SyntheticHeadLength(msgs []api.Message) int {
	if len(msgs) < 2 {
		return 0
	}
	if msgs[0].Role != api.RoleUser {
		return 0
	}
	if !messageStartsWith(msgs[0], SummaryOpenTag) {
		return 0
	}
	if msgs[1].Role != api.RoleAssistant {
		return 0
	}
	return 2
}

// ExtractPreviousSummary returns the previous compaction's summary text
// when present, else "". Strips the wrapping tags.
func ExtractPreviousSummary(msgs []api.Message) string {
	if SyntheticHeadLength(msgs) == 0 {
		return ""
	}
	for _, b := range msgs[0].Content {
		if t, ok := b.(api.TextBlock); ok {
			text := t.Text
			start := strings.Index(text, SummaryOpenTag)
			end := strings.Index(text, SummaryCloseTag)
			if start < 0 || end < 0 || end <= start {
				continue
			}
			return strings.TrimSpace(text[start+len(SummaryOpenTag) : end])
		}
	}
	return ""
}

// FindCutPoint walks msgs from the end, accumulating an estimated token
// count, then returns the index at which the kept tail begins. Everything
// before that index is the discard prefix; everything from it onward is
// preserved verbatim.
//
// Rules:
//   - Once accumulated tokens reach keepRecentTokens, the walker keeps
//     going backward until it lands on a "safe boundary" — a user message
//     whose content has no tool_result blocks. This is the only kind of
//     boundary that survives slicing without leaving an orphan tool_use
//     reference in the kept tail (which the API would reject).
//   - Returns 0 when the entire history fits within keepRecentTokens
//     (nothing to compact) or when no safe boundary exists in the prefix
//     (caller should skip this round rather than corrupt the message
//     sequence).
//   - The caller is expected to have stripped any synthetic compaction
//     head before calling — see SyntheticHeadLength.
func FindCutPoint(msgs []api.Message, keepRecentTokens int) int {
	if keepRecentTokens <= 0 || len(msgs) == 0 {
		return 0
	}
	accumulated := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		accumulated += EstimateMessageTokens(msgs[i])
		if accumulated >= keepRecentTokens {
			for j := i; j >= 0; j-- {
				if isSafeCutBoundary(msgs[j]) {
					return j
				}
			}
			return 0
		}
	}
	return 0
}

// isSafeCutBoundary returns true when msg can serve as the first message
// of a kept tail. It must be a user turn whose content carries only
// "self-contained" blocks — text or images — never tool_result blocks
// (those depend on a tool_use block in the previous assistant turn that
// would be discarded).
func isSafeCutBoundary(m api.Message) bool {
	if m.Role != api.RoleUser {
		return false
	}
	if len(m.Content) == 0 {
		return false
	}
	for _, b := range m.Content {
		switch b.(type) {
		case api.ToolResultBlock:
			return false
		}
	}
	return true
}

// EstimateMessageTokens is a rough char-based proxy used by FindCutPoint
// and post-compaction telemetry. Real triggering uses API-reported
// usage; this estimator is only ever asked "is this batch of messages
// roughly under N tokens?", so a 4-chars-per-token heuristic is adequate
// and stable across providers.
func EstimateMessageTokens(m api.Message) int {
	chars := 0
	for _, b := range m.Content {
		switch v := b.(type) {
		case api.TextBlock:
			chars += len(v.Text)
		case api.ReasoningBlock:
			chars += len(v.Text) + len(v.Signature)
		case api.ToolUseBlock:
			chars += len(v.Name) + len(v.Input)
		case api.ToolResultBlock:
			chars += len(v.Content) + len(v.ToolUseID)
		case api.ImageBlock:
			// Conservative: a typical image block costs ~1.2K tokens with
			// Claude/Anthropic's image tokenizer; the path metadata is
			// essentially free. Use a fixed estimate so a workspace full
			// of attached images doesn't get treated as cheap text.
			chars += 5000
		}
	}
	return chars / 4
}

// EstimateMessagesTokens sums per-message estimates over a slice.
func EstimateMessagesTokens(msgs []api.Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateMessageTokens(m)
	}
	return total
}

func messageStartsWith(m api.Message, prefix string) bool {
	for _, b := range m.Content {
		if t, ok := b.(api.TextBlock); ok {
			if strings.HasPrefix(strings.TrimLeft(t.Text, " \t\n\r"), prefix) {
				return true
			}
			return false
		}
	}
	return false
}
