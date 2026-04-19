package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider using the OpenAI-compatible API.
// It works with OpenAI, Ollama, 豆包, Gemini OpenAI-compat, DeepSeek, etc.
// by setting BaseURL in the client config.
type OpenAIProvider struct {
	client     *openai.Client
	httpClient openai.HTTPDoer
	apiKey     string
	baseURL    string
	model      string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
// Set baseURL to "" to use the official OpenAI API.
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAIProvider{
		client:     openai.NewClientWithConfig(cfg),
		httpClient: httpClient,
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      model,
	}
}

func (p *OpenAIProvider) Name() string { return "openai/" + p.model }

// Stream implements Provider.Stream.
func (p *OpenAIProvider) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	tools []ToolDefinition,
	maxTokens int,
	opts RequestOptions,
) (<-chan StreamEvent, error) {
	apiMessages, err := convertMessagesToOpenAI(messages, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("converting messages: %w", err)
	}

	req := openai.ChatCompletionRequest{
		Model:     p.model,
		Messages:  apiMessages,
		MaxTokens: maxTokens,
		Stream:    true,
	}
	if len(tools) > 0 {
		req.Tools = convertToolsToOpenAI(tools)
	}

	stream, err := p.openStream(ctx, req, opts)
	if err != nil {
		return nil, fmt.Errorf("creating stream: %w", err)
	}

	ch := make(chan StreamEvent, 64)
	go func() {
		defer close(ch)
		defer stream.Close()

		// Accumulate tool call deltas by index
		type toolAccum struct {
			id      string
			name    string
			args    []byte
			started bool
		}
		toolAccums := map[int]*toolAccum{}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) || err.Error() == "EOF" {
					break
				}
				ch <- StreamEvent{Type: EventError, Err: err}
				return
			}
			if len(resp.Choices) == 0 {
				continue
			}
			delta := resp.Choices[0].Delta

			// Reasoning content for providers like Qwen / DeepSeek.
			if delta.ReasoningContent != "" {
				ch <- StreamEvent{Type: EventReasoningDelta, Text: delta.ReasoningContent}
			}

			// Text content
			if delta.Content != "" {
				ch <- StreamEvent{Type: EventTextDelta, Text: delta.Content}
			}

			// Tool call deltas
			for _, tc := range delta.ToolCalls {
				idx := tc.Index
				if idx == nil {
					continue
				}
				i := *idx
				if _, ok := toolAccums[i]; !ok {
					toolAccums[i] = &toolAccum{
						id: fmt.Sprintf("openai_tool_%d", i),
					}
				}
				acc := toolAccums[i]
				if tc.ID != "" && !acc.started {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				if !acc.started && acc.name != "" {
					acc.started = true
					ch <- StreamEvent{
						Type: EventToolUseStart,
						ToolUse: &ToolUseBlock{
							ID:   acc.id,
							Name: acc.name,
						},
					}
				}
				acc.args = append(acc.args, tc.Function.Arguments...)
			}

			// Emit complete tool uses when finish_reason arrives
			if resp.Choices[0].FinishReason == openai.FinishReasonToolCalls {
				for _, acc := range toolAccums {
					input := json.RawMessage(acc.args)
					if len(input) == 0 {
						input = json.RawMessage("{}")
					}
					ch <- StreamEvent{
						Type: EventToolUse,
						ToolUse: &ToolUseBlock{
							ID:    acc.id,
							Name:  acc.name,
							Input: input,
						},
					}
				}
				toolAccums = map[int]*toolAccum{}
			}
		}

		// Emit any remaining tool uses (some providers don't send finish_reason=tool_calls)
		for _, acc := range toolAccums {
			if acc.id == "" && acc.name == "" {
				continue
			}
			input := json.RawMessage(acc.args)
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			ch <- StreamEvent{
				Type: EventToolUse,
				ToolUse: &ToolUseBlock{
					ID:    acc.id,
					Name:  acc.name,
					Input: input,
				},
			}
		}

		ch <- StreamEvent{Type: EventDone}
	}()

	return ch, nil
}

type chatCompletionStream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

func (p *OpenAIProvider) openStream(
	ctx context.Context,
	req openai.ChatCompletionRequest,
	opts RequestOptions,
) (chatCompletionStream, error) {
	if opts.EnableThinking == nil {
		return p.client.CreateChatCompletionStream(ctx, req)
	}

	body, err := marshalChatCompletionRequest(req, map[string]any{
		"enable_thinking": *opts.EnableThinking,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		bodyText, _ := io.ReadAll(resp.Body)
		if len(bodyText) == 0 {
			return nil, fmt.Errorf("unexpected status: %s", resp.Status)
		}
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(bodyText)))
	}

	return &manualChatCompletionStream{
		body:   resp.Body,
		reader: bufio.NewReader(resp.Body),
	}, nil
}

func marshalChatCompletionRequest(req openai.ChatCompletionRequest, extra map[string]any) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return body, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	for key, value := range extra {
		payload[key] = value
	}
	return json.Marshal(payload)
}

type manualChatCompletionStream struct {
	body   io.ReadCloser
	reader *bufio.Reader
}

func (s *manualChatCompletionStream) Close() error {
	return s.body.Close()
}

func (s *manualChatCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	var (
		resp      openai.ChatCompletionStreamResponse
		dataLines []string
	)

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(dataLines) == 0 {
					return resp, io.EOF
				}
				break
			}
			return resp, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) == 0 {
				continue
			}
			break
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	payload := strings.Join(dataLines, "\n")
	if payload == "[DONE]" {
		return resp, io.EOF
	}
	if payload == "" {
		return resp, io.EOF
	}
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// ─── Conversion helpers ───────────────────────────────────────────────────

func convertMessagesToOpenAI(msgs []Message, systemPrompt string) ([]openai.ChatCompletionMessage, error) {
	out := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			// User messages may contain text and tool_result blocks.
			// OpenAI encodes tool results as separate "tool" role messages.
			for _, b := range m.Content {
				switch v := b.(type) {
				case TextBlock:
					out = append(out, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: v.Text,
					})
				case ToolResultBlock:
					out = append(out, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    v.Content,
						ToolCallID: v.ToolUseID,
					})
				}
			}

		case RoleAssistant:
			msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
			var toolCalls []openai.ToolCall
			for _, b := range m.Content {
				switch v := b.(type) {
				case TextBlock:
					msg.Content = v.Text
				case ToolUseBlock:
					toolCalls = append(toolCalls, openai.ToolCall{
						ID:   v.ID,
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      v.Name,
							Arguments: string(v.Input),
						},
					})
				}
			}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
			out = append(out, msg)

		default:
			return nil, fmt.Errorf("unknown role: %q", m.Role)
		}
	}
	return out, nil
}

func convertToolsToOpenAI(tools []ToolDefinition) []openai.Tool {
	out := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		var params interface{}
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &params)
		}
		paramBytes, _ := json.Marshal(params)
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(paramBytes),
			},
		})
	}
	return out
}
