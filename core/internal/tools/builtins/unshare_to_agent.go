package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// UnshareToAgentTool revokes a share previously granted via share_to_agent.
type UnshareToAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewUnshareToAgentTool(directory tools.AgentDirectory, selfID string) *UnshareToAgentTool {
	return &UnshareToAgentTool{directory: directory, selfID: selfID}
}

func (t *UnshareToAgentTool) Name() string { return "unshare_to_agent" }

func (t *UnshareToAgentTool) PermissionKey() tools.PermissionKey {
	return tools.PermissionSendToAgent
}

func (t *UnshareToAgentTool) Description() string {
	return `Revoke a workspace share previously granted to another agent via share_to_agent.
The receiver's symlink is removed; the underlying files on your disk are not touched.
Use list_shares first if you do not remember exactly what you have shared.`
}

func (t *UnshareToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The agent the share was granted to."
			},
			"source_path": {
				"type": "string",
				"description": "The path inside your workspace that was shared (must match exactly what was passed to share_to_agent)."
			}
		},
		"required": ["target_agent_id", "source_path"]
	}`)
}

type unshareToAgentInput struct {
	TargetAgentID string `json:"target_agent_id"`
	SourcePath    string `json:"source_path"`
}

func (t *UnshareToAgentTool) Summary(raw json.RawMessage) string {
	var in unshareToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	return fmt.Sprintf("%s ↛ %s", in.SourcePath, in.TargetAgentID)
}

func (t *UnshareToAgentTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in unshareToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid unshare_to_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("unshare_to_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.SourcePath) == "" {
		return "", fmt.Errorf("unshare_to_agent: source_path is required")
	}

	if err := t.directory.RemoveShare(t.selfID, in.TargetAgentID, in.SourcePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("Revoked share of %q from %s.", in.SourcePath, in.TargetAgentID), nil
}
