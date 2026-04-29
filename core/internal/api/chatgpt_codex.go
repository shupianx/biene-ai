// chatgpt_codex.go drives the Codex backend at chatgpt.com — the
// endpoint Codex CLI itself uses for ChatGPT Plus/Pro accounts. It is
// the only path that works without a platform.openai.com organization;
// the standard api.openai.com /v1/chat/completions route requires an
// `sk-…` key, which in turn requires the user has an OpenAI org.
//
// The wire format is the OpenAI Responses API (a structured input/output
// item model rather than the flat messages list of Chat Completions).
// We piggy-back on openai-go v3's typed parameter and event unions so
// we don't have to define the ~30 event variants ourselves; the SDK
// handles SSE framing and JSON decoding via ResponseStreamEventUnion.
//
// Reference implementation: pi-coding-agent's openai-codex-responses.ts
// at https://github.com/badlogic/pi-mono/blob/main/packages/ai/src/
// providers/openai-codex-responses.ts. The streaming protocol behavior
// in this file (item lifecycle, partial-JSON arguments, signature
// echo) follows that implementation.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const (
	// codexBaseURL is the base path Codex CLI talks to. The SDK
	// appends `/responses` so the resulting full URL is
	// https://chatgpt.com/backend-api/codex/responses.
	codexBaseURL = "https://chatgpt.com/backend-api/codex"

	// codexOriginator identifies us in request headers; matches the
	// value set in the OAuth authorize URL so the two ends agree.
	codexOriginator = "biene"
)

// ChatGPTCodexResolver is the credential gate the Codex provider uses
// to obtain a freshly-refreshed access_token plus the
// chatgpt-account-id required by the backend. Implementations live in
// the auth package; the interface keeps this provider free of any
// direct dependency on credential storage.
//
// IngestRateLimitHeaders is invoked once per upstream response with
// the raw headers, so the auth manager can cache the `x-codex-*`
// rate-limit family for later display in Settings. Implementations
// that don't care about quota tracking can leave the method as a
// no-op — the header parser tolerates missing fields.
type ChatGPTCodexResolver interface {
	APIKey(ctx context.Context) (string, error)
	AccountID(ctx context.Context) (string, error)
	IngestRateLimitHeaders(h http.Header)
}

// ChatGPTCodexProvider streams responses from the Codex backend and
// converts the result to Biene's internal block model.
type ChatGPTCodexProvider struct {
	resolver ChatGPTCodexResolver
	model    string
}

// NewChatGPTCodexProvider builds a provider bound to the given resolver
// and OpenAI model id (e.g. "gpt-5", "gpt-5-codex").
func NewChatGPTCodexProvider(resolver ChatGPTCodexResolver, model string) *ChatGPTCodexProvider {
	return &ChatGPTCodexProvider{resolver: resolver, model: model}
}

// Name surfaces the provider in logs so stale Anthropic/OpenAI mentions
// don't confuse triage.
func (p *ChatGPTCodexProvider) Name() string { return "chatgpt-codex/" + p.model }

// Stream implements api.Provider.
func (p *ChatGPTCodexProvider) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	tools []ToolDefinition,
	opts RequestOptions,
) (<-chan StreamEvent, error) {
	token, err := p.resolver.APIKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve chatgpt access token: %w", err)
	}
	accountID, err := p.resolver.AccountID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve chatgpt account id: %w", err)
	}

	// NewClient (rather than &Client{Options: ...}) is required so the
	// options propagate into the per-service constructors — without it
	// Responses gets a zero-value sub-service whose internal Options
	// list is empty, and the SDK's request builder fails with
	// "requestconfig: base url is not set" at send time.
	//
	// The Codex backend rejects requests without account/originator/
	// beta headers; WithAPIKey handles Authorization on its own.
	clientOpts := []option.RequestOption{
		option.WithBaseURL(codexBaseURL),
		option.WithAPIKey(token),
		option.WithHeader("chatgpt-account-id", accountID),
		option.WithHeader("originator", codexOriginator),
		option.WithHeader("OpenAI-Beta", "responses=experimental"),
		// Capture the response headers on every Codex turn and hand
		// them to the resolver so the rate-limit snapshot used by
		// the Settings panel stays current. The middleware runs
		// after the SDK has parsed status + headers but before the
		// SSE body iteration begins, so the data lands well ahead
		// of any in-stream events the caller will see.
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			resp, err := next(req)
			if resp != nil && p.resolver != nil {
				p.resolver.IngestRateLimitHeaders(resp.Header)
			}
			return resp, err
		}),
		// User-Agent identifies us to the Codex backend. pi-coding-agent
		// uses "pi (<os> <release>; <arch>)"; we keep the same shape so
		// if OpenAI ever starts UA-gating the endpoint our client looks
		// like a legitimate Codex-style consumer rather than a generic
		// openai-go default ("OpenAI/Go …"). runtime.GOOS / GOARCH are
		// stable per-binary, so this is computed lazily on the first
		// request and never changes.
		option.WithHeader("User-Agent", codexUserAgent()),
		// 3 retries matches pi-coding-agent. The SDK retries on
		// 408/409/429/5xx, on connection errors (no response), and
		// honors `Retry-After` / `Retry-After-Ms` headers — that
		// covers the upstream's "rate limit / overloaded / 503 from
		// origin" pattern without us reinventing the backoff loop.
		// Default is 2; one more attempt is the cheapest way to ride
		// out a brief edge-server hiccup the user would otherwise see
		// as a hard failure.
		option.WithMaxRetries(3),
	}
	// session_id + x-client-request-id keep consecutive turns of one
	// Biene session on the same cache shard upstream. Pi-coding-agent
	// sets both headers to the same value (the OpenAI session id) and
	// pairs them with the body-level prompt_cache_key below — without
	// the trio the backend may route the second turn to a worker that
	// doesn't have the prior turn's cache entry, defeating prompt cache.
	if sid := strings.TrimSpace(opts.SessionID); sid != "" {
		clientOpts = append(clientOpts,
			option.WithHeader("session_id", sid),
			option.WithHeader("x-client-request-id", sid),
		)
	}
	client := openai.NewClient(clientOpts...)

	inputItems, err := convertMessagesToResponsesInput(messages)
	if err != nil {
		return nil, fmt.Errorf("convert messages: %w", err)
	}

	params := responses.ResponseNewParams{
		Model:             responses.ResponsesModel(p.model),
		Store:             param.NewOpt(false),
		ParallelToolCalls: param.NewOpt(true),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputItems,
		},
		// `reasoning.encrypted_content` keeps reasoning round-trippable
		// on a stateless backend (store=false) — without it the model
		// can't see its own prior chain-of-thought on the next turn.
		Include: []responses.ResponseIncludable{
			responses.ResponseIncludableReasoningEncryptedContent,
		},
	}
	// prompt_cache_key tells the Codex backend "this request is part of
	// the same conversation as previous requests with the same key" so
	// it can serve the system prompt + leading history from cache. The
	// Biene session id is stable for the lifetime of a conversation, so
	// using it directly gives one cache bucket per agent.
	if sid := strings.TrimSpace(opts.SessionID); sid != "" {
		params.PromptCacheKey = param.NewOpt(sid)
	}
	// Reasoning effort + summary are session-controlled via the
	// existing thinking_on / thinking_off plumbing. The fragment lands
	// here as opts.ThinkingExtra["reasoning"]; the typed enums below
	// then drive shared.ReasoningParam, with per-model clamping so
	// effort values the upstream rejects (e.g. gpt-5.2+ refusing
	// "minimal") don't 400 the request.
	params.Reasoning = buildCodexReasoning(p.model, opts.ThinkingExtra)
	// text.verbosity defaults to "low" — pi-coding-agent's default
	// and a noticeably tighter answer length than the model's stock
	// behavior on the Codex backend. Biene's UX favors concise
	// answers (chat panes are narrow, the user rereads the diff
	// rather than a prose recap), so "low" is the right floor here.
	// Flip to "medium"/"high" only if a future per-session knob lands.
	params.Text = responses.ResponseTextConfigParam{
		Verbosity: responses.ResponseTextConfigVerbosityLow,
	}
	// service_tier is the latency-vs-cost knob ("flex" ≈0.5×, slower;
	// "priority" ≈2×–2.5×, faster). The session passes this through
	// from config; we drop unknown values so a typo doesn't 400 the
	// request.
	if tier := normalizeCodexServiceTier(opts.ServiceTier); tier != "" {
		params.ServiceTier = tier
	}
	if strings.TrimSpace(systemPrompt) != "" {
		params.Instructions = param.NewOpt(systemPrompt)
	}
	if len(tools) > 0 {
		params.Tools = convertToolsToResponses(tools)
	}

	stream := client.Responses.NewStreaming(ctx, params)

	out := make(chan StreamEvent, 64)
	go p.dispatchStream(stream, out)
	return out, nil
}

// dispatchStream is the long-lived goroutine that consumes the SSE
// event union and emits StreamEvents until the upstream closes. It is
// the core protocol-translation layer; everything else in this file
// just shapes inputs/outputs around it.
func (p *ChatGPTCodexProvider) dispatchStream(
	stream codexStream,
	out chan<- StreamEvent,
) {
	defer close(out)

	tracker := newCodexStreamTracker(out)
	for stream.Next() {
		ev := stream.Current()
		tracker.handle(ev)
	}
	if err := stream.Err(); err != nil {
		out <- StreamEvent{Type: EventError, Err: translateCodexError(err)}
		return
	}
	tracker.flush()
	out <- StreamEvent{Type: EventDone}
}

// codexServiceTiers is the allow-list of values the Codex backend
// accepts for `service_tier`. Anything else gets dropped at request
// build time — better to silently ignore a typo than to 400 every
// turn while the user wonders what changed.
var codexServiceTiers = map[string]responses.ResponseNewParamsServiceTier{
	"auto":     responses.ResponseNewParamsServiceTierAuto,
	"default":  responses.ResponseNewParamsServiceTierDefault,
	"flex":     responses.ResponseNewParamsServiceTierFlex,
	"scale":    responses.ResponseNewParamsServiceTierScale,
	"priority": responses.ResponseNewParamsServiceTierPriority,
}

// normalizeCodexServiceTier turns a free-form string into the typed
// SDK enum, or "" when the input isn't recognised. Empty input is
// treated as "no override" so callers don't need a separate guard.
func normalizeCodexServiceTier(raw string) responses.ResponseNewParamsServiceTier {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	return codexServiceTiers[v] // map zero-value is "" — exactly what we want
}

// codexUserAgent returns the User-Agent string sent on every Codex
// request. The shape "biene (<os> <arch>)" mirrors pi-coding-agent's
// "pi (<os> <release>; <arch>)" closely enough to look like a
// first-class Codex client to the backend without us reaching for
// platform-specific syscalls just to read the kernel release.
func codexUserAgent() string {
	return fmt.Sprintf("biene (%s %s)", runtime.GOOS, runtime.GOARCH)
}

// buildCodexReasoning translates a Biene thinking_on/_off fragment
// into shared.ReasoningParam, applying per-model effort clamps that
// the Codex backend silently enforces.
//
// The fragment shape this expects (declared in
// session_manager_config.chatgptOfficialEntry):
//
//	{"reasoning": {"effort": "high"|"low"|…, "summary": "auto"|…}}
//
// Returns the zero value when the fragment is missing or malformed —
// the SDK then sends no `reasoning` block at all and the model picks
// its own default, which matches the previous behavior so existing
// sessions aren't disrupted.
//
// Clamps mirror pi-coding-agent (clampReasoningEffort):
//   - gpt-5.2 / 5.3 / 5.4 / 5.5: refuse "minimal" → demote to "low"
//   - gpt-5.1: refuse "xhigh" → demote to "high"
//   - gpt-5.1-codex-mini: cap at "medium" unless caller asked for
//     "high"/"xhigh" (in which case "high" goes through)
func buildCodexReasoning(modelID string, extra map[string]any) shared.ReasoningParam {
	frag, _ := extra["reasoning"].(map[string]any)
	if len(frag) == 0 {
		return shared.ReasoningParam{}
	}

	out := shared.ReasoningParam{}
	if rawEffort, ok := frag["effort"].(string); ok && rawEffort != "" {
		out.Effort = shared.ReasoningEffort(clampCodexReasoningEffort(modelID, rawEffort))
	}
	// summary is non-load-bearing — when omitted the upstream picks
	// "auto" anyway. We still respect an explicit value so callers
	// can force "concise" / "detailed" without coding around the SDK.
	if rawSummary, ok := frag["summary"].(string); ok && rawSummary != "" {
		out.Summary = shared.ReasoningSummary(rawSummary)
	}
	return out
}

// clampCodexReasoningEffort returns the effort value that's actually
// safe to send for the given model id. Unknown / non-listed models
// pass through unchanged so newer releases keep working without us
// needing a code change for every variant.
func clampCodexReasoningEffort(modelID, effort string) string {
	id := modelID
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		id = id[idx+1:]
	}
	switch {
	case (strings.HasPrefix(id, "gpt-5.2") ||
		strings.HasPrefix(id, "gpt-5.3") ||
		strings.HasPrefix(id, "gpt-5.4") ||
		strings.HasPrefix(id, "gpt-5.5")) && effort == "minimal":
		return "low"
	case id == "gpt-5.1" && effort == "xhigh":
		return "high"
	case id == "gpt-5.1-codex-mini":
		if effort == "high" || effort == "xhigh" {
			return "high"
		}
		return "medium"
	}
	return effort
}

// codexStreamError shapes an in-stream error event into the same
// EventError shape the HTTP-level path uses, routing through
// friendlyCodexMessage when the body carries a usage-limit envelope.
//
// fallback is the un-parsed message from the event union; rawError is
// the JSON body of the nested ResponseError struct (or "" when the
// event isn't a `response.failed`); statusCode is 0 for in-stream
// failures since SSE doesn't surface one — friendlyCodexMessage's 429
// fallback only fires on real HTTP errors.
func codexStreamError(fallback, rawError string, statusCode int) error {
	if msg := friendlyCodexMessage(rawError, statusCode); msg != "" {
		return errors.New(msg)
	}
	if m := strings.TrimSpace(fallback); m != "" {
		return errors.New(m)
	}
	return errors.New("codex stream error")
}

// wrapResponseError lifts a ResponseError JSON blob (which is shaped
// like {"code": "...", "message": "...", "plan_type": "..."}) into the
// envelope friendlyCodexMessage expects ({"error": {...}}). Returns ""
// when the input is empty so the caller's status-only fallback path
// can take over.
func wrapResponseError(rawError string) string {
	s := strings.TrimSpace(rawError)
	if s == "" {
		return ""
	}
	return `{"error":` + s + `}`
}

// codexUsageLimitCodes is the set of error codes the Codex backend
// uses when a Plus/Pro/Team plan has run out of quota for the current
// rolling window. They share the same `plan_type` + `resets_at`
// envelope, so one regex is enough to route all three onto the same
// "you've hit your limit" message.
var codexUsageLimitCodes = regexp.MustCompile(
	`(?i)usage_limit_reached|usage_not_included|rate_limit_exceeded`,
)

// codexErrorBody mirrors the JSON envelope the Codex backend emits on
// non-2xx responses. plan_type / resets_at are out-of-band fields the
// typed SDK shapes don't surface (they live under
// `apierror.Error.JSON.ExtraFields`), so we re-parse the raw body.
type codexErrorBody struct {
	Error struct {
		Code     string `json:"code"`
		Type     string `json:"type"`
		Message  string `json:"message"`
		PlanType string `json:"plan_type"`
		// resets_at is unix seconds. The backend sometimes sends it
		// as a number, sometimes as a numeric string — json.Number
		// accepts both without us hand-rolling a UnmarshalJSON.
		ResetsAt json.Number `json:"resets_at"`
	} `json:"error"`
}

// translateCodexError turns the SDK's HTTP error into something a user
// can act on. The most common case is "you've hit your ChatGPT usage
// limit"; without this translation users see a raw 429 / opaque JSON
// blob and don't know whether to retry, wait, or check their plan.
//
// Returns the original error untouched if it's not a recognised Codex
// envelope — better to bubble up an unfamiliar message than to mask it
// with a misleading "usage limit reached" when something else broke.
func translateCodexError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	if msg := friendlyCodexMessage(apiErr.RawJSON(), apiErr.StatusCode); msg != "" {
		return errors.New(msg)
	}
	// Fall back to the SDK's parsed message field if the raw envelope
	// didn't yield anything useful (e.g. the body wasn't JSON).
	if apiErr.Message != "" {
		return errors.New(apiErr.Message)
	}
	return err
}

// friendlyCodexMessage examines a raw error body and returns a
// human-readable string when it matches a known Codex pattern, or ""
// when nothing actionable could be extracted.
//
// statusCode is consulted as a fallback: a 429 with no parseable code
// is still a usage-limit signal worth surfacing as such.
func friendlyCodexMessage(rawBody string, statusCode int) string {
	if rawBody == "" {
		if statusCode == 429 {
			return "You have hit your ChatGPT usage limit."
		}
		return ""
	}
	var env codexErrorBody
	if err := json.Unmarshal([]byte(rawBody), &env); err != nil {
		return ""
	}
	code := env.Error.Code
	if code == "" {
		code = env.Error.Type
	}

	if codexUsageLimitCodes.MatchString(code) || statusCode == 429 {
		var b strings.Builder
		b.WriteString("You have hit your ChatGPT usage limit")
		if env.Error.PlanType != "" {
			b.WriteString(" (")
			b.WriteString(strings.ToLower(env.Error.PlanType))
			b.WriteString(" plan)")
		}
		b.WriteString(".")
		if mins, ok := codexResetMinutes(env.Error.ResetsAt); ok {
			fmt.Fprintf(&b, " Try again in ~%d min.", mins)
		}
		return b.String()
	}

	if env.Error.Message != "" {
		return env.Error.Message
	}
	return ""
}

// codexResetMinutes converts the resets_at unix-seconds value into
// "minutes from now", clamped at zero. Returns ok=false when the
// timestamp is missing or unparseable so the caller can omit the "try
// again in ~X min" tail entirely.
func codexResetMinutes(raw json.Number) (int, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false
	}
	secs, err := raw.Int64()
	if err != nil {
		// Some older Codex builds emit floats; fall back through
		// Float64 → Int64 truncation rather than failing outright.
		f, ferr := raw.Float64()
		if ferr != nil {
			return 0, false
		}
		secs = int64(f)
	}
	if secs <= 0 {
		return 0, false
	}
	delta := time.Until(time.Unix(secs, 0))
	mins := int(delta.Round(time.Minute).Minutes())
	if mins < 0 {
		mins = 0
	}
	return mins, true
}

// codexStream abstracts the openai-go SSE iterator so tests can swap
// it for a static slice without spinning up a real HTTP server. The
// real implementation is *ssestream.Stream[ResponseStreamEventUnion].
type codexStream interface {
	Next() bool
	Current() responses.ResponseStreamEventUnion
	Err() error
}

// codexStreamTracker is the per-stream state machine that maps the
// Responses API's item lifecycle onto Biene's block model.
//
// The Responses API emits events at three nested levels:
//
//   - `response.output_item.added/done` opens/closes an *item*
//     (reasoning, message, function_call).
//   - Inside a message, `response.output_text.delta/done` carries the
//     visible text. Inside a reasoning item,
//     `response.reasoning_summary_text.delta/done` carries the visible
//     thought summary.
//   - `response.function_call_arguments.delta/done` carries the
//     streaming JSON for tool calls.
//
// We track the currently-open item type so each delta can route to the
// right Biene block. Tool calls also need an id-mapping table because
// Biene's downstream agentloop keys tool_results by Biene's tool_use
// id, but the Codex backend returns two distinct ids per call (`id`
// and `call_id`). We follow pi-coding-agent and surface "callID|itemID"
// as the Biene id, so a later ToolResultBlock can be split back into
// the call_id we owe the backend.
type codexStreamTracker struct {
	out      chan<- StreamEvent
	openItem string // "reasoning" | "message" | "function_call" | ""

	// activeToolUseID is set while a function_call item is open so
	// argument deltas can be routed to the right EventInputJSONDelta.
	activeToolUseID string
	// activeToolName retains the call's name across argument deltas
	// (the SDK does not echo it on every delta event).
	activeToolName string
	// activeToolArgs accumulates the streaming JSON so the final
	// EventToolUse can be emitted with the parsed body when
	// `function_call_arguments.done` fires.
	activeToolArgs strings.Builder
}

func newCodexStreamTracker(out chan<- StreamEvent) *codexStreamTracker {
	return &codexStreamTracker{out: out}
}

func (t *codexStreamTracker) handle(ev responses.ResponseStreamEventUnion) {
	switch ev.Type {
	case "response.output_item.added":
		t.onItemAdded(ev)

	case "response.reasoning_summary_text.delta":
		if ev.Delta != "" {
			t.out <- StreamEvent{Type: EventReasoningDelta, Text: ev.Delta}
		}

	case "response.reasoning_summary_part.done":
		// Reasoning is split into multiple "summary parts" per item.
		// pi-coding-agent inserts a paragraph break between them so
		// the user-visible text reads as one continuous thought.
		t.out <- StreamEvent{Type: EventReasoningDelta, Text: "\n\n"}

	case "response.output_text.delta":
		if ev.Delta != "" {
			t.out <- StreamEvent{Type: EventTextDelta, Text: ev.Delta}
		}

	case "response.refusal.delta":
		// Refusals are surfaced as plain text so the user sees the
		// model's apology rather than a silent empty turn.
		if ev.Delta != "" {
			t.out <- StreamEvent{Type: EventTextDelta, Text: ev.Delta}
		}

	case "response.function_call_arguments.delta":
		if t.activeToolUseID != "" && ev.Delta != "" {
			t.activeToolArgs.WriteString(ev.Delta)
			t.out <- StreamEvent{
				Type:      EventInputJSONDelta,
				ToolUseID: t.activeToolUseID,
				InputJSON: ev.Delta,
			}
		}

	case "response.function_call_arguments.done":
		// Replace any partial buffer with the canonical final string
		// so downstream consumers parse exactly what the backend sent.
		if ev.Arguments != "" {
			t.activeToolArgs.Reset()
			t.activeToolArgs.WriteString(ev.Arguments)
		}

	case "response.output_item.done":
		t.onItemDone(ev)

	case "response.completed":
		if ev.Response.Usage.InputTokens != 0 || ev.Response.Usage.OutputTokens != 0 {
			cached := ev.Response.Usage.InputTokensDetails.CachedTokens
			// InputTokens stays as "fresh tokens" (total minus cache
			// hits) so compaction triggers compare like-with-like
			// against the model's effective remaining-context budget.
			// CacheReadTokens carries the hit count separately for UI
			// surfaces. The Codex backend's prompt-cache writes are
			// implicit (no separate billable counter), so
			// CacheWriteTokens stays zero — Anthropic providers fill
			// it in when their API does break it out.
			t.out <- StreamEvent{
				Type: EventUsage,
				Usage: Usage{
					InputTokens:     int(ev.Response.Usage.InputTokens - cached),
					OutputTokens:    int(ev.Response.Usage.OutputTokens),
					CacheReadTokens: int(cached),
				},
			}
		}

	case "error":
		// `error` events carry the message at the top level of the
		// union (ResponseErrorEvent shape).
		t.out <- StreamEvent{Type: EventError, Err: codexStreamError(ev.Message, "", 0)}

	case "response.failed":
		// `response.failed` puts the failure detail under
		// ev.Response.Error (ResponseError). The raw JSON of that
		// nested struct also carries plan_type / resets_at when the
		// failure is a usage-limit hit, so we hand it off to the
		// same friendly-message extractor used for HTTP errors.
		t.out <- StreamEvent{
			Type: EventError,
			Err: codexStreamError(
				ev.Response.Error.Message,
				wrapResponseError(ev.Response.Error.RawJSON()),
				0,
			),
		}
	}
}

func (t *codexStreamTracker) onItemAdded(ev responses.ResponseStreamEventUnion) {
	switch ev.Item.Type {
	case "reasoning":
		t.openItem = "reasoning"

	case "message":
		t.openItem = "message"

	case "function_call":
		t.openItem = "function_call"
		// pi-coding-agent uses "callID|itemID" so a later round-trip
		// can recover the call_id that the Codex backend insists on.
		// Biene's outer loop only sees the composite id; we split it
		// back out when converting tool results into input items.
		callID := ev.Item.CallID
		itemID := ev.Item.ID
		t.activeToolUseID = callID + "|" + itemID
		t.activeToolName = ev.Item.Name
		t.activeToolArgs.Reset()
		t.activeToolArgs.WriteString(ev.Item.Arguments.OfString) // usually ""
		t.out <- StreamEvent{
			Type: EventToolUseStart,
			ToolUse: &ToolUseBlock{
				ID:   t.activeToolUseID,
				Name: t.activeToolName,
			},
		}
	}
}

func (t *codexStreamTracker) onItemDone(ev responses.ResponseStreamEventUnion) {
	switch ev.Item.Type {
	case "reasoning":
		t.openItem = ""

	case "message":
		t.openItem = ""

	case "function_call":
		args := []byte(t.activeToolArgs.String())
		if !json.Valid(args) {
			// Empty / malformed args go out as an empty object so the
			// downstream agentloop has something to parse instead of
			// erroring on a partial JSON fragment.
			args = []byte("{}")
		}
		t.out <- StreamEvent{
			Type: EventToolUse,
			ToolUse: &ToolUseBlock{
				ID:    t.activeToolUseID,
				Name:  t.activeToolName,
				Input: args,
			},
		}
		t.openItem = ""
		t.activeToolUseID = ""
		t.activeToolName = ""
		t.activeToolArgs.Reset()
	}
}

// flush is invoked after the stream closes cleanly to surface any
// in-flight tool call as a final event. Under normal conditions
// `output_item.done` already does this; flush guards against a
// truncated stream so the outer loop sees something coherent.
func (t *codexStreamTracker) flush() {
	if t.openItem == "function_call" && t.activeToolUseID != "" {
		args := []byte(t.activeToolArgs.String())
		if !json.Valid(args) {
			args = []byte("{}")
		}
		t.out <- StreamEvent{
			Type: EventToolUse,
			ToolUse: &ToolUseBlock{
				ID:    t.activeToolUseID,
				Name:  t.activeToolName,
				Input: args,
			},
		}
	}
}

// ─── Message conversion ───────────────────────────────────────────────

// convertMessagesToResponsesInput translates Biene's flat
// (role + ContentBlock list) format into the Responses API's
// item-list shape. Each Biene block becomes its own input item, and
// runs of plain text in a single turn collapse into one message item
// to match how the API expects them.
func convertMessagesToResponsesInput(messages []Message) (responses.ResponseInputParam, error) {
	out := make(responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role == "" {
			role = RoleUser
		}

		// Pass 1: collect text + image blocks for the message item.
		// We split into two emission paths:
		//  - Text-only → the easy-input fast path. This is the vast
		//    majority of turns and keeps the wire format minimal.
		//  - Has images → a structured content list. The Responses
		//    API's `input_image` part only exists on the message-form
		//    item (not on EasyInputMessage), and can only sit inside a
		//    user role anyway.
		//
		// Assistant turns never carry images here (the model doesn't
		// emit them through this path), so image-bearing messages are
		// always anchored to role=user.
		var textParts []string
		var imageBlocks []ImageBlock
		for _, block := range msg.Content {
			switch b := block.(type) {
			case TextBlock:
				if b.Text != "" {
					textParts = append(textParts, b.Text)
				}
			case ImageBlock:
				if len(b.Data) > 0 {
					imageBlocks = append(imageBlocks, b)
				}
				// Drop blocks with no Data: the session layer is
				// supposed to inline the bytes just before handing
				// the message to the provider (see ImageBlock doc in
				// types.go). An empty Data block at this point means
				// either persistence skipped the inline step or the
				// caller is replaying without rehydrating — either
				// way the model can't see anything useful.
			}
		}
		if len(imageBlocks) > 0 {
			// Mixed text + image, or image-only. Build the structured
			// item; role is forced to user (developer/system can also
			// carry the message form, but in practice only user
			// messages reach this branch).
			out = append(out, buildCodexUserMessageWithImages(textParts, imageBlocks))
		} else if len(textParts) > 0 {
			text := strings.Join(textParts, "\n")
			out = append(out, responses.ResponseInputItemParamOfMessage(text, easyRoleFor(role)))
		}

		// Pass 2: tool use / tool result / reasoning items, in source order.
		for _, block := range msg.Content {
			switch b := block.(type) {
			case ToolUseBlock:
				callID, itemID := splitCodexToolUseID(b.ID)
				args := string(b.Input)
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				item := responses.ResponseInputItemParamOfFunctionCall(args, callID, b.Name)
				if itemID != "" && item.OfFunctionCall != nil {
					item.OfFunctionCall.ID = param.NewOpt(itemID)
				}
				out = append(out, item)

			case ToolResultBlock:
				callID, _ := splitCodexToolUseID(b.ToolUseID)
				out = append(out,
					responses.ResponseInputItemParamOfFunctionCallOutput(callID, b.Content))

			case ReasoningBlock:
				// b.Signature carries the encrypted_content blob the
				// backend emitted on the previous turn — required for
				// statelessly continuing the same line of reasoning.
				if b.Signature == "" {
					continue
				}
				item := responses.ResponseInputItemUnionParam{
					OfReasoning: &responses.ResponseReasoningItemParam{
						EncryptedContent: param.NewOpt(b.Signature),
					},
				}
				out = append(out, item)
			}
		}
	}
	return out, nil
}

// buildCodexUserMessageWithImages constructs a structured user message
// that mixes text and image content parts. The Responses API supports
// images only via the `message`-form item (not via the EasyInputMessage
// fast path), so any user turn that carries even one image has to take
// this slower branch.
//
// Each image is sent as an `input_image` content part with an inline
// data URI; we don't upload to OpenAI's files API because:
//
//   - The Codex backend (chatgpt.com/backend-api) is a different
//     surface from platform.openai.com and doesn't share the file
//     store.
//   - Biene attachments are typically small (screenshots, snippets);
//     a base64 data URI keeps the request self-contained without an
//     extra round-trip.
//
// Detail is fixed to "auto" — letting the model pick "high" for
// detailed crops vs "low" for thumbnails. We don't expose a tuner
// because the user-side workflow doesn't have a knob for it.
func buildCodexUserMessageWithImages(textParts []string, images []ImageBlock) responses.ResponseInputItemUnionParam {
	content := make(responses.ResponseInputMessageContentListParam, 0, len(textParts)+len(images))
	if len(textParts) > 0 {
		// Collapse multiple TextBlocks into a single input_text part —
		// matches how the easy-input fast path joins them, and gives
		// the model one continuous prose preamble before the images.
		text := strings.Join(textParts, "\n")
		content = append(content, responses.ResponseInputContentUnionParam{
			OfInputText: &responses.ResponseInputTextParam{Text: text},
		})
	}
	for _, img := range images {
		uri := buildImageDataURI(img.MediaType, img.Data)
		content = append(content, responses.ResponseInputContentUnionParam{
			OfInputImage: &responses.ResponseInputImageParam{
				ImageURL: param.NewOpt(uri),
				Detail:   responses.ResponseInputImageDetailAuto,
			},
		})
	}
	return responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role:    "user",
			Content: content,
		},
	}
}

// buildImageDataURI base64-encodes the bytes into the `data:<mt>;base64,…`
// form OpenAI's `input_image.image_url` accepts. MediaType falls back
// to image/png when missing — that's the most common attachment in
// Biene's flow (screenshot pastes default to PNG).
func buildImageDataURI(mediaType string, data []byte) string {
	mt := strings.TrimSpace(mediaType)
	if mt == "" {
		mt = "image/png"
	}
	return "data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// easyRoleFor maps Biene's role strings to the Responses API's enum.
// Anything we don't recognize falls back to "user" — the API will
// reject it explicitly if that turns out to be wrong, which is better
// than us silently dropping a turn.
func easyRoleFor(role string) responses.EasyInputMessageRole {
	switch role {
	case RoleAssistant:
		return responses.EasyInputMessageRoleAssistant
	case "system":
		return responses.EasyInputMessageRoleSystem
	default:
		return responses.EasyInputMessageRoleUser
	}
}

// splitCodexToolUseID undoes the "callID|itemID" composite the tracker
// builds when emitting EventToolUseStart. ToolUseBlocks that came
// from other providers (no pipe in the id) round-trip as call_id
// directly so a session migrated mid-conversation still works.
func splitCodexToolUseID(id string) (callID, itemID string) {
	if idx := strings.Index(id, "|"); idx >= 0 {
		return id[:idx], id[idx+1:]
	}
	return id, ""
}

// ─── Tool conversion ──────────────────────────────────────────────────

// convertToolsToResponses translates Biene's ToolDefinition list into
// the function-tool variant of the Responses API tool union. Strict
// mode is left off because Biene tools don't pin themselves to the
// JSON Schema "additionalProperties: false" requirement strict mode
// imposes.
func convertToolsToResponses(tools []ToolDefinition) []responses.ToolUnionParam {
	out := make([]responses.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		var params map[string]any
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &params)
		}
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tool := responses.ToolParamOfFunction(t.Name, params, false)
		if tool.OfFunction != nil && t.Description != "" {
			tool.OfFunction.Description = param.NewOpt(t.Description)
		}
		out = append(out, tool)
	}
	return out
}
