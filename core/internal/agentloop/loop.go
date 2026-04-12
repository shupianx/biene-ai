package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"biene/internal/api"
	"biene/internal/tools"
)

// EventKind classifies an Event.
type EventKind string

const (
	KindTextDelta   EventKind = "text_delta"
	KindToolCompose EventKind = "tool_compose"
	KindToolStart   EventKind = "tool_start"
	KindToolResult  EventKind = "tool_result"
	KindToolDenied  EventKind = "tool_denied"
	KindInterrupted EventKind = "interrupted"
	KindDone        EventKind = "done"
	KindError       EventKind = "error"
)

// Event is a single update emitted to the caller.
type Event struct {
	Kind        EventKind
	Text        string
	ToolID      string
	ToolName    string
	ToolSummary string
	ToolInput   json.RawMessage
	IsError     bool
}

// PermissionChecker decides whether a tool call is allowed to proceed.
// Both permission.Checker (CLI) and webperm.Checker (Web) implement this.
type PermissionChecker interface {
	Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, error)
}

// Config holds everything needed for a single conversational turn.
type Config struct {
	Provider     api.Provider
	Registry     *tools.Registry
	Checker      PermissionChecker
	SystemPrompt string
	Messages     []api.Message
	MaxTokens    int
}

// Run executes one user turn and emits events on the returned channel.
func Run(ctx context.Context, cfg *Config) <-chan Event {
	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		if err := runLoop(ctx, cfg, ch); err != nil {
			if isInterruptError(ctx, err) {
				ch <- Event{Kind: KindInterrupted}
				return
			}
			ch <- Event{Kind: KindError, Text: err.Error()}
			return
		}
		ch <- Event{Kind: KindDone}
	}()
	return ch
}

func runLoop(ctx context.Context, cfg *Config, ch chan<- Event) error {
	toolDefs := toolDefinitions(cfg.Registry)

	for {
		stream, err := cfg.Provider.Stream(
			ctx,
			cfg.SystemPrompt,
			cfg.Messages,
			toolDefs,
			cfg.MaxTokens,
		)
		if err != nil {
			return fmt.Errorf("API error: %w", err)
		}

		var preparedWriteToolID string
		var preparedWrite *preparedPermission

		assistantMsg, toolUses, err := collectStream(ctx, stream, ch, func(tu api.ToolUseBlock) {
			tool := cfg.Registry.Find(tu.Name)
			if tool == nil || tool.PermissionKey() != tools.PermissionWrite {
				return
			}

			ch <- Event{
				Kind:        KindToolCompose,
				ToolID:      tu.ID,
				ToolName:    tu.Name,
				ToolSummary: earlyToolSummary(tu.Name),
			}

			if preparedWrite != nil {
				return
			}
			preparedWriteToolID = tu.ID
			preparedWrite = startPreparedPermission(ctx, cfg.Checker, tool, json.RawMessage(`{}`))
		})
		if err != nil {
			return err
		}
		cfg.Messages = append(cfg.Messages, assistantMsg)

		if len(toolUses) == 0 {
			return nil
		}

		var resultBlocks []api.ContentBlock
		for _, tu := range toolUses {
			tool := cfg.Registry.Find(tu.Name)
			if tool == nil {
				errMsg := fmt.Sprintf("unknown tool: %q", tu.Name)
				ch <- Event{Kind: KindToolResult, ToolID: tu.ID, ToolName: tu.Name, Text: errMsg, IsError: true}
				resultBlocks = append(resultBlocks, api.ToolResultBlock{
					ToolUseID: tu.ID,
					Content:   errMsg,
					IsError:   true,
				})
				continue
			}

			ch <- Event{
				Kind:        KindToolStart,
				ToolID:      tu.ID,
				ToolName:    tu.Name,
				ToolSummary: tool.Summary(tu.Input),
				ToolInput:   tu.Input,
			}
			var allowed bool
			if preparedWrite != nil && tu.ID == preparedWriteToolID {
				allowed, err = preparedWrite.Wait()
			} else {
				allowed, err = cfg.Checker.Check(ctx, tool, tu.Input)
			}
			if err != nil {
				return fmt.Errorf("permission check: %w", err)
			}
			if !allowed {
				denyMsg := "User denied this tool call."
				ch <- Event{Kind: KindToolDenied, ToolID: tu.ID, ToolName: tu.Name, Text: denyMsg}
				resultBlocks = append(resultBlocks, api.ToolResultBlock{
					ToolUseID: tu.ID,
					Content:   denyMsg,
					IsError:   true,
				})
				continue
			}

			result, execErr := tool.Execute(ctx, tu.Input)
			if execErr != nil {
				if isInterruptError(ctx, execErr) {
					return execErr
				}
				result = fmt.Sprintf("Error: %s", execErr)
				ch <- Event{Kind: KindToolResult, ToolID: tu.ID, ToolName: tu.Name, Text: result, IsError: true}
				resultBlocks = append(resultBlocks, api.ToolResultBlock{
					ToolUseID: tu.ID,
					Content:   result,
					IsError:   true,
				})
				continue
			}
			ch <- Event{Kind: KindToolResult, ToolID: tu.ID, ToolName: tu.Name, Text: result, IsError: false}
			resultBlocks = append(resultBlocks, api.ToolResultBlock{
				ToolUseID: tu.ID,
				Content:   result,
				IsError:   false,
			})
		}

		cfg.Messages = append(cfg.Messages, api.Message{
			Role:    api.RoleUser,
			Content: resultBlocks,
		})
	}
}

func toolDefinitions(registry *tools.Registry) []api.ToolDefinition {
	defs := make([]api.ToolDefinition, 0, len(registry.All()))
	for _, tool := range registry.All() {
		defs = append(defs, api.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

type preparedPermission struct {
	done    chan struct{}
	allowed bool
	err     error
}

func startPreparedPermission(
	ctx context.Context,
	checker PermissionChecker,
	tool tools.Tool,
	input json.RawMessage,
) *preparedPermission {
	prep := &preparedPermission{done: make(chan struct{})}
	go func() {
		prep.allowed, prep.err = checker.Check(ctx, tool, input)
		close(prep.done)
	}()
	return prep
}

func (p *preparedPermission) Wait() (bool, error) {
	<-p.done
	return p.allowed, p.err
}

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
