package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// CoworkWithAgentTool establishes a cowork relationship with another agent
// by creating a persistent symlink in the target's workspace pointing at a
// file or directory in this agent's workspace. The receiver can read and
// write through the link; changes land on the sender's disk.
type CoworkWithAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewCoworkWithAgentTool(directory tools.AgentDirectory, selfID string) *CoworkWithAgentTool {
	return &CoworkWithAgentTool{directory: directory, selfID: selfID}
}

func (t *CoworkWithAgentTool) Name() string { return "cowork_with_agent" }

func (t *CoworkWithAgentTool) PermissionKey() tools.PermissionKey { return tools.PermissionCowork }

func (t *CoworkWithAgentTool) Description() string {
	return `Invite another agent to cowork on one of your files or directories — like sharing a Google Doc. The cowork link appears in the receiver's workspace at cowork/<your-agent-id>/<basename>; their edits land directly on YOUR disk in real time, with no copy step.
This is the right tool when the user says share / 共享 / 分享 / cowork / 协作 / 协同 / 一起改 / 让他改 / 让对方编辑 — anything that implies the other agent will be editing your files and you want those edits to land back on your disk. The ONLY time you should pick send_message_to_agent over this is when the user is explicit about wanting a frozen copy / snapshot ('发一份', 'send a copy', 'send for reference').
cowork_with_agent only sets up the link — it does not deliver a message. If the receiver also needs context ("please refactor this", "review and propose changes"), follow up with send_message_to_agent (message-only, no files needed).
Use list_coworks to see what coworks you have already started, end_cowork_with_agent to revoke access.`
}

func (t *CoworkWithAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from list_agents. Not your own agent ID."
			},
			"source_path": {
				"type": "string",
				"description": "Path inside your workspace (file or directory) to invite the receiver to cowork on. The receiver will see it under cowork/<your-agent-id>/<basename>."
			}
		},
		"required": ["target_agent_id", "source_path"]
	}`)
}

type coworkWithAgentInput struct {
	TargetAgentID string `json:"target_agent_id"`
	SourcePath    string `json:"source_path"`
}

func (t *CoworkWithAgentTool) Summary(raw json.RawMessage) string {
	var in coworkWithAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	return fmt.Sprintf("%s → %s", in.SourcePath, in.TargetAgentID)
}

func (t *CoworkWithAgentTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var in coworkWithAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid cowork_with_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("cowork_with_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.SourcePath) == "" {
		return "", fmt.Errorf("cowork_with_agent: source_path is required")
	}

	linkPath, err := t.directory.CreateCowork(ctx, t.selfID, in.TargetAgentID, in.SourcePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Invited %s to cowork on %q at %s (read/write).", in.TargetAgentID, in.SourcePath, linkPath), nil
}
