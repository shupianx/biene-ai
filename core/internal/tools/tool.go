package tools

import (
	"context"
	"encoding/json"

	"biene/internal/api"
)

type PermissionKey string

const (
	PermissionNone        PermissionKey = ""
	PermissionWrite       PermissionKey = "write"
	PermissionSendToAgent PermissionKey = "send_to_agent"
)

type PermissionSet struct {
	Write       bool `json:"write"`
	SendToAgent bool `json:"send_to_agent"`
}

func (p PermissionSet) Allows(key PermissionKey) bool {
	switch key {
	case PermissionWrite:
		return p.Write
	case PermissionSendToAgent:
		return p.SendToAgent
	default:
		return true
	}
}

func (p PermissionSet) With(key PermissionKey, allowed bool) PermissionSet {
	switch key {
	case PermissionWrite:
		p.Write = allowed
	case PermissionSendToAgent:
		p.SendToAgent = allowed
	}
	return p
}

// Tool defines the interface every built-in tool must implement.
type Tool interface {
	// Name returns the tool's identifier used in API calls.
	Name() string

	// PermissionKey returns the approval group needed for this tool.
	// Read-only tools should return PermissionNone.
	PermissionKey() PermissionKey

	// Description is shown to the model in the system prompt.
	Description() string

	// InputSchema returns the JSON Schema for the tool's input object.
	InputSchema() json.RawMessage

	// IsReadOnly returns true when the tool never modifies the filesystem or
	// runs processes.  Read-only tools skip the interactive permission prompt.
	IsReadOnly() bool

	// Execute runs the tool with the given JSON input and returns a text result.
	Execute(ctx context.Context, input json.RawMessage) (string, error)

	// Summary returns a short human-readable description of this invocation,
	// shown in the permission confirmation prompt.
	Summary(input json.RawMessage) string
}

// Registry holds all registered tools and allows lookup by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Find returns the tool with the given name, or nil if not found.
func (r *Registry) Find(name string) Tool {
	return r.tools[name]
}

// All returns all registered tools as a slice (order is undefined).
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Definitions converts all registered tools to API tool definitions.
func (r *Registry) Definitions() []api.ToolDefinition {
	defs := make([]api.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, api.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// DefaultRegistry returns a registry pre-loaded with all built-in tools.
func DefaultRegistry() *Registry {
	return RegistryForWorkDir("")
}

// RegistryForWorkDir returns a registry where Bash commands execute inside workDir.
// Pass an empty string to use the process working directory.
func RegistryForWorkDir(workDir string) *Registry {
	r := NewRegistry()
	r.Register(NewFileReadToolInDir(workDir))
	r.Register(NewFileWriteToolInDir(workDir))
	r.Register(NewFileEditToolInDir(workDir))
	return r
}
