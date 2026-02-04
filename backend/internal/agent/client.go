// Package agent provides a client for calling the Python agent service.
//
// The Python agent handles LLM interactions and E2B sandbox management.
// This client streams SSE events from the agent back to the frontend.
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls the Python agent service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new agent client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // Long timeout for agent runs
		},
	}
}

// Message represents a chat message for the agent.
type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Result    string `json:"result,omitempty"`
	} `json:"tool_calls,omitempty"`
}

// UploadedFile represents a file uploaded by the user.
type UploadedFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
	Data string `json:"data"` // base64 encoded
}

// RunRequest is the request to run the agent.
type RunRequest struct {
	Message        string         `json:"message"`
	Messages       []Message      `json:"messages,omitempty"`        // Full conversation history
	UserID         string         `json:"user_id"`
	SessionToken   string         `json:"session_token"`
	ConversationID string         `json:"conversation_id,omitempty"`
	SandboxID      string         `json:"sandbox_id,omitempty"`      // Reuse existing sandbox
	MCPProxyURL    string         `json:"mcp_proxy_url,omitempty"`   // Backend MCP proxy URL
	Files          []UploadedFile `json:"files,omitempty"`           // Files to upload to sandbox
}

// RunResponse is the response from the agent.
type RunResponse struct {
	Response  string `json:"response"`
	SandboxID string `json:"sandbox_id,omitempty"`
}

// Event represents an SSE event from the agent.
type Event struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content,omitempty"`
}

// Run executes the agent and returns the result (non-streaming).
func (c *Client) Run(ctx context.Context, req RunRequest) (*RunResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var result RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// RunStream executes the agent with SSE streaming.
// Events are sent to the eventChan channel.
func (c *Client) RunStream(ctx context.Context, req RunRequest, eventChan chan<- Event) error {
	defer close(eventChan)

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/run/stream", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			var content interface{}
			if err := json.Unmarshal([]byte(data), &content); err != nil {
				content = data
			}

			eventChan <- Event{
				Type:    eventType,
				Content: content,
			}

			if eventType == "done" {
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}

// Health checks if the agent service is healthy.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status=%d", resp.StatusCode)
	}

	return nil
}

// WarmResponse represents a response from the warm endpoint.
type WarmResponse struct {
	Status    string `json:"status"`
	SandboxID string `json:"sandbox_id,omitempty"`
	Ready     bool   `json:"ready"`
	Message   string `json:"message,omitempty"`
}

// WarmSandbox pre-warms a sandbox for a user.
func (c *Client) WarmSandbox(ctx context.Context, req map[string]interface{}) (*WarmResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/warm", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Short timeout for warming - we don't wait for completion
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result WarmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// WarmSandboxStatus checks the status of a warm sandbox.
func (c *Client) WarmSandboxStatus(ctx context.Context, userID string) (*WarmResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/warm/status/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result WarmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
