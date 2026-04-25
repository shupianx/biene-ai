package builtins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// UseSkillTool loads the full instructions for one installed skill and marks
// it active on the current session. The returned tool result is the skill
// body, so the model gets the detailed guidance in context for the rest of
// the conversation.
type UseSkillTool struct {
	activator tools.SkillActivator
}

func NewUseSkillTool(activator tools.SkillActivator) *UseSkillTool {
	return &UseSkillTool{activator: activator}
}

func (t *UseSkillTool) Name() string { return "use_skill" }

func (t *UseSkillTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *UseSkillTool) Description() string {
	return `Load the full instructions for one installed skill.
Call this when an installed skill looks relevant to the current request. Pass the skill's exact name from the Installed Skills list. The returned text is the skill's complete guidance; follow it for the rest of the task. Calling this once is enough — the instructions remain available in the conversation for follow-up turns.`
}

func (t *UseSkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "Exact skill name from the Installed Skills list."
			}
		},
		"required": ["name"]
	}`)
}

type useSkillInput struct {
	Name string `json:"name"`
}

func (t *UseSkillTool) Summary(raw json.RawMessage) string {
	var in useSkillInput
	_ = json.Unmarshal(raw, &in)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "load skill"
	}
	return fmt.Sprintf("load skill %s", name)
}

func (t *UseSkillTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in useSkillInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("use_skill: invalid input: %w", err)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("use_skill: name is required")
	}
	if t.activator == nil {
		return "", errors.New("use_skill: no activator registered")
	}
	instructions, err := t.activator.ActivateSkill(name)
	if err != nil {
		return "", err
	}
	return instructions, nil
}
