package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

// OpenAIClient implements the Client interface for OpenAI
type OpenAIClient struct {
	client *openai.Client
	model  string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(cfg Config) (*OpenAIClient, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}

	model := cfg.Model
	if model == "" {
		model = "gpt-4o"
	}

	return &OpenAIClient{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}, nil
}

// ChatCompletion performs a non-streaming chat completion
func (c *OpenAIClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}
		
		// Include tool calls if present
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				messages[i].ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	}

	if len(req.Tools) > 0 {
		openaiReq.Tools = convertTools(req.Tools)
	}

	if req.Temperature > 0 {
		openaiReq.Temperature = req.Temperature
	}

	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = req.MaxTokens
	}

	resp, err := c.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices returned from OpenAI")
	}

	choice := resp.Choices[0]
	response := &ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
	}

	if len(choice.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			response.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
			}
			response.ToolCalls[i].Function.Name = tc.Function.Name
			response.ToolCalls[i].Function.Arguments = tc.Function.Arguments
		}
	}

	return response, nil
}

// ChatCompletionStream performs a streaming chat completion
func (c *OpenAIClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	if len(req.Tools) > 0 {
		openaiReq.Tools = convertTools(req.Tools)
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, openaiReq)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer stream.Close()

		var toolCalls []ToolCall
		toolCallArgs := make(map[int]string) // index -> accumulated arguments

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- StreamChunk{Done: true, ToolCalls: toolCalls}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}

			if len(response.Choices) == 0 {
				continue
			}

			delta := response.Choices[0].Delta

			// Handle content
			if delta.Content != "" {
				ch <- StreamChunk{Content: delta.Content}
			}

			// Handle tool calls
			for _, tc := range delta.ToolCalls {
				idx := *tc.Index

				// Initialize tool call if new
				if idx >= len(toolCalls) {
					toolCalls = append(toolCalls, ToolCall{
						ID:   tc.ID,
						Type: string(tc.Type),
					})
					toolCalls[idx].Function.Name = tc.Function.Name
				}

				// Accumulate arguments
				if tc.Function.Arguments != "" {
					toolCallArgs[idx] += tc.Function.Arguments
					toolCalls[idx].Function.Arguments = toolCallArgs[idx]
				}
			}
		}
	}()

	return ch, nil
}

func convertTools(tools []Tool) []openai.Tool {
	result := make([]openai.Tool, len(tools))
	for i, t := range tools {
		params, _ := json.Marshal(t.Function.Parameters)
		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  json.RawMessage(params),
			},
		}
	}
	return result
}
