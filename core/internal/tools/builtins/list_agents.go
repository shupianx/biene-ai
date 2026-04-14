package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// ListAgentsTool shows other agents that can receive messages or files.
type ListAgentsTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewListAgentsTool(directory tools.AgentDirectory, selfID string) *ListAgentsTool {
	return &ListAgentsTool{directory: directory, selfID: selfID}
}

func (t *ListAgentsTool) Name() string { return "list_agents" }

func (t *ListAgentsTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ListAgentsTool) Description() string {
	return `List the other available agent instances.
Use this before send_to_agent so you know the target agent ID and current status.`
}

func (t *ListAgentsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

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
