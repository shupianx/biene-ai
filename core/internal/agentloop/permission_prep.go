package agentloop

import (
	"context"
	"encoding/json"

	"tinte/internal/tools"
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
	toolID string,
	input json.RawMessage,
) *preparedPermission {
	prep := &preparedPermission{done: make(chan struct{})}
	tagged := tools.WithToolID(ctx, toolID)
	go func() {
		prep.allowed, prep.resolution, prep.err = checker.Check(tagged, tool, input)
		close(prep.done)
	}()
	return prep
}

func (p *preparedPermission) Wait() (bool, json.RawMessage, error) {
	<-p.done
	return p.allowed, p.resolution, p.err
}
