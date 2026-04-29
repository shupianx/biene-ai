package api

import (
	"context"
	"fmt"
)

// ErrorProvider is a stub Provider that fails every Stream call with a
// pre-baked error. It exists for cases where the SessionManager has to
// hand back a Provider but the underlying credential / dependency is
// missing (e.g. a session pinned to chatgpt_official that survived a
// logout).
//
// Returning a real provider with empty credentials would mask the root
// cause behind a generic "API key invalid" message. The stub instead
// surfaces the actual reason verbatim — auth.ErrChatGPTNotAuthenticated
// at the time of writing — so the renderer's chat composer can show
// "ChatGPT 未授权" instead of a confusing OpenAI 401.
//
// Stream returns the error rather than a channel-with-error so callers
// don't need a goroutine just to read EventError. The agentloop already
// handles a non-nil error from Stream by propagating it as a turn-level
// failure, which is exactly the UX we want here.
type ErrorProvider struct {
	// label distinguishes this stub from real providers in logs. It's
	// echoed by Name() and used only for display.
	label string
	// err is the failure surfaced from Stream. Non-nil by construction
	// — NewErrorProvider rejects a nil error so misuse is caught at
	// build time rather than at first request.
	err error
}

// NewErrorProvider builds a stub that fails with err on every call.
// Panics if err is nil — passing nil would silently produce a Provider
// that never produces output, which is worse than a clear failure.
func NewErrorProvider(label string, err error) *ErrorProvider {
	if err == nil {
		panic("api.NewErrorProvider: err must not be nil")
	}
	return &ErrorProvider{label: label, err: err}
}

func (p *ErrorProvider) Name() string {
	if p.label == "" {
		return "error"
	}
	return p.label
}

func (p *ErrorProvider) Stream(
	_ context.Context,
	_ string,
	_ []Message,
	_ []ToolDefinition,
	_ RequestOptions,
) (<-chan StreamEvent, error) {
	// %w preserves the underlying sentinel so callers using errors.Is
	// against e.g. auth.ErrChatGPTNotAuthenticated still match.
	return nil, fmt.Errorf("%s: %w", p.Name(), p.err)
}
