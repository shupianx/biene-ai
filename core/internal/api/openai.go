package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements Provider using the official OpenAI Go SDK and a
// small Chat Completions compatibility layer for OpenAI-compatible backends.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
// Set baseURL to "" to use the official OpenAI API.
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIBaseURL
	}

	opts := []option.RequestOption{
		option.WithBaseURL(strings.TrimRight(baseURL, "/")),
	}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	return &OpenAIProvider{
		client: &openai.Client{Options: opts},
		model:  model,
	}
}

func (p *OpenAIProvider) Name() string { return "openai/" + p.model }

// Stream implements Provider.Stream.
func (p *OpenAIProvider) Stream(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	tools []ToolDefinition,
	opts RequestOptions,
) (<-chan StreamEvent, error) {
	apiMessages, err := convertMessagesToOpenAI(messages, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("converting messages: %w", err)
	}

	req := openAIChatCompletionRequest{
		Model:    p.model,
		Messages: apiMessages,
		Stream:   true,
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

			reasoningText := delta.ReasoningContent
			if reasoningText == "" {
				reasoningText = delta.Reasoning
			}
			if reasoningText != "" {
				ch <- StreamEvent{Type: EventReasoningDelta, Text: reasoningText}
			}

			if delta.Content != "" {
				ch <- StreamEvent{Type: EventTextDelta, Text: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				if tc.Index == nil {
					continue
				}
				i := *tc.Index
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

			if resp.Choices[0].FinishReason == openAIChatFinishReasonToolCalls {
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
	Recv() (openAIChatCompletionStreamResponse, error)
	Close() error
}

func (p *OpenAIProvider) openStream(
	ctx context.Context,
	req openAIChatCompletionRequest,
	opts RequestOptions,
) (chatCompletionStream, error) {
	var resp *http.Response
	reqOpts := []option.RequestOption{
		option.WithHeader("Accept", "text/event-stream"),
		option.WithHeader("Cache-Control", "no-cache"),
		option.WithHeader("Connection", "keep-alive"),
	}
	for key, value := range opts.ThinkingExtra {
		reqOpts = append(reqOpts, option.WithJSONSet(key, value))
	}

	if err := p.client.Post(ctx, "chat/completions", req, &resp, reqOpts...); err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("empty streaming response")
	}

	return &manualChatCompletionStream{
		body:   resp.Body,
		reader: bufio.NewReader(resp.Body),
	}, nil
}

type manualChatCompletionStream struct {
	body   io.ReadCloser
	reader *bufio.Reader
}

func (s *manualChatCompletionStream) Close() error {
	return s.body.Close()
}

func (s *manualChatCompletionStream) Recv() (openAIChatCompletionStreamResponse, error) {
	var (
		resp      openAIChatCompletionStreamResponse
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

type openAIChatCompletionRequest struct {
	Model    string                        `json:"model"`
	Messages []openAIChatCompletionMessage `json:"messages"`
	Stream   bool                          `json:"stream,omitempty"`
	Tools    []openAIChatCompletionTool    `json:"tools,omitempty"`
}

type openAIChatCompletionMessage struct {
	Role             string                         `json:"role"`
	Content          any                            `json:"content,omitempty"`
	ReasoningContent string                         `json:"reasoning_content,omitempty"`
	ToolCallID       string                         `json:"tool_call_id,omitempty"`
	ToolCalls        []openAIChatCompletionToolCall `json:"tool_calls,omitempty"`
}

type openAIContentPart struct {
	Type     string                      `json:"type"`
	Text     string                      `json:"text,omitempty"`
	ImageURL *openAIContentPartImageURL  `json:"image_url,omitempty"`
}

type openAIContentPartImageURL struct {
	URL string `json:"url"`
}

type openAIChatCompletionTool struct {
	Type     string                                  `json:"type"`
	Function *openAIChatCompletionFunctionDefinition `json:"function,omitempty"`
}

type openAIChatCompletionFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openAIChatCompletionToolCall struct {
	ID       string                           `json:"id,omitempty"`
	Type     string                           `json:"type"`
	Function openAIChatCompletionFunctionCall `json:"function"`
}

type openAIChatCompletionFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatCompletionStreamResponse struct {
	Choices []openAIChatCompletionStreamChoice `json:"choices"`
}

type openAIChatCompletionStreamChoice struct {
	Delta        openAIChatCompletionStreamChoiceDelta `json:"delta"`
	FinishReason string                                `json:"finish_reason"`
}

type openAIChatCompletionStreamChoiceDelta struct {
	Content          string                              `json:"content"`
	Reasoning        string                              `json:"reasoning"`
	ReasoningContent string                              `json:"reasoning_content"`
	ToolCalls        []openAIChatCompletionDeltaToolCall `json:"tool_calls"`
}

type openAIChatCompletionDeltaToolCall struct {
	Index    *int                             `json:"index"`
	ID       string                           `json:"id"`
	Function openAIChatCompletionFunctionCall `json:"function"`
}

const openAIChatFinishReasonToolCalls = "tool_calls"

// ─── Conversion helpers ───────────────────────────────────────────────────

func convertMessagesToOpenAI(msgs []Message, systemPrompt string) ([]openAIChatCompletionMessage, error) {
	out := []openAIChatCompletionMessage{
		{Role: "system", Content: systemPrompt},
	}

	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			var parts []openAIContentPart
			var textBuf strings.Builder
			for _, b := range m.Content {
				switch v := b.(type) {
				case TextBlock:
					if v.Text == "" {
						continue
					}
					parts = append(parts, openAIContentPart{Type: "text", Text: v.Text})
					if textBuf.Len() > 0 {
						textBuf.WriteString("\n")
					}
					textBuf.WriteString(v.Text)
				case ImageBlock:
					if len(v.Data) == 0 {
						continue
					}
					dataURL := "data:" + v.MediaType + ";base64," + base64.StdEncoding.EncodeToString(v.Data)
					parts = append(parts, openAIContentPart{
						Type:     "image_url",
						ImageURL: &openAIContentPartImageURL{URL: dataURL},
					})
				case ToolResultBlock:
					out = append(out, openAIChatCompletionMessage{
						Role:       "tool",
						Content:    v.Content,
						ToolCallID: v.ToolUseID,
					})
				}
			}
			if len(parts) > 0 {
				hasImage := false
				for _, p := range parts {
					if p.Type == "image_url" {
						hasImage = true
						break
					}
				}
				msg := openAIChatCompletionMessage{Role: "user"}
				if hasImage {
					msg.Content = parts
				} else {
					msg.Content = textBuf.String()
				}
				out = append(out, msg)
			}

		case RoleAssistant:
			msg := openAIChatCompletionMessage{Role: "assistant"}
			var toolCalls []openAIChatCompletionToolCall
			for _, b := range m.Content {
				switch v := b.(type) {
				case TextBlock:
					if v.Text != "" {
						msg.Content = v.Text
					}
				case ReasoningBlock:
					msg.ReasoningContent = v.Text
				case ToolUseBlock:
					toolCalls = append(toolCalls, openAIChatCompletionToolCall{
						ID:   v.ID,
						Type: "function",
						Function: openAIChatCompletionFunctionCall{
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

func convertToolsToOpenAI(tools []ToolDefinition) []openAIChatCompletionTool {
	out := make([]openAIChatCompletionTool, 0, len(tools))
	for _, t := range tools {
		var params interface{}
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &params)
		}
		paramBytes, _ := json.Marshal(params)
		out = append(out, openAIChatCompletionTool{
			Type: "function",
			Function: &openAIChatCompletionFunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(paramBytes),
			},
		})
	}
	return out
}
