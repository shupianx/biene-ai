package config

import "testing"

func TestGetModelUsesID(t *testing.T) {
	cfg := &Config{
		DefaultModel: "main",
		ModelList: []ModelEntry{
			{
				ID:       "main",
				Name:     "Main",
				Provider: "anthropic",
				Model:    "claude-opus-4-6",
			},
		},
	}

	entry, err := cfg.GetModel("")
	if err != nil {
		t.Fatalf("GetModel returned error: %v", err)
	}
	if entry.ID != "main" {
		t.Fatalf("expected id lookup, got %q", entry.ID)
	}
}

