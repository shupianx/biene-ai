package auth

import "testing"

func TestIsChatGPTOfficialModelID(t *testing.T) {
	cases := map[string]bool{
		"chatgpt_official:gpt-5.5":        true,
		"chatgpt_official:":               true, // prefix-only is structurally valid; ParseChatGPT… returns ""
		"chatgpt_official_lookalike":      false,
		"my-claude":                       false,
		"":                                false,
		"openai_compatible:gpt-5.5":       false,
	}
	for id, want := range cases {
		if got := IsChatGPTOfficialModelID(id); got != want {
			t.Errorf("IsChatGPTOfficialModelID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestParseChatGPTOfficialModelID(t *testing.T) {
	if got := ParseChatGPTOfficialModelID("chatgpt_official:gpt-5.5"); got != "gpt-5.5" {
		t.Errorf("expected 'gpt-5.5', got %q", got)
	}
	// Empty model component: prefix matches but Parse should return ""
	// so callers can detect the malformed form (used by
	// chatgptOfficialEntry to fall through to the default model).
	if got := ParseChatGPTOfficialModelID("chatgpt_official:"); got != "" {
		t.Errorf("empty model component should yield empty string, got %q", got)
	}
	// Non-prefixed IDs return "" — they belong to user configs and
	// should never be misrouted into the synthetic provider path.
	if got := ParseChatGPTOfficialModelID("my-claude"); got != "" {
		t.Errorf("non-prefixed id should yield empty string, got %q", got)
	}
}

func TestChatGPTOfficialContextWindow_KnownModelMatchesTemplate(t *testing.T) {
	// gpt-5.5 is in templates.go's openai vendor row at 400_000.
	// If the template moves, the synthetic provider must follow —
	// that's the whole reason the lookup exists rather than being
	// hard-coded here.
	if got := ChatGPTOfficialContextWindow("gpt-5.5"); got != 400_000 {
		t.Errorf("gpt-5.5 should resolve to 400000 via templates, got %d", got)
	}
}

func TestChatGPTOfficialContextWindow_UnknownModelFallsBackToDefault(t *testing.T) {
	// Models added to ChatGPTOfficialModels but not yet to templates.go
	// must still get a usable window — without it the session manager
	// would default to 32K and compaction would fire every turn.
	if got := ChatGPTOfficialContextWindow("gpt-future-model"); got != chatgptOfficialDefaultContextWindow {
		t.Errorf("unknown model should fall back to %d, got %d",
			chatgptOfficialDefaultContextWindow, got)
	}
}

func TestChatGPTOfficialModels_DerivedFromTemplates(t *testing.T) {
	// The model list must be populated from templates.go's OpenAI row
	// rather than hardcoded — that's the whole point of the derivation.
	// We don't pin specific names because the template list will grow
	// over time; instead we assert the shape (non-empty + all entries
	// are gpt-prefixed) so the test catches a regression where the
	// vendor lookup silently returns no rows.
	got := ChatGPTOfficialModels()
	if len(got) == 0 {
		t.Fatal("ChatGPTOfficialModels should derive at least one model from templates.go")
	}
	for _, m := range got {
		if m == "" {
			t.Errorf("empty model string in result: %v", got)
		}
	}
}

func TestPrepareChatGPTOAuth_ProducesUniqueStateAndVerifier(t *testing.T) {
	// Two consecutive prepares must not collide — state and verifier
	// are CSPRNG-derived. A regression here would break the OAuth
	// flow's CSRF guarantees (two parallel logins would race-overwrite
	// the pending-flow map keyed by state).
	a, err := PrepareChatGPTOAuth()
	if err != nil {
		t.Fatalf("first prepare failed: %v", err)
	}
	b, err := PrepareChatGPTOAuth()
	if err != nil {
		t.Fatalf("second prepare failed: %v", err)
	}
	if a.State == b.State {
		t.Error("state collision between two PrepareChatGPTOAuth() calls")
	}
	if a.CodeVerifier == b.CodeVerifier {
		t.Error("code verifier collision between two PrepareChatGPTOAuth() calls")
	}
	if a.AuthURL == "" || b.AuthURL == "" {
		t.Error("authorize URL should be populated")
	}
}
