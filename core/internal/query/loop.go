package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"biene/internal/api"
	"biene/internal/tools"
)

// ─── Event types emitted by Run ───────────────────────────────────────────

// EventKind classifies a query.Event.
type EventKind string

const (
	KindTextDelta   EventKind = "text_delta"   // partial assistant text
	KindToolCompose EventKind = "tool_compose" // tool intent detected, still composing input
	KindToolStart   EventKind = "tool_start"   // tool about to execute
	KindToolResult  EventKind = "tool_result"  // tool finished
	KindToolDenied  EventKind = "tool_denied"  // user denied permission
	KindInterrupted EventKind = "interrupted"  // conversation turn was interrupted
	KindDone        EventKind = "done"         // conversation turn complete
	KindError       EventKind = "error"        // unrecoverable error
)

// Event is a single update emitted to the caller.
type Event struct {
	Kind        EventKind
	Text        string          // KindTextDelta, KindToolResult, KindToolDenied, KindError
	ToolID      string          // KindToolCompose / KindToolStart / KindToolResult / KindToolDenied
	ToolName    string          // KindToolStart / KindToolResult / KindToolDenied
	ToolSummary string          // KindToolStart — human-readable description
	ToolInput   json.RawMessage // KindToolStart
	IsError     bool            // KindToolResult
}

// ─── Permission interface ─────────────────────────────────────────────────

// PermissionChecker decides whether a tool call is allowed to proceed.
// Both permission.Checker (CLI) and permission.HTTPChecker (Web) implement this.
type PermissionChecker interface {
	Check(ctx context.Context, tool tools.Tool, input json.RawMessage) (bool, error)
}

// ─── Run config ───────────────────────────────────────────────────────────

// Config holds everything needed for a single conversational turn.
type Config struct {
	Provider     api.Provider
	Registry     *tools.Registry
	Checker      PermissionChecker
	SystemPrompt string
	Messages     []api.Message // mutable: the caller owns this slice
	MaxTokens    int
}

// ─── Run ──────────────────────────────────────────────────────────────────

// Run executes one user turn and emits events on the returned channel.
// It manages the full agentic loop: model → tools → model → … until the
// model produces a response with no tool calls.
//
// Appended messages (assistant reply + tool results) are added to
// cfg.Messages so the caller can pass the updated history to the next turn.
//
// The channel is closed when the turn is complete or on error.
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
	toolDefs := cfg.Registry.Definitions()

	for {
		// ── 1. Call the model (streaming) ────────────────────────────────
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

		// Collect the assistant's response into a Message
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

		// ── 2. No tool calls → turn is done ──────────────────────────────
		if len(toolUses) == 0 {
			return nil
		}

		// ── 3. Execute each tool call (serially) ─────────────────────────
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

			// Permission check — send KindToolStart before asking
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

			// Execute
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

		// ── 4. Append tool results as a user message and loop ─────────────
		cfg.Messages = append(cfg.Messages, api.Message{
			Role:    api.RoleUser,
			Content: resultBlocks,
		})
	}
}

// collectStream drains the streaming channel, forwards text deltas as events,
// and returns the assembled assistant Message plus any tool_use blocks.
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

func earlyToolSummary(toolName string) string {
	switch toolName {
	case "Write":
		return "Preparing file write"
	case "Edit":
		return "Preparing file edit"
	default:
		return "Preparing tool input"
	}
}

func collectStream(
	ctx context.Context,
	stream <-chan api.StreamEvent,
	ch chan<- Event,
	onToolUseStart func(api.ToolUseBlock),
) (api.Message, []api.ToolUseBlock, error) {
	var textBuf []string
	var toolUses []api.ToolUseBlock

	for {
		select {
		case <-ctx.Done():
			return api.Message{}, nil, ctx.Err()
		case ev, ok := <-stream:
			if !ok {
				break
			}
			switch ev.Type {
			case api.EventTextDelta:
				ch <- Event{Kind: KindTextDelta, Text: ev.Text}
				textBuf = append(textBuf, ev.Text)
			case api.EventToolUseStart:
				if ev.ToolUse != nil && onToolUseStart != nil {
					onToolUseStart(*ev.ToolUse)
				}
			case api.EventToolUse:
				toolUses = append(toolUses, *ev.ToolUse)
			case api.EventError:
				return api.Message{}, nil, ev.Err
			case api.EventDone:
				goto done
			}
			continue
		}
		break
	}
done:
	var content []api.ContentBlock
	if len(textBuf) > 0 {
		combined := ""
		for _, t := range textBuf {
			combined += t
		}
		content = append(content, api.TextBlock{Text: combined})
	}
	for _, tu := range toolUses {
		content = append(content, tu)
	}

	msg := api.Message{Role: api.RoleAssistant, Content: content}
	return msg, toolUses, nil
}

func isInterruptError(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}
	return errors.Is(err, ctx.Err()) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
