// chatgpt_usage.go — *passive* rate-limit / quota tracking.
//
// Earlier iterations of this file proxied chatgpt.com/api/codex/usage.
// That endpoint is fronted by Cloudflare's bot manager and rejects
// any non-browser-shaped client with a 403 + JS challenge page; even
// after replicating Codex CLI's persistent cookie jar + UA the
// challenge could not be cleared from a Go HTTP client. We removed
// the active path entirely.
//
// The Codex backend ALSO emits the same data as response headers on
// every /responses turn (see codex-rs/codex-api/src/rate_limits.rs in
// openai/codex). The header-shaped path doesn't traverse Cloudflare's
// public tier, so it works without any cookie/UA dance. We capture
// the headers via openai-go's request middleware in
// api.ChatGPTCodexProvider and store the parsed snapshot here.
//
// Trade-off vs the active endpoint: the user must send at least one
// message before any usage data appears in Settings. In practice
// that's fine — the panel was always going to be near-empty for a
// freshly-authenticated user, and once they've sent a turn we have
// fresh data on every subsequent turn for free.
package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitWindow is one rolling-window bucket the Codex backend
// tracks. The headers emit `window_minutes` (int) and `reset_at`
// (unix seconds). We do NOT emit `reset_after_seconds` — the
// renderer can compute it from `reset_at - now()` at display time,
// which stays accurate even when the snapshot is several minutes
// old.
type RateLimitWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int64   `json:"window_minutes,omitempty"`
	ResetAt       int64   `json:"reset_at,omitempty"`
}

// RateLimitSnapshot is the cached view of the most recent Codex
// response's rate-limit headers. UpdatedAt lets the UI hint "data
// is X minutes old" if the user hasn't sent anything recently.
//
// LimitName carries the bucket id (e.g. "codex" for the default
// chat budget, "codex_other" for tools / longer features). Today
// we only surface the default bucket — additional buckets land
// untouched in the manager but aren't exposed yet.
type RateLimitSnapshot struct {
	UpdatedAt int64            `json:"updated_at"`
	LimitName string           `json:"limit_name,omitempty"`
	Primary   *RateLimitWindow `json:"primary,omitempty"`
	Secondary *RateLimitWindow `json:"secondary,omitempty"`
}

// ParseRateLimitHeaders inspects an HTTP response's headers for the
// `x-codex-*` family and assembles a RateLimitSnapshot. Returns nil
// when no relevant headers are present so callers can leave the
// previous snapshot untouched (some endpoints / fast paths skip the
// rate-limit headers entirely).
//
// The header layout is documented in openai/codex's
// codex-rs/codex-api/src/rate_limits.rs. Format:
//
//	x-codex-primary-used-percent:   "42"   (or "42.5" for some plans)
//	x-codex-primary-window-minutes: "60"
//	x-codex-primary-reset-at:       "1735689720"
//	x-codex-secondary-used-percent: "5"
//	x-codex-secondary-window-minutes: "1440"
//	x-codex-secondary-reset-at:     "1735693200"
//	x-codex-limit-name:             "codex"
//
// We default to the `codex` prefix; additional limit families
// (codex_other, etc.) use their own prefix and aren't parsed here.
func ParseRateLimitHeaders(h http.Header) *RateLimitSnapshot {
	primary := parseRateLimitWindow(h, "x-codex-primary")
	secondary := parseRateLimitWindow(h, "x-codex-secondary")
	if primary == nil && secondary == nil {
		// No rate-limit headers in this response — usually means the
		// upstream stripped them on a fast-path route or this was a
		// non-Codex request that got routed here by mistake.
		return nil
	}
	return &RateLimitSnapshot{
		UpdatedAt: time.Now().Unix(),
		LimitName: strings.TrimSpace(h.Get("x-codex-limit-name")),
		Primary:   primary,
		Secondary: secondary,
	}
}

func parseRateLimitWindow(h http.Header, prefix string) *RateLimitWindow {
	pct, pctOK := parseFloatHeader(h, prefix+"-used-percent")
	winMin, winOK := parseIntHeader(h, prefix+"-window-minutes")
	resetAt, resetOK := parseIntHeader(h, prefix+"-reset-at")
	if !pctOK && !winOK && !resetOK {
		return nil
	}
	w := &RateLimitWindow{}
	if pctOK {
		w.UsedPercent = pct
	}
	if winOK {
		w.WindowMinutes = winMin
	}
	if resetOK {
		w.ResetAt = resetAt
	}
	return w
}

func parseFloatHeader(h http.Header, name string) (float64, bool) {
	raw := strings.TrimSpace(h.Get(name))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseIntHeader(h http.Header, name string) (int64, bool) {
	raw := strings.TrimSpace(h.Get(name))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		// Some plans emit float-shaped values for nominally-int fields.
		// Fall back so an unexpected ".0" suffix doesn't drop the data.
		f, ferr := strconv.ParseFloat(raw, 64)
		if ferr != nil {
			return 0, false
		}
		return int64(f), true
	}
	return v, true
}
