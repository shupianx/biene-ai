package templates

import "testing"

func TestLookupContextWindowExactMatch(t *testing.T) {
	got, ok := LookupContextWindow("openai_compatible", "deepseek-v4-flash", "https://api.deepseek.com")
	if !ok {
		t.Fatal("expected match for deepseek-v4-flash")
	}
	if got != 128000 {
		t.Fatalf("expected 128000, got %d", got)
	}
}

func TestLookupContextWindow_TrailingSlashTolerant(t *testing.T) {
	got, ok := LookupContextWindow("openai_compatible", "deepseek-v4-flash", "https://api.deepseek.com/")
	if !ok || got != 128000 {
		t.Fatalf("trailing slash should still match, got (%d, %v)", got, ok)
	}
}

func TestLookupContextWindow_ProviderCaseInsensitive(t *testing.T) {
	got, ok := LookupContextWindow("Anthropic", "claude-opus-4-7", "https://api.anthropic.com")
	if !ok || got != 200000 {
		t.Fatalf("expected case-insensitive provider match, got (%d, %v)", got, ok)
	}
}

func TestLookupContextWindow_NoMatch(t *testing.T) {
	if _, ok := LookupContextWindow("anthropic", "fake-model-9000", "https://api.anthropic.com"); ok {
		t.Fatal("unknown model should not match")
	}
	if _, ok := LookupContextWindow("openai_compatible", "deepseek-v4-flash", "https://impostor.com"); ok {
		t.Fatal("wrong base URL should not match")
	}
}
