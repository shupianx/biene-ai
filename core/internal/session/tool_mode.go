package session

import (
	"tinte/internal/prompt"
	"tinte/internal/tools"
)

func registryForToolMode(registry *tools.Registry, mode ToolMode) (*tools.Registry, bool) {
	return registry, false
}

func defaultToolModeForProfile(_ prompt.AgentProfile) ToolMode {
	return ToolModeWorkspaceChange
}
