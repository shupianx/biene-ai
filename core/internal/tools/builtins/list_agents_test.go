package builtins

import (
	"context"
	"strings"
	"testing"

	"tinte/internal/tools"
)

type listAgentsTestDirectory struct {
	agents        []tools.AgentPeer
	lastExcludeID string
}

func (d *listAgentsTestDirectory) ListAgents(excludeID string) []tools.AgentPeer {
	d.lastExcludeID = excludeID
	if excludeID == "" {
		out := make([]tools.AgentPeer, len(d.agents))
		copy(out, d.agents)
		return out
	}

	out := make([]tools.AgentPeer, 0, len(d.agents))
	for _, agent := range d.agents {
		if agent.ID != excludeID {
			out = append(out, agent)
		}
	}
	return out
}

func (d *listAgentsTestDirectory) DeliverFromAgent(context.Context, string, tools.DeliveryRequest) (tools.DeliveryResult, error) {
	return tools.DeliveryResult{}, nil
}

func (d *listAgentsTestDirectory) DetectFileCollisions(string, string, []string) ([]tools.FileCollision, error) {
	return nil, nil
}

func (d *listAgentsTestDirectory) CreateShare(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (d *listAgentsTestDirectory) RemoveShare(string, string, string) error { return nil }

func (d *listAgentsTestDirectory) ListShares(string) []tools.SharedEntry { return nil }

func TestListAgentsToolIncludesCurrentAgentAndPeers(t *testing.T) {
	directory := &listAgentsTestDirectory{
		agents: []tools.AgentPeer{
			{ID: "sess_self", Name: "Planner", Status: "idle", WorkDir: "/tmp/self"},
			{ID: "sess_peer", Name: "Reviewer", Status: "running", WorkDir: "/tmp/peer"},
		},
	}

	tool := NewListAgentsTool(directory, "sess_self")
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if directory.lastExcludeID != "" {
		t.Fatalf("expected list_agents to request all agents, got excludeID=%q", directory.lastExcludeID)
	}
	if !strings.Contains(out, "Current agent:") {
		t.Fatalf("expected current agent section, got:\n%s", out)
	}
	if !strings.Contains(out, "- sess_self | Planner | idle | /tmp/self") {
		t.Fatalf("expected self entry, got:\n%s", out)
	}
	if !strings.Contains(out, "Other available agents:") {
		t.Fatalf("expected peers section, got:\n%s", out)
	}
	if !strings.Contains(out, "- sess_peer | Reviewer | running | /tmp/peer") {
		t.Fatalf("expected peer entry, got:\n%s", out)
	}
}

func TestListAgentsToolHandlesOnlyCurrentAgent(t *testing.T) {
	directory := &listAgentsTestDirectory{
		agents: []tools.AgentPeer{
			{ID: "sess_self", Name: "Planner", Status: "idle", WorkDir: "/tmp/self"},
		},
	}

	tool := NewListAgentsTool(directory, "sess_self")
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !strings.Contains(out, "Current agent:") {
		t.Fatalf("expected current agent section, got:\n%s", out)
	}
	if !strings.Contains(out, "No other agents are available.") {
		t.Fatalf("expected no-peers message, got:\n%s", out)
	}
}
