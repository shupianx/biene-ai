package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanForWorkDirFindsAgentSkills(t *testing.T) {
	workDir := t.TempDir()
	skillDir := filepath.Join(workDir, ".tinte", "skills", "release-notes")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: release-notes
description: Generate release notes from recent changes
---
# Release Notes
Use {baseDir} as your reference folder.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	metas, err := ScanForWorkDir(workDir)
	if err != nil {
		t.Fatalf("ScanForWorkDir returned error: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(metas))
	}
	if metas[0].Name != "release-notes" {
		t.Fatalf("expected skill name release-notes, got %q", metas[0].Name)
	}
	def, err := LoadDefinition(metas[0])
	if err != nil {
		t.Fatalf("LoadDefinition returned error: %v", err)
	}
	if !strings.Contains(def.Instructions, skillDir) {
		t.Fatalf("expected {baseDir} replacement, got %q", def.Instructions)
	}
}

func TestScanFromDirIgnoresInvalidSkills(t *testing.T) {
	root := t.TempDir()
	validDir := filepath.Join(root, "valid")
	invalidDir := filepath.Join(root, "invalid")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	valid := `---
name: reviewer
description: Review diffs carefully
---
Check correctness first.
`
	invalid := `# missing frontmatter`

	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}

	metas, err := ScanFromDir(root)
	if err != nil {
		t.Fatalf("ScanFromDir returned error: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected only valid skill to load, got %d", len(metas))
	}
	if metas[0].Name != "reviewer" {
		t.Fatalf("expected reviewer skill, got %q", metas[0].Name)
	}
}

func TestScanRepositoryCreatesRootAndLoadsSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	root, err := EnsureRepositoryRoot()
	if err != nil {
		t.Fatalf("EnsureRepositoryRoot returned error: %v", err)
	}

	skillDir := filepath.Join(root, "triage")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: triage
description: Sort incoming work quickly
---
Look at urgency first.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	metas, scanRoot, err := ScanRepository()
	if err != nil {
		t.Fatalf("ScanRepository returned error: %v", err)
	}
	if scanRoot != root {
		t.Fatalf("expected root %q, got %q", root, scanRoot)
	}
	if len(metas) != 1 || metas[0].Name != "triage" {
		t.Fatalf("expected triage skill, got %#v", metas)
	}
}
