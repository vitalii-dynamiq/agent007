package llm

import (
	"context"
)

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`    // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool represents a tool definition for the LLM
type Tool struct {
	Type     string      `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines a function tool
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content     string     `json:"content"`
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string    `json:"finish_reason"`
}

// StreamChunk represents a streamed chunk of response
type StreamChunk struct {
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Done       bool       `json:"done"`
	Error      error      `json:"-"`
}

// Client interface for LLM providers
type Client interface {
	// ChatCompletion performs a non-streaming chat completion
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	
	// ChatCompletionStream performs a streaming chat completion
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// Config for LLM client
type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

// NewClient creates a new LLM client based on provider
func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "openai", "":
		return NewOpenAIClient(cfg)
	default:
		// Default to OpenAI-compatible
		return NewOpenAIClient(cfg)
	}
}
