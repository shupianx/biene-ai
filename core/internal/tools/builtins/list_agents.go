package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biene/internal/tools"
)

// ListAgentsTool shows the current agent plus any other agents that can receive messages or files.
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
	return `List the current agent and any other available agent instances.
Use this to confirm your own agent ID/name and to find another agent's ID and current status before send_to_agent.`
}

func (t *ListAgentsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *ListAgentsTool) Summary(_ json.RawMessage) string { return "list available agents" }

func (t *ListAgentsTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	agents := t.directory.ListAgents("")

	var self *tools.AgentPeer
	others := make([]tools.AgentPeer, 0, len(agents))
	for _, agent := range agents {
		if agent.ID == t.selfID {
			agentCopy := agent
			self = &agentCopy
			continue
		}
		others = append(others, agent)
	}

	var sb strings.Builder
	if self != nil {
		sb.WriteString("Current agent:\n")
		fmt.Fprintf(&sb, "- %s | %s | %s | %s\n", self.ID, self.Name, self.Status, self.WorkDir)
	}

	if len(others) == 0 {
		if sb.Len() == 0 {
			return "No agents are available.", nil
		}
		sb.WriteString("\nNo other agents are available.")
		return strings.TrimRight(sb.String(), "\n"), nil
	}

	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString("Other available agents:\n")
	for _, peer := range others {
		fmt.Fprintf(&sb, "- %s | %s | %s | %s\n", peer.ID, peer.Name, peer.Status, peer.WorkDir)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
