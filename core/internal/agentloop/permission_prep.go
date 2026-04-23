package agentloop

import (
	"context"
	"encoding/json"

	"biene/internal/tools"
)

type preparedPermission struct {
	done       chan struct{}
	allowed    bool
	resolution json.RawMessage
	err        error
}

func startPreparedPermission(
	ctx context.Context,
	checker PermissionChecker,
	tool tools.Tool,
	input json.RawMessage,
) *preparedPermission {
	prep := &preparedPermission{done: make(chan struct{})}
	go func() {
		prep.allowed, prep.resolution, prep.err = checker.Check(ctx, tool, input)
		close(prep.done)
	}()
	return prep
}

func (p *preparedPermission) Wait() (bool, json.RawMessage, error) {
	<-p.done
	return p.allowed, p.resolution, p.err
}
