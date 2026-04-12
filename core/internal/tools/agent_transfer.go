package tools

import (
	"context"
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
