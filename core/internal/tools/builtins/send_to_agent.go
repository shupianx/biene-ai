package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"tinte/internal/tools"
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
Files must be paths inside your own workspace. The receiving agent gets the files under inbox/<your-agent-id>/ and sees your message in its chat.
If a file with the same name already exists in the receiver's inbox, the user is prompted to pick overwrite, rename, or skip before delivery proceeds.
Use this when the user clearly wants agent collaboration, file handoff, or delegation, or when you are sending results back to another agent.`
}

func (t *SendToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from the Other available agents section of list_agents, or extracted from an @[Name](agent:<ID>) mention in the user's message. Do not use your own current agent ID."
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

// PermissionContext reports to the UI which inbox files on the receiver would
// collide with the ones about to be delivered, so the user can pick a strategy.
func (t *SendToAgentTool) PermissionContext(_ context.Context, raw json.RawMessage) (any, error) {
	var in sendToAgentInput
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
