package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"biene/internal/tools"
)

// ListCoworksTool reports the cowork relationships this agent has granted
// to other agents.
type ListCoworksTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewListCoworksTool(directory tools.AgentDirectory, selfID string) *ListCoworksTool {
	return &ListCoworksTool{directory: directory, selfID: selfID}
}

func (t *ListCoworksTool) Name() string { return "list_coworks" }

func (t *ListCoworksTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ListCoworksTool) Description() string {
	return `List the files and directories you have invited other agents to cowork on via cowork_with_agent.
Call this when you want to remember what is currently exposed, or before calling end_cowork_with_agent.`
}

func (t *ListCoworksTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *ListCoworksTool) Summary(_ json.RawMessage) string { return "list active coworks" }

func (t *ListCoworksTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	entries := t.directory.ListCoworks(t.selfID)
	if len(entries) == 0 {
		return "You have not invited any agent to cowork yet.", nil
	}

	// Group by target for a compact, scannable output.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TargetAgentID != entries[j].TargetAgentID {
			return entries[i].TargetAgentID < entries[j].TargetAgentID
		}
		return entries[i].SourcePath < entries[j].SourcePath
	})

	var sb strings.Builder
	var currentTarget string
	for _, e := range entries {
		if e.TargetAgentID != currentTarget {
			currentTarget = e.TargetAgentID
			label := e.TargetAgentName
			if label == "" {
				label = e.TargetAgentID
			} else {
				label = fmt.Sprintf("%s (%s)", label, e.TargetAgentID)
			}
			fmt.Fprintf(&sb, "Coworking with %s:\n", label)
		}
		fmt.Fprintf(&sb, "  - %s (since %s)\n", e.SourcePath, e.CreatedAt.Format("2006-01-02 15:04"))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
