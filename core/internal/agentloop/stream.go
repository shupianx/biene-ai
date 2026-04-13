package agentloop

import (
	"context"
	"errors"
	"strings"

	"biene/internal/api"
)

func collectStream(
	ctx context.Context,
	stream <-chan api.StreamEvent,
	ch chan<- Event,
	onToolUseStart func(api.ToolUseBlock),
) (api.Message, []api.ToolUseBlock, error) {
	var content []api.ContentBlock
	var text strings.Builder
	var toolUses []api.ToolUseBlock

done:
	for {
		select {
		case <-ctx.Done():
			return api.Message{}, nil, ctx.Err()
		case ev, ok := <-stream:
			if !ok {
				break done
			}
			switch ev.Type {
			case api.EventTextDelta:
				text.WriteString(ev.Text)
				ch <- Event{Kind: KindTextDelta, Text: ev.Text}
			case api.EventToolUseStart:
				if ev.ToolUse != nil && onToolUseStart != nil {
					onToolUseStart(*ev.ToolUse)
				}
			case api.EventToolUse:
				if text.Len() > 0 {
					content = append(content, api.TextBlock{Text: text.String()})
					text.Reset()
				}
				if ev.ToolUse != nil {
					content = append(content, *ev.ToolUse)
					toolUses = append(toolUses, *ev.ToolUse)
				}
			case api.EventDone:
				break done
			case api.EventError:
				if ev.Err != nil {
					return api.Message{}, nil, ev.Err
				}
				return api.Message{}, nil, errors.New("stream error")
			}
		}
	}

	if text.Len() > 0 {
		content = append(content, api.TextBlock{Text: text.String()})
	}

	return api.Message{
		Role:    api.RoleAssistant,
		Content: content,
	}, toolUses, nil
}

func earlyToolSummary(name string) string {
	switch name {
	case "Write":
		return "prepare file write"
	case "Edit":
		return "prepare file edit"
	default:
		return "prepare tool call"
	}
}

func isInterruptError(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil
}
