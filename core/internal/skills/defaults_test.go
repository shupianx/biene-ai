package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallDefaultEnabledCopiesConfiguredGlobalSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	globalRoot, err := EnsureGlobalRoot()
	if err != nil {
		t.Fatalf("EnsureGlobalRoot returned error: %v", err)
	}

	firstDir := filepath.Join(globalRoot, "reviewer")
	secondDir := filepath.Join(globalRoot, "release-notes")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := `---
name: ` + filepath.Base(dir) + `
description: test skill
---
Use this skill.
`
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	configPath, err := skillConfigPath()
	if err != nil {
		t.Fatalf("skillConfigPath returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{\n  \"defaultEnabledSkillDirs\": [\n    "+jsonString(firstDir)+",\n    "+jsonString(secondDir)+"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()
	if err := InstallDefaultEnabled(workDir); err != nil {
		t.Fatalf("InstallDefaultEnabled returned error: %v", err)
	}

	for _, dir := range []string{firstDir, secondDir} {
		target := filepath.Join(workDir, ".biene", "skills", filepath.Base(dir), "SKILL.md")
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("expected copied skill file at %s: %v", target, err)
		}
	}
}

func TestLoadGlobalSkillConfigMigratesLegacyDefaultSkillDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	globalRoot, err := EnsureGlobalRoot()
	if err != nil {
		t.Fatalf("EnsureGlobalRoot returned error: %v", err)
	}

	legacyDir := filepath.Join(globalRoot, "triage")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath, err := skillConfigPath()
	if err != nil {
		t.Fatalf("skillConfigPath returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{\n  \"defaultSkillDir\": "+jsonString(legacyDir)+"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadGlobalSkillConfig()
	if err != nil {
		t.Fatalf("loadGlobalSkillConfig returned error: %v", err)
	}
	if len(cfg.DefaultEnabledSkillDirs) != 1 || cfg.DefaultEnabledSkillDirs[0] != filepath.Clean(legacyDir) {
		t.Fatalf("expected migrated legacy dir, got %#v", cfg.DefaultEnabledSkillDirs)
	}
}

func jsonString(value string) string {
	return `"` + value + `"`
}
