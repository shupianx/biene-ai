package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/skills"
	"biene/internal/tools"
)

// ListSkillsTool lists installed skill metadata for the current agent workspace.
type ListSkillsTool struct {
	WorkDir string
}

func NewListSkillsTool() *ListSkillsTool                { return &ListSkillsTool{} }
func NewListSkillsToolInDir(dir string) *ListSkillsTool { return &ListSkillsTool{WorkDir: dir} }

func (t *ListSkillsTool) Name() string { return "list_skills" }

func (t *ListSkillsTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ListSkillsTool) Description() string {
	return `List the installed skills available in this agent's own workspace.
Use this when the user asks what skills are installed, or when you need to inspect the available skill names and descriptions explicitly.`
}

func (t *ListSkillsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *ListSkillsTool) Summary(_ json.RawMessage) string { return "list installed skills" }

func (t *ListSkillsTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	metas, err := skills.ScanForWorkDir(t.WorkDir)
	if err != nil {
		return "", fmt.Errorf("list_skills: %w", err)
	}
	if len(metas) == 0 {
		return "No skills are installed in this agent workspace.", nil
	}

	var sb strings.Builder
	sb.WriteString("Installed skills:\n")
	for _, meta := range metas {
		fmt.Fprintf(&sb, "- %s: %s\n", meta.Name, meta.Description)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
