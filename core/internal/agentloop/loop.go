package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"biene/internal/api"
	"biene/internal/tools"
)

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
	log := slog.Default().With("session_id", cfg.SessionID)
	toolDefs := toolDefinitions(cfg.Registry)
	log.Info("turn start", "tool_count", len(toolDefs), "message_count", len(cfg.Messages))

	for {
		stream, err := cfg.Provider.Stream(
			ctx,
			cfg.SystemPrompt,
			cfg.Messages,
			toolDefs,
			cfg.RequestOpts,
		)
		if err != nil {
			log.Error("provider stream failed", "err", err)
			return fmt.Errorf("API error: %w", err)
		}

		var preparedWriteToolID string
		var preparedWrite *preparedPermission

		// Per-tool progress state for write-class tools. The parser incrementally
		// extracts file_path and counts file_text bytes as the tool input streams
		// in, so the UI permission card can show real information instead of the
		// blind "preparing…" placeholder during pre-warmed approval.
		progressParsers := map[string]*writeProgressParser{}
		progressTools := map[string]struct{}{}
		lastSnapshots := map[string]writeProgressSnapshot{}
		toolNames := map[string]string{}

		assistantMsg, toolUses, err := collectStream(ctx, stream, ch, func(tu api.ToolUseBlock) {
			tool := cfg.Registry.Find(tu.Name)
			if tool == nil || tool.PermissionKey() != tools.PermissionWrite {
				return
			}
			toolNames[tu.ID] = tu.Name
			progressTools[tu.ID] = struct{}{}
			progressParsers[tu.ID] = &writeProgressParser{}

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
			preparedWrite = startPreparedPermission(ctx, cfg.Checker, tool, tu.ID, json.RawMessage(`{}`))
		}, func(toolID, chunk string) {
			if _, ok := progressTools[toolID]; !ok {
				return
			}
			parser := progressParsers[toolID]
			if parser == nil {
				return
			}
			parser.Append(chunk)
			snap := parser.Snapshot()
			if snap == lastSnapshots[toolID] {
				return
			}
			lastSnapshots[toolID] = snap
			ch <- Event{
				Kind:          KindToolComposeProgress,
				ToolID:        toolID,
				ToolName:      toolNames[toolID],
				FilePath:      snap.FilePath,
				FileTextBytes: snap.FileTextBytes,
			}
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
			var resolution json.RawMessage
			if preparedWrite != nil && tu.ID == preparedWriteToolID {
				allowed, resolution, err = preparedWrite.Wait()
			} else {
				allowed, resolution, err = cfg.Checker.Check(tools.WithToolID(ctx, tu.ID), tool, tu.Input)
			}
			if err != nil {
				log.Error("permission check failed", "tool", tu.Name, "tool_id", tu.ID, "err", err)
				return fmt.Errorf("permission check: %w", err)
			}
			if !allowed {
				log.Info("tool denied", "tool", tu.Name, "tool_id", tu.ID)
				denyMsg := "User denied this tool call."
				ch <- Event{Kind: KindToolDenied, ToolID: tu.ID, ToolName: tu.Name, Text: denyMsg}
				resultBlocks = append(resultBlocks, api.ToolResultBlock{
					ToolUseID: tu.ID,
					Content:   denyMsg,
					IsError:   true,
				})
				continue
			}

			execCtx := tools.WithPermissionResolution(ctx, resolution)
			log.Debug("tool execute", "tool", tu.Name, "tool_id", tu.ID)
			result, execErr := tool.Execute(execCtx, tu.Input)
			if execErr != nil {
				if isInterruptError(ctx, execErr) {
					return execErr
				}
				log.Warn("tool execute failed", "tool", tu.Name, "tool_id", tu.ID, "err", execErr)
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
