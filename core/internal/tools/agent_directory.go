package tools

import (
	"context"
)

// AgentMessageMeta is the per-message metadata for inter-agent delivery.
type AgentMessageMeta struct {
	ThreadID  string `json:"thread_id"`
	MessageID string `json:"message_id"`
	InReplyTo string `json:"in_reply_to,omitempty"`
}

// AgentPeer describes an available agent instance.
type AgentPeer struct {
	ID      string
	Name    string
	WorkDir string
	Status  string
}

// CollisionResolution controls how a sender's file is written into the
// receiver's inbox when a file with the same name already exists.
type CollisionResolution string

const (
	CollisionRename    CollisionResolution = "rename"
	CollisionOverwrite CollisionResolution = "overwrite"
	CollisionSkip      CollisionResolution = "skip"
)

// FileCollision describes one name conflict discovered before delivery.
type FileCollision struct {
	RequestedPath string `json:"requested_path"`
	TargetPath    string `json:"target_path"`
}

// DeliveryRequest is one outbound agent-to-agent message.
type DeliveryRequest struct {
	TargetAgentID      string
	Message            string
	FilePaths          []string
	CollisionStrategy  CollisionResolution
}

// DeliveryResult captures the outcome of an inter-agent transfer.
type DeliveryResult struct {
	TargetID    string
	TargetName  string
	StoredPaths []string
	Skipped     []string
	Overwritten []string
	Renamed     []string
	MessageMeta AgentMessageMeta
}

// AgentDirectory exposes the session manager features needed by inter-agent tools.
type AgentDirectory interface {
	ListAgents(excludeID string) []AgentPeer
	DeliverFromAgent(ctx context.Context, fromAgentID string, req DeliveryRequest) (DeliveryResult, error)
	DetectFileCollisions(fromAgentID, targetAgentID string, filePaths []string) ([]FileCollision, error)
}
