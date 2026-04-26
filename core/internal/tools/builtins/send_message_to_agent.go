package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"biene/internal/tools"
)

// SendMessageToAgentTool delivers a chat message — and, in rare cases, a
// snapshot of files — from this agent to another agent.
type SendMessageToAgentTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewSendMessageToAgentTool(directory tools.AgentDirectory, selfID string) *SendMessageToAgentTool {
	return &SendMessageToAgentTool{directory: directory, selfID: selfID}
}

func (t *SendMessageToAgentTool) Name() string { return "send_message_to_agent" }

func (t *SendMessageToAgentTool) PermissionKey() tools.PermissionKey {
	return tools.PermissionSendMessageToAgent
}

func (t *SendMessageToAgentTool) Description() string {
	return `Send a chat message to another agent — like sending a chat or an email. This is the primary channel for agent-to-agent communication: ask a question, give a status update, request work, return a result, follow up after a cowork invitation. The receiver sees the message in its chat and can reply back.

File attachments are a SECONDARY feature reserved for the rare case where the receiver legitimately needs a frozen snapshot they will not edit back. file_paths inside your workspace are copied to the receiver's inbox/<your-agent-id>/ as one-time copies; same-name conflicts prompt the user to overwrite, rename, or skip. The receiver's edits to attached files do NOT come back to your workspace.

DO NOT attach files when the user says share / 共享 / 分享 / 协作 / 让他改 / let them edit — those words mean the user wants edits to come back, which is cowork_with_agent's job. Only attach files when the user is explicit about a frozen copy ('发一份' / 'send a copy for reference' / 'archive a version'). When in doubt, send the message alone (no file_paths) and let the user clarify.`
}

func (t *SendMessageToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from the Other available agents section of list_agents, or extracted from an @[Name](agent:<ID>) mention in the user's message. Do not use your own current agent ID."
			},
			"message": {
				"type": "string",
				"description": "The chat message to deliver. This is the primary payload of the tool — fill it in even when also attaching files."
			},
			"file_paths": {
				"type": "array",
				"description": "Optional, rarely needed. File paths from your workspace to send as one-time snapshots (the receiver owns the copies; their edits do not sync back). Skip this field for ordinary messaging; only use it when the user explicitly wants a frozen copy delivered.",
				"items": { "type": "string" }
			}
		},
		"required": ["target_agent_id", "message"]
	}`)
}

type sendMessageToAgentInput struct {
	TargetAgentID string   `json:"target_agent_id"`
	Message       string   `json:"message"`
	FilePaths     []string `json:"file_paths"`
}

func (t *SendMessageToAgentTool) Summary(raw json.RawMessage) string {
	var in sendMessageToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	parts := []string{in.TargetAgentID}
	if msg := strings.TrimSpace(in.Message); msg != "" {
		parts = append(parts, msg)
	}
	return strings.Join(parts, ": ")
}

// PermissionContext reports to the UI which inbox files on the receiver would
// collide with the ones about to be delivered, so the user can pick a strategy.
func (t *SendMessageToAgentTool) PermissionContext(_ context.Context, raw json.RawMessage) (any, error) {
	var in sendMessageToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, nil
	}
	if strings.TrimSpace(in.TargetAgentID) == "" || len(in.FilePaths) == 0 {
		return nil, nil
	}
	collisions, err := t.directory.DetectFileCollisions(t.selfID, in.TargetAgentID, in.FilePaths)
	if err != nil || len(collisions) == 0 {
		return nil, err
	}
	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].TargetPath < collisions[j].TargetPath
	})
	return map[string]any{"collisions": collisions}, nil
}

// resolutionPayload mirrors the shape sent by the UI when the user picks a
// collision strategy alongside their allow decision.
type resolutionPayload struct {
	Collision string `json:"collision,omitempty"`
}

func (t *SendMessageToAgentTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var in sendMessageToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid send_message_to_agent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("send_message_to_agent: target_agent_id is required")
	}
	if strings.TrimSpace(in.Message) == "" && len(in.FilePaths) == 0 {
		return "", fmt.Errorf("send_message_to_agent: provide message and/or file_paths")
	}

	var strategy tools.CollisionResolution
	if data := tools.PermissionResolutionFromContext(ctx); len(data) > 0 {
		var r resolutionPayload
		if err := json.Unmarshal(data, &r); err == nil {
			switch tools.CollisionResolution(r.Collision) {
			case tools.CollisionOverwrite, tools.CollisionRename, tools.CollisionSkip:
				strategy = tools.CollisionResolution(r.Collision)
			}
		}
	}

	result, err := t.directory.DeliverFromAgent(ctx, t.selfID, tools.DeliveryRequest{
		TargetAgentID:     in.TargetAgentID,
		Message:           in.Message,
		FilePaths:         in.FilePaths,
		CollisionStrategy: strategy,
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if len(result.StoredPaths) == 0 && len(result.Skipped) == 0 {
		fmt.Fprintf(&sb, "Delivered message to %s (%s).", result.TargetName, result.TargetID)
		return sb.String(), nil
	}
	fmt.Fprintf(&sb, "Delivered message and %d file(s) to %s (%s)", len(result.StoredPaths), result.TargetName, result.TargetID)
	if len(result.StoredPaths) > 0 {
		fmt.Fprintf(&sb, ": %s", strings.Join(result.StoredPaths, ", "))
	}
	sb.WriteString(".")
	if len(result.Renamed) > 0 {
		fmt.Fprintf(&sb, " Renamed to avoid conflicts: %s.", strings.Join(result.Renamed, ", "))
	}
	if len(result.Overwritten) > 0 {
		fmt.Fprintf(&sb, " Overwrote existing files: %s.", strings.Join(result.Overwritten, ", "))
	}
	if len(result.Skipped) > 0 {
		fmt.Fprintf(&sb, " Skipped existing files: %s.", strings.Join(result.Skipped, ", "))
	}
	return sb.String(), nil
}
