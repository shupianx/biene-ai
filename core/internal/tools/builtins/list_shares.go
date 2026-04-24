package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"tinte/internal/tools"
)

// ListSharesTool reports the shares this agent has granted to other agents.
type ListSharesTool struct {
	directory tools.AgentDirectory
	selfID    string
}

func NewListSharesTool(directory tools.AgentDirectory, selfID string) *ListSharesTool {
	return &ListSharesTool{directory: directory, selfID: selfID}
}

func (t *ListSharesTool) Name() string { return "list_shares" }

func (t *ListSharesTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ListSharesTool) Description() string {
	return `List the files and directories you have shared with other agents via share_to_agent.
Call this when you want to remember what is currently exposed, or before calling unshare_to_agent.`
}

func (t *ListSharesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *ListSharesTool) Summary(_ json.RawMessage) string { return "list shared items" }

func (t *ListSharesTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	entries := t.directory.ListShares(t.selfID)
	if len(entries) == 0 {
		return "You have not shared anything with other agents.", nil
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
			fmt.Fprintf(&sb, "Shared with %s:\n", label)
		}
		fmt.Fprintf(&sb, "  - %s (granted %s)\n", e.SourcePath, e.CreatedAt.Format("2006-01-02 15:04"))
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
