package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// ShareToAgentTool creates a persistent symlink in another agent's
// workspace pointing at a file or directory inside this agent's
// workspace. The receiver can read and write through the link; changes
// land on the sender's disk.
type ShareToAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewShareToAgentTool(directory tools.AgentDirectory, selfID string) *ShareToAgentTool {
	return &ShareToAgentTool{directory: directory, selfID: selfID}
}

func (t *ShareToAgentTool) Name() string { return "share_to_agent" }

func (t *ShareToAgentTool) PermissionKey() tools.PermissionKey { return tools.PermissionSendToAgent }

func (t *ShareToAgentTool) Description() string {
	return `Grant another agent read/write access to one of your files or directories via a workspace symlink.
The share appears in the target agent's workspace at shared/<your-agent-id>/<basename>. The receiver can read, edit, and create files under that path; every write lands on your disk immediately.
Use this for multi-file collaborative work (e.g. asking another agent to modify a project) where copying the files back and forth would be impractical.
Prefer send_to_agent for one-off file handoffs that do not need bidirectional editing.
Call list_shares to see what you have already shared, and unshare_to_agent to revoke access.`
}

func (t *ShareToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from list_agents. Not your own agent ID."
			},
			"source_path": {
				"type": "string",
				"description": "Path inside your workspace (file or directory) to share. The receiver will see it under shared/<your-agent-id>/<basename>."
			}
		},
		"required": ["target_agent_id", "source_path"]
	}`)
}

type shareToAgentInput struct {
	TargetAgentID string `json:"target_agent_id"`
	SourcePath    string `json:"source_path"`
}

func (t *ShareToAgentTool) Summary(raw json.RawMessage) string {
	var in shareToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	return fmt.Sprintf("%s → %s", in.SourcePath, in.TargetAgentID)
}

func (t *ShareToAgentTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var in shareToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid share_to_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("share_to_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.SourcePath) == "" {
		return "", fmt.Errorf("share_to_agent: source_path is required")
	}

	linkPath, err := t.directory.CreateShare(ctx, t.selfID, in.TargetAgentID, in.SourcePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Shared %q with %s at %s (read/write).", in.SourcePath, in.TargetAgentID, linkPath), nil
}
