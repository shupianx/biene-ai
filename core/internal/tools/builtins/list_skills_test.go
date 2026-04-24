package builtins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSkillsToolListsInstalledSkills(t *testing.T) {
	workDir := t.TempDir()

	skillA := filepath.Join(workDir, ".tinte", "skills", "reviewer")
	skillB := filepath.Join(workDir, ".tinte", "skills", "release-notes")
	if err := os.MkdirAll(skillA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skillB, 0o755); err != nil {
		t.Fatal(err)
	}

	reviewer := `---
name: reviewer
description: Review changes carefully
---
# Reviewer
`
	releaseNotes := `---
name: release-notes
description: Draft release notes from recent changes
---
# Release Notes
`

	if err := os.WriteFile(filepath.Join(skillA, "SKILL.md"), []byte(reviewer), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillB, "SKILL.md"), []byte(releaseNotes), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewListSkillsToolInDir(workDir)
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !strings.Contains(out, "Installed skills:") {
		t.Fatalf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "- reviewer: Review changes carefully") {
		t.Fatalf("expected reviewer entry, got:\n%s", out)
	}
	if !strings.Contains(out, "- release-notes: Draft release notes from recent changes") {
		t.Fatalf("expected release-notes entry, got:\n%s", out)
	}
}

func TestListSkillsToolHandlesEmptyWorkspace(t *testing.T) {
	tool := NewListSkillsToolInDir(t.TempDir())
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if out != "No skills are installed in this agent workspace." {
		t.Fatalf("unexpected output: %q", out)
	}
}
