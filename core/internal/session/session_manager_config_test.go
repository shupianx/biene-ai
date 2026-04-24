package session

import (
	"testing"

	"tinte/internal/config"
	"tinte/internal/prompt"
	"tinte/internal/tools"
)

func TestCreatePinsSelectedModel(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "main",
		ModelList: []config.ModelEntry{
			{ID: "main", Name: "Main", Provider: "anthropic", Model: "claude-opus-4-6"},
			{ID: "backup", Name: "Backup", Provider: "openai_compatible", Model: "gpt-5"},
		},
	}
	mgr := NewSessionManager(t.TempDir(), cfg)

	sess, err := mgr.Create("Agent", tools.PermissionSet{}, prompt.DefaultProfile(), "backup")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	meta := sess.Meta()
	if meta.ModelID != "backup" {
		t.Fatalf("expected pinned model id backup, got %q", meta.ModelID)
	}
	if meta.ModelName != "Backup" {
		t.Fatalf("expected pinned model name Backup, got %q", meta.ModelName)
	}

	usage := mgr.ModelUsageCounts()
	if usage["backup"] != 1 {
		t.Fatalf("expected backup usage count 1, got %d", usage["backup"])
	}
}

func TestCreateDefaultsThinkingOffWhenModelSupportsIt(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "main",
		ModelList: []config.ModelEntry{
			{
				ID:                "main",
				Name:              "Main",
				Provider:          "openai_compatible",
				Model:             "qwen3.6-plus",
				ThinkingAvailable: true,
			},
		},
	}
	mgr := NewSessionManager(t.TempDir(), cfg)

	sess, err := mgr.Create("Agent", tools.PermissionSet{}, prompt.DefaultProfile(), "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	meta := sess.Meta()
	if !meta.ThinkingAvailable {
		t.Fatal("expected thinking to be available")
	}
	if meta.ThinkingEnabled {
		t.Fatal("expected new session thinking to default to disabled")
	}
}

func TestUpdateConfigRefreshesPinnedModelName(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "main",
		ModelList: []config.ModelEntry{
			{ID: "main", Name: "Main", Provider: "anthropic", Model: "claude-opus-4-6"},
			{ID: "backup", Name: "Backup", Provider: "openai_compatible", Model: "gpt-5"},
		},
	}
	mgr := NewSessionManager(t.TempDir(), cfg)

	mainSess, err := mgr.Create("Main Agent", tools.PermissionSet{}, prompt.DefaultProfile(), "")
	if err != nil {
		t.Fatalf("Create main session: %v", err)
	}
	backupSess, err := mgr.Create("Backup Agent", tools.PermissionSet{}, prompt.DefaultProfile(), "backup")
	if err != nil {
		t.Fatalf("Create backup session: %v", err)
	}

	next := &config.Config{
		DefaultModel: "main",
		ModelList: []config.ModelEntry{
			{ID: "main", Name: "Primary", Provider: "anthropic", Model: "claude-opus-4-6"},
			{ID: "backup", Name: "Research", Provider: "openai_compatible", Model: "gpt-5"},
		},
	}

	if err := mgr.UpdateConfig(next); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if got := mainSess.Meta().ModelName; got != "Primary" {
		t.Fatalf("expected updated main model name Primary, got %q", got)
	}
	if got := backupSess.Meta().ModelName; got != "Research" {
		t.Fatalf("expected updated backup model name Research, got %q", got)
	}
}
