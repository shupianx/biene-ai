package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// EndCoworkWithAgentTool revokes a cowork relationship previously
// established via cowork_with_agent.
type EndCoworkWithAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewEndCoworkWithAgentTool(directory tools.AgentDirectory, selfID string) *EndCoworkWithAgentTool {
	return &EndCoworkWithAgentTool{directory: directory, selfID: selfID}
}

func (t *EndCoworkWithAgentTool) Name() string { return "end_cowork_with_agent" }

func (t *EndCoworkWithAgentTool) PermissionKey() tools.PermissionKey {
	return tools.PermissionCowork
}

func (t *EndCoworkWithAgentTool) Description() string {
	return `End a cowork relationship previously established with another agent via cowork_with_agent.
The receiver's symlink is removed; the underlying files on your disk are not touched.
Use list_coworks first if you do not remember exactly what you have invited the agent to cowork on.`
}

func (t *EndCoworkWithAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The agent the cowork link was created with."
			},
			"source_path": {
				"type": "string",
				"description": "The path inside your workspace that was offered for cowork (must match exactly what was passed to cowork_with_agent)."
			}
		},
		"required": ["target_agent_id", "source_path"]
	}`)
}

type endCoworkWithAgentInput struct {
	TargetAgentID string `json:"target_agent_id"`
	SourcePath    string `json:"source_path"`
}

func (t *EndCoworkWithAgentTool) Summary(raw json.RawMessage) string {
	var in endCoworkWithAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	return fmt.Sprintf("%s ↛ %s", in.SourcePath, in.TargetAgentID)
}

func (t *EndCoworkWithAgentTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in endCoworkWithAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid end_cowork_with_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("end_cowork_with_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.SourcePath) == "" {
		return "", fmt.Errorf("end_cowork_with_agent: source_path is required")
	}

	if err := t.directory.EndCowork(t.selfID, in.TargetAgentID, in.SourcePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("Ended cowork of %q with %s.", in.SourcePath, in.TargetAgentID), nil
}
