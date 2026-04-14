package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// SendToAgentTool delivers a message and optional files to another agent.
type SendToAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewSendToAgentTool(directory tools.AgentDirectory, selfID string) *SendToAgentTool {
	return &SendToAgentTool{directory: directory, selfID: selfID}
}

func (t *SendToAgentTool) Name() string { return "send_to_agent" }

func (t *SendToAgentTool) PermissionKey() tools.PermissionKey { return tools.PermissionSendToAgent }

func (t *SendToAgentTool) Description() string {
	return `Send a message and optional files to another agent instance.
Files must be paths inside your own workspace. The receiving agent gets the files in its inbox and sees your message in its chat.
Use this when the user clearly wants agent collaboration, file handoff, or delegation, or when you are sending results back to another agent.`
}

func (t *SendToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from list_agents"
			},
			"message": {
				"type": "string",
				"description": "Message text to deliver"
			},
			"file_paths": {
				"type": "array",
				"description": "Optional file paths from your workspace to send",
				"items": { "type": "string" }
			}
		},
		"required": ["target_agent_id"]
	}`)
}

type sendToAgentInput struct {
	TargetAgentID string   `json:"target_agent_id"`
	Message       string   `json:"message"`
	FilePaths     []string `json:"file_paths"`
}

func (t *SendToAgentTool) Summary(raw json.RawMessage) string {
	var in sendToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	parts := []string{in.TargetAgentID}
	if msg := strings.TrimSpace(in.Message); msg != "" {
		parts = append(parts, msg)
	}
	return strings.Join(parts, ": ")
}

func (t *SendToAgentTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var in sendToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid send_to_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("send_to_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.Message) == "" && len(in.FilePaths) == 0 {
		return "", fmt.Errorf("send_to_agent: provide message and/or file_paths")
	}

	result, err := t.directory.DeliverFromAgent(ctx, t.selfID, tools.DeliveryRequest{
		TargetAgentID: in.TargetAgentID,
		Message:       in.Message,
		FilePaths:     in.FilePaths,
	})
	if err != nil {
		return "", err
	}

	if len(result.StoredPaths) == 0 {
		return fmt.Sprintf("Delivered message to %s (%s).", result.TargetName, result.TargetID), nil
	}
	return fmt.Sprintf(
		"Delivered message and %d file(s) to %s (%s): %s",
		len(result.StoredPaths),
		result.TargetName,
		result.TargetID,
		strings.Join(result.StoredPaths, ", "),
	), nil
}
