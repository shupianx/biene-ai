package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicMaxTokensFor returns a safe per-model output-token cap. The value
// matches each Claude 4 model's documented maximum so reasoning-heavy turns
// are not truncated mid-thought.
func anthropicMaxTokensFor(model string) int64 {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return 32000
	case strings.Contains(m, "sonnet"), strings.Contains(m, "haiku"):
		return 64000
	default:
		return 32000
	}
}

// AnthropicProvider implements Provider using the official Anthropic Go SDK.
// It uses the Beta.Messages API which supports the latest features.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

// NewAnthropicProvider creates a new Anthropic-backed provider.
// baseURL is optional; leave empty to use the official API endpoint.
func NewAnthropicProvider(apiKey, model, baseURL string) *AnthropicProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &AnthropicProvider{
		client: anthropic.NewClient(opts...),
		model:  model,
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic/" + p.model }

// Stream implements Provider.Stream.
func (p *AnthropicProvider) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	tools []ToolDefinition,
	_ RequestOptions,
) (<-chan StreamEvent, error) {
	apiMessages, err := convertMessagesToAnthropic(messages)
	if err != nil {
		return nil, fmt.Errorf("converting messages: %w", err)
	}

	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: anthropicMaxTokensFor(p.model),
		System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
		Messages:  apiMessages,
	}
	if len(tools) > 0 {
		params.Tools = convertToolsToAnthropic(tools)
	}

	stream := p.client.Beta.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamEvent, 64)
	go func() {
		defer close(ch)

		var currentToolID, currentToolName string
		var toolInputBuf []byte

		for stream.Next() {
			ev := stream.Current()
			switch ev.AsAny().(type) {
			case anthropic.BetaRawContentBlockStartEvent:
				startEv := ev.AsContentBlockStart()
				// Check if it's a tool_use block by inspecting the Type field
				block := startEv.ContentBlock
				if block.Type == "tool_use" {
					tu := block.AsToolUse()
					currentToolID = tu.ID
					currentToolName = tu.Name
					toolInputBuf = nil
					ch <- StreamEvent{
						Type: EventToolUseStart,
						ToolUse: &ToolUseBlock{
							ID:   currentToolID,
							Name: currentToolName,
						},
					}
				}

			case anthropic.BetaRawContentBlockDeltaEvent:
				deltaEv := ev.AsContentBlockDelta()
				switch deltaEv.Delta.AsAny().(type) {
				case anthropic.BetaTextDelta:
					td := deltaEv.Delta.AsTextDelta()
					ch <- StreamEvent{Type: EventTextDelta, Text: td.Text}
				case anthropic.BetaThinkingDelta:
					tk := deltaEv.Delta.AsThinkingDelta()
					ch <- StreamEvent{Type: EventReasoningDelta, Text: tk.Thinking}
				case anthropic.BetaSignatureDelta:
					sd := deltaEv.Delta.AsSignatureDelta()
					ch <- StreamEvent{Type: EventSignatureDelta, Text: sd.Signature}
				case anthropic.BetaInputJSONDelta:
					ij := deltaEv.Delta.AsInputJSONDelta()
					toolInputBuf = append(toolInputBuf, ij.PartialJSON...)
					if currentToolID != "" {
						ch <- StreamEvent{
							Type:      EventInputJSONDelta,
							ToolUseID: currentToolID,
							InputJSON: ij.PartialJSON,
						}
					}
				}

			case anthropic.BetaRawContentBlockStopEvent:
				if currentToolID != "" {
					input := finalizeToolInput(toolInputBuf, currentToolID, currentToolName)
					ch <- StreamEvent{
						Type: EventToolUse,
						ToolUse: &ToolUseBlock{
							ID:    currentToolID,
							Name:  currentToolName,
							Input: input,
						},
					}
					currentToolID = ""
					currentToolName = ""
					toolInputBuf = nil
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: EventError, Err: err}
			return
		}
		ch <- StreamEvent{Type: EventDone}
	}()

	return ch, nil
}

// ─── Conversion helpers ───────────────────────────────────────────────────

func convertMessagesToAnthropic(msgs []Message) ([]anthropic.BetaMessageParam, error) {
	out := make([]anthropic.BetaMessageParam, 0, len(msgs))
	for _, m := range msgs {
		var blocks []anthropic.BetaContentBlockParamUnion
		for _, b := range m.Content {
			switch v := b.(type) {
			case TextBlock:
				blocks = append(blocks, anthropic.NewBetaTextBlock(v.Text))
			case ReasoningBlock:
				if v.Signature == "" {
					// Anthropic rejects thinking blocks without a signature; skip
					// reasoning captured from providers that don't issue one.
					continue
				}
				blocks = append(blocks, anthropic.NewBetaThinkingBlock(v.Signature, v.Text))
			case ToolUseBlock:
				blocks = append(blocks, anthropic.NewBetaToolUseBlock(v.ID, v.Input, v.Name))
			case ToolResultBlock:
				blocks = append(blocks, anthropic.NewBetaToolResultBlock(v.ToolUseID, v.Content, v.IsError))
			case ImageBlock:
				if len(v.Data) == 0 {
					continue
				}
				blocks = append(blocks, anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{
					Data:      base64.StdEncoding.EncodeToString(v.Data),
					MediaType: anthropic.BetaBase64ImageSourceMediaType(v.MediaType),
				}))
			default:
				return nil, fmt.Errorf("unknown content block type: %T", b)
			}
		}
		switch m.Role {
		case RoleUser:
			out = append(out, anthropic.NewBetaUserMessage(blocks...))
		case RoleAssistant:
			out = append(out, anthropic.BetaMessageParam{
				Role:    anthropic.BetaMessageParamRoleAssistant,
				Content: blocks,
			})
		default:
			return nil, fmt.Errorf("unknown role: %q", m.Role)
		}
	}
	return out, nil
}

func convertToolsToAnthropic(tools []ToolDefinition) []anthropic.BetaToolUnionParam {
	out := make([]anthropic.BetaToolUnionParam, 0, len(tools))
	for _, t := range tools {
		// Parse the JSON Schema to extract properties
		var schemaObj map[string]interface{}
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &schemaObj)
		}

		var properties interface{}
		var required []string
		if schemaObj != nil {
			properties = schemaObj["properties"]
			if req, ok := schemaObj["required"].([]interface{}); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						required = append(required, s)
					}
				}
			}
		}

		tool := anthropic.BetaToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.BetaToolInputSchemaParam{
				Properties: properties,
				Required:   required,
			},
		}
		out = append(out, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return out
}
