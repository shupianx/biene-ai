package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AgentMessageMeta is the per-message metadata for inter-agent delivery.
type AgentMessageMeta struct {
	ThreadID      string `json:"thread_id"`
	MessageID     string `json:"message_id"`
	InReplyTo     string `json:"in_reply_to,omitempty"`
	RequiresReply bool   `json:"requires_reply,omitempty"`
	ReplySent     bool   `json:"reply_sent,omitempty"`
}

// AgentPeer describes another available agent instance.
type AgentPeer struct {
	ID      string
	Name    string
	WorkDir string
	Status  string
}

// DeliveryRequest is one outbound agent-to-agent message.
type DeliveryRequest struct {
	TargetAgentID string
	Message       string
	FilePaths     []string
	RequiresReply bool
}

// DeliveryResult captures the outcome of an inter-agent transfer.
type DeliveryResult struct {
	TargetID    string
	TargetName  string
	StoredPaths []string
	MessageMeta AgentMessageMeta
	IsReply     bool
}

// AgentDirectory exposes the session manager features needed by agent tools.
type AgentDirectory interface {
	ListAgents(excludeID string) []AgentPeer
	DeliverFromAgent(ctx context.Context, fromAgentID string, req DeliveryRequest) (DeliveryResult, error)
}

// ListAgentsTool shows other agents that can receive messages or files.
type ListAgentsTool struct {
	directory AgentDirectory
	selfID    string
}

func NewListAgentsTool(directory AgentDirectory, selfID string) *ListAgentsTool {
	return &ListAgentsTool{directory: directory, selfID: selfID}
}

func (t *ListAgentsTool) Name() string { return "ListAgents" }

func (t *ListAgentsTool) PermissionKey() PermissionKey { return PermissionNone }

func (t *ListAgentsTool) Description() string {
	return `List the other available agent instances.
Use this before SendToAgent so you know the target agent ID and current status.`
}

func (t *ListAgentsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *ListAgentsTool) IsReadOnly() bool { return true }

func (t *ListAgentsTool) Summary(_ json.RawMessage) string { return "list available agents" }

func (t *ListAgentsTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	peers := t.directory.ListAgents(t.selfID)
	if len(peers) == 0 {
		return "No other agents are available.", nil
	}

	var sb strings.Builder
	sb.WriteString("Available agents:\n")
	for _, peer := range peers {
		fmt.Fprintf(&sb, "- %s | %s | %s | %s\n", peer.ID, peer.Name, peer.Status, peer.WorkDir)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// SendToAgentTool delivers a message and optional files to another agent.
type SendToAgentTool struct {
	directory AgentDirectory
	selfID    string
}

func NewSendToAgentTool(directory AgentDirectory, selfID string) *SendToAgentTool {
	return &SendToAgentTool{directory: directory, selfID: selfID}
}

func (t *SendToAgentTool) Name() string { return "SendToAgent" }

func (t *SendToAgentTool) PermissionKey() PermissionKey { return PermissionSendToAgent }

func (t *SendToAgentTool) Description() string {
	return `Send a message and optional files to another agent instance.
Files must be paths inside your own workspace. The receiving agent gets the files in its inbox and sees your message in its chat.
Request a direct response only when you actually need the target agent to reply.`
}

func (t *SendToAgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_agent_id": {
				"type": "string",
				"description": "The destination agent ID from ListAgents"
			},
			"message": {
				"type": "string",
				"description": "Message text to deliver"
			},
			"file_paths": {
				"type": "array",
				"description": "Optional file paths from your workspace to send",
				"items": { "type": "string" }
			},
			"requires_reply": {
				"type": "boolean",
				"description": "Request a direct reply from the target agent when needed"
			}
		},
		"required": ["target_agent_id"]
	}`)
}

func (t *SendToAgentTool) IsReadOnly() bool { return false }

type sendToAgentInput struct {
	TargetAgentID string   `json:"target_agent_id"`
	Message       string   `json:"message"`
	FilePaths     []string `json:"file_paths"`
	RequiresReply bool     `json:"requires_reply"`
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
	if in.RequiresReply {
		parts = append(parts, "(reply requested)")
	}
	return strings.Join(parts, ": ")
}

func (t *SendToAgentTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var in sendToAgentInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid SendToAgent input: %w", err)
	}
	if strings.TrimSpace(in.TargetAgentID) == "" {
		return "", fmt.Errorf("SendToAgent: target_agent_id is required")
	}
	if strings.TrimSpace(in.Message) == "" && len(in.FilePaths) == 0 {
		return "", fmt.Errorf("SendToAgent: provide message and/or file_paths")
	}

	result, err := t.directory.DeliverFromAgent(ctx, t.selfID, DeliveryRequest{
		TargetAgentID: in.TargetAgentID,
		Message:       in.Message,
		FilePaths:     in.FilePaths,
		RequiresReply: in.RequiresReply,
	})
	if err != nil {
		return "", err
	}

	kind := "message"
	switch {
	case result.IsReply:
		kind = "reply"
	case result.MessageMeta.RequiresReply:
		kind = "request"
	}

	if len(result.StoredPaths) == 0 {
		return fmt.Sprintf("Delivered %s to %s (%s).", kind, result.TargetName, result.TargetID), nil
	}
	return fmt.Sprintf(
		"Delivered %s and %d file(s) to %s (%s): %s",
		kind,
		len(result.StoredPaths),
		result.TargetName,
		result.TargetID,
		strings.Join(result.StoredPaths, ", "),
	), nil
}
