package api

import (
	"context"
	"errors"
	"testing"
)

func TestErrorProvider_StreamReturnsWrappedSentinel(t *testing.T) {
	// Sentinel-style errors are the typical use case: callers use
	// errors.Is to detect a specific failure mode (e.g. "not signed
	// in to ChatGPT"). The %w wrapping inside Stream must preserve
	// that match through one hop of fmt.Errorf.
	sentinel := errors.New("not signed in")
	p := NewErrorProvider("chatgpt-official/gpt-5.5", sentinel)

	ch, err := p.Stream(context.Background(), "", nil, nil, RequestOptions{})
	if ch != nil {
		t.Errorf("Stream should return nil channel on failure, got %v", ch)
	}
	if err == nil {
		t.Fatal("expected non-nil error from ErrorProvider")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("err should wrap sentinel, got %v", err)
	}
}

func TestErrorProvider_NameDefaultsAndOverride(t *testing.T) {
	if got := NewErrorProvider("custom-label", errors.New("x")).Name(); got != "custom-label" {
		t.Errorf("Name should return label, got %q", got)
	}
	// Empty label falls back to a recognisable string for log triage.
	if got := NewErrorProvider("", errors.New("x")).Name(); got != "error" {
		t.Errorf("empty-label Name should be 'error', got %q", got)
	}
}

func TestErrorProvider_NilErrorPanics(t *testing.T) {
	// A nil-error stub would silently produce a Provider that fails
	// every call with a meaningless "<nil>" message. Catch the misuse
	// at construction time instead.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on nil error")
		}
	}()
	_ = NewErrorProvider("x", nil)
}
