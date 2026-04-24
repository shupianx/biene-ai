package session

import (
	"context"
	"encoding/json"
	"testing"

	"tinte/internal/prompt"
	"tinte/internal/tools"
)

type stubTool struct {
	name string
}

func (t stubTool) Name() string                                             { return t.name }
func (t stubTool) PermissionKey() tools.PermissionKey                       { return tools.PermissionNone }
func (t stubTool) Description() string                                      { return t.name }
func (t stubTool) InputSchema() json.RawMessage                             { return json.RawMessage(`{"type":"object"}`) }
func (t stubTool) Execute(context.Context, json.RawMessage) (string, error) { return "", nil }
func (t stubTool) Summary(json.RawMessage) string                           { return t.name }

func TestRegistryForToolModeRestrictsAnswerOnlyTools(t *testing.T) {
	registry := tools.NewRegistry()
	for _, name := range []string{"list_skills", "list_files", "read_file", "write_file", "edit_file", "run_command", "list_agents", "send_to_agent"} {
		registry.Register(stubTool{name: name})
	}

	filtered, changed := registryForToolMode(registry, ToolModeAnswerOnly)
	if changed {
		t.Fatal("expected registry to remain unchanged")
	}
	for _, name := range []string{"list_skills", "list_files", "read_file", "write_file", "edit_file", "run_command", "list_agents", "send_to_agent"} {
		if filtered.Find(name) == nil {
			t.Fatalf("expected %s to remain available", name)
		}
	}
}

func TestDefaultToolModeForProfileUsesWorkspaceChangeForGeneral(t *testing.T) {
	mode := defaultToolModeForProfile(prompt.AgentProfile{Domain: "general"})
	if mode != ToolModeWorkspaceChange {
		t.Fatalf("expected workspace_change for general profile, got %q", mode)
	}
}

func TestDefaultToolModeForProfileUsesWorkspaceChangeForCoding(t *testing.T) {
	mode := defaultToolModeForProfile(prompt.AgentProfile{Domain: "coding"})
	if mode != ToolModeWorkspaceChange {
		t.Fatalf("expected workspace_change for coding profile, got %q", mode)
	}
}

func TestNormalizeToolModeFallsBackToWorkspaceChange(t *testing.T) {
	mode := normalizeToolMode("unexpected")
	if mode != ToolModeWorkspaceChange {
		t.Fatalf("expected workspace_change fallback, got %q", mode)
	}
}
