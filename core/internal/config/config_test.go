package config

import "testing"

func TestNormalizeMigratesLegacyDefaultModelNameToID(t *testing.T) {
	cfg := &Config{
		DefaultModel: "Main Provider",
		ModelList: []ModelEntry{
			{
				Name:     "Main Provider",
				Provider: "anthropic",
				Model:    "claude-opus-4-6",
			},
			{
				Name:     "Backup",
				Provider: "openai",
				Model:    "gpt-5",
			},
		},
	}

	changed := Normalize(cfg)
	if !changed {
		t.Fatalf("expected normalization to update legacy config")
	}
	if cfg.ModelList[0].ID != "main-provider" {
		t.Fatalf("expected first id to be derived from name, got %q", cfg.ModelList[0].ID)
	}
	if cfg.ModelList[1].ID != "backup" {
		t.Fatalf("expected second id to be derived from name, got %q", cfg.ModelList[1].ID)
	}
	if cfg.DefaultModel != "main-provider" {
		t.Fatalf("expected default model to use id, got %q", cfg.DefaultModel)
	}
	if cfg.ModelList[1].Provider != "openai_compatible" {
		t.Fatalf("expected provider alias to normalize, got %q", cfg.ModelList[1].Provider)
	}
}

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

func TestNormalizeAutoEnablesThinkingForQwen36Plus(t *testing.T) {
	cfg := &Config{
		DefaultModel: "qwen",
		ModelList: []ModelEntry{
			{
				ID:       "qwen",
				Name:     "Qwen",
				Provider: "openai_compatible",
				Model:    "qwen3.6-plus",
				BaseURL:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
			},
		},
	}

	changed := Normalize(cfg)
	if !changed {
		t.Fatalf("expected normalization to add thinking support")
	}
	if !cfg.ModelList[0].ThinkingAvailable {
		t.Fatal("expected qwen3.6-plus entry to enable thinking support")
	}
}
