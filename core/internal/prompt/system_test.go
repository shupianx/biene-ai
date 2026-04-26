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

func TestBuildDoesNotReferenceStaleToolName(t *testing.T) {
	// send_to_agent was renamed to send_message_to_agent. Catch any future
	// catalog / Base prompt drift that reintroduces the old name. Run with
	// every domain so a single domain regressing also fails.
	domains := []Domain{"general", "coding"}
	workDir := t.TempDir()
	for _, d := range domains {
		profile := AgentProfile{Domain: d, Style: "balanced"}
		got := Build(tools.NewRegistry(), workDir, profile, AgentIdentity{
			ID:      "sess_test",
			Name:    "Test",
			WorkDir: workDir,
		}, nil)
		if strings.Contains(got, "send_to_agent") && !strings.Contains(got, "send_message_to_agent") {
			t.Fatalf("domain=%s: stale tool name 'send_to_agent' present without the new name; output:\n%s", d, got)
		}
		// Stricter check: the bare "send_to_agent" token (not as part of
		// send_message_to_agent) must never appear.
		idx := 0
		for {
			at := strings.Index(got[idx:], "send_to_agent")
			if at < 0 {
				break
			}
			absolute := idx + at
			// If it's part of "send_message_to_agent", skip past it.
			if absolute >= len("send_message_") &&
				strings.HasPrefix(got[absolute-len("send_message_"):], "send_message_to_agent") {
				idx = absolute + len("send_to_agent")
				continue
			}
			t.Fatalf("domain=%s: stale tool name 'send_to_agent' found at offset %d:\n%s", d, absolute, got)
		}
	}
}

func TestDomainRulesAreUnique(t *testing.T) {
	// Domain rules must be domain-specific. Anything that appears verbatim
	// in two domains is a sign of a rule that belongs in Base instead —
	// keep it from sneaking back in via copy-paste.
	catalog := CurrentCatalog()
	seen := map[string]Domain{}
	for _, d := range catalog.Domains {
		for _, rule := range d.Rules {
			if other, dup := seen[rule]; dup {
				t.Fatalf("rule duplicated across domains %q and %q: %q", other, d.Value, rule)
			}
			seen[rule] = d.Value
		}
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
