// Package compaction implements context-window compaction for long
// agent sessions. Trigger logic and cut-point selection match the design
// notes (see project discussion): shouldCompact is a pure predicate over
// (input_tokens, context_window, reserve), the cut keeps the last
// `keep_recent_tokens` budget verbatim, and the discarded prefix is
// replaced with a single synthetic [user, assistant] pair carrying an
// LLM-generated structured summary.
//
// The package is provider-agnostic: callers pass an api.Provider for
// summary generation. Local full history (display_messages) is the
// caller's responsibility — compaction only mutates the API-facing list.
package compaction

import (
	"biene/internal/api"
)

// Settings is the per-call tuning. Mirrors config.CompactionConfig but
// kept local so the package has no upward dependency.
type Settings struct {
	Enabled          bool
	ReserveTokens    int
	KeepRecentTokens int
	ContextWindow    int
}

// ShouldCompact returns true when the API-reported input token count has
// grown so close to the model's context ceiling that the next call risks
// exhausting reserve headroom for the response.
//
//	contextTokens > contextWindow - reserveTokens
//
// Returns false when compaction is disabled, when the window is unknown
// (<= 0), or when reserve is misconfigured.
func ShouldCompact(contextTokens int, settings Settings) bool {
	if !settings.Enabled {
		return false
	}
	if settings.ContextWindow <= 0 || settings.ReserveTokens <= 0 {
		return false
	}
	if contextTokens <= 0 {
		return false
	}
	return contextTokens > settings.ContextWindow-settings.ReserveTokens
}

// Result describes the outcome of a successful compaction. Messages is
// the new api_messages list to install on the session. Summary is the
// LLM-generated markdown for display alongside the marker. TokensBefore
// records the API-reported usage that triggered this run; TokensAfter is
// a local estimate of the new tail size (for telemetry/UI).
type Result struct {
	Messages     []api.Message
	Summary      string
	TokensBefore int
	TokensAfter  int
	Replaced     int // number of messages from the original list that were folded into the summary
}
