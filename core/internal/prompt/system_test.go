package prompt

import (
	"path/filepath"
	"strings"
	"testing"

	"biene/internal/skills"
	"biene/internal/tools"
)

func TestBuildIncludesInstalledSkills(t *testing.T) {
	workDir := t.TempDir()
	installed := []skills.Metadata{{
		Name:        "reviewer",
		Description: "Review changes carefully",
		Dir:         filepath.Join(workDir, ".biene", "skills", "reviewer"),
		FilePath:    filepath.Join(workDir, ".biene", "skills", "reviewer", "SKILL.md"),
	}}

	promptText := Build(tools.NewRegistry(), workDir, DefaultProfile(), AgentIdentity{
		ID:      "sess_test",
		Name:    "Reviewer",
		WorkDir: workDir,
	}, installed)
	if !strings.Contains(promptText, "## Installed Skills") {
		t.Fatalf("expected installed skills section, got:\n%s", promptText)
	}
	if !strings.Contains(promptText, "**reviewer**: Review changes carefully") {
		t.Fatalf("expected reviewer summary, got:\n%s", promptText)
	}
}

func TestBuildOmitsInstalledSkillsWhenEmpty(t *testing.T) {
	workDir := t.TempDir()

	promptText := Build(tools.NewRegistry(), workDir, DefaultProfile(), AgentIdentity{
		ID:      "sess_test",
		Name:    "Reviewer",
		WorkDir: workDir,
	}, nil)
	if strings.Contains(promptText, "## Installed Skills") {
		t.Fatalf("did not expect installed skills section, got:\n%s", promptText)
	}
}

func TestBuildIncludesCurrentAgentIdentity(t *testing.T) {
	workDir := t.TempDir()

	promptText := Build(tools.NewRegistry(), workDir, DefaultProfile(), AgentIdentity{
		ID:      "sess_123",
		Name:    "Planner",
		WorkDir: workDir,
	}, nil)

	if !strings.Contains(promptText, "## Current Agent") {
		t.Fatalf("expected current agent section, got:\n%s", promptText)
	}
	if !strings.Contains(promptText, "Agent name: Planner") {
		t.Fatalf("expected agent name, got:\n%s", promptText)
	}
	if !strings.Contains(promptText, "Agent ID: sess_123") {
		t.Fatalf("expected agent ID, got:\n%s", promptText)
	}
	if !strings.Contains(promptText, "Agent workspace: "+workDir) {
		t.Fatalf("expected agent workspace, got:\n%s", promptText)
	}
}
