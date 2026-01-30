package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DirectMCPProvider implements the Provider interface for standard MCP servers
// that follow the JSON-RPC 2.0 protocol over HTTP/SSE
type DirectMCPProvider struct {
	name       string
	baseURL    string
	apiKey     string
	httpClient *http.Client
	// tokenProvider supplies per-user access tokens (OAuth2)
	tokenProvider func(ctx context.Context, userID string) (string, error)
}

// NewDirectMCPProvider creates a new direct MCP provider
func NewDirectMCPProvider(name, baseURL, apiKey string) *DirectMCPProvider {
	// Ensure URL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &DirectMCPProvider{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetTokenProvider sets a per-user token provider for OAuth2 flows.
func (d *DirectMCPProvider) SetTokenProvider(provider func(ctx context.Context, userID string) (string, error)) {
	d.tokenProvider = provider
}

// Info returns provider metadata
func (d *DirectMCPProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        d.name,
		Type:        ProviderTypeDirect,
		Description: fmt.Sprintf("Direct MCP server at %s", d.baseURL),
		BaseURL:     d.baseURL,
	}
}

// Name returns the provider name
func (d *DirectMCPProvider) Name() string {
	return d.name
}

// jsonRPCRequest makes a JSON-RPC 2.0 request to the MCP server
func (d *DirectMCPProvider) jsonRPCRequest(ctx context.Context, userID, method string, params interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if params != nil {
		reqBody["params"] = params
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	token := d.apiKey
	if d.tokenProvider != nil {
		var err error
		token, err = d.tokenProvider(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// Handle SSE response format
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		return d.parseSSEResponse(respBody)
	}

	// Standard JSON-RPC response
	var jsonRPCResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &jsonRPCResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if jsonRPCResp.Error != nil {
		return nil, fmt.Errorf("MCP error: code=%d message=%s", jsonRPCResp.Error.Code, jsonRPCResp.Error.Message)
	}

	return jsonRPCResp.Result, nil
}

// parseSSEResponse parses an SSE response to extract JSON-RPC result
func (d *DirectMCPProvider) parseSSEResponse(body []byte) (json.RawMessage, error) {
	lines := strings.Split(string(body), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "" || data == "[DONE]" {
				continue
			}

			var jsonRPCResp struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(data), &jsonRPCResp); err != nil {
				continue
			}

			if jsonRPCResp.Error != nil {
				return nil, fmt.Errorf("MCP error: code=%d message=%s", jsonRPCResp.Error.Code, jsonRPCResp.Error.Message)
			}

			if jsonRPCResp.Result != nil {
				return jsonRPCResp.Result, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid JSON-RPC response found in SSE stream")
}

// ListTools lists available tools from the MCP server
func (d *DirectMCPProvider) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	result, err := d.jsonRPCRequest(ctx, userID, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var listResp struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(result, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	tools := make([]Tool, len(listResp.Tools))
	for i, t := range listResp.Tools {
		tools[i] = Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	return tools, nil
}

// CallTool calls a tool on the MCP server
func (d *DirectMCPProvider) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"name":      tool,
		"arguments": input,
	}

	result, err := d.jsonRPCRequest(ctx, userID, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var callResp struct {
		Content []struct {
			Type string      `json:"type"`
			Text string      `json:"text,omitempty"`
			Data interface{} `json:"data,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(result, &callResp); err != nil {
		return nil, fmt.Errorf("failed to parse call response: %w", err)
	}

	// Extract content
	var content interface{}
	if len(callResp.Content) == 1 {
		if callResp.Content[0].Text != "" {
			content = callResp.Content[0].Text
		} else {
			content = callResp.Content[0].Data
		}
	} else if len(callResp.Content) > 1 {
		texts := make([]string, 0, len(callResp.Content))
		for _, c := range callResp.Content {
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			content = texts
		} else {
			content = callResp.Content
		}
	}

	return &ToolResult{
		Content: content,
		IsError: callResp.IsError,
	}, nil
}

// GetConnectToken is not supported for direct MCP servers
func (d *DirectMCPProvider) GetConnectToken(ctx context.Context, userID string) (string, error) {
	return "", fmt.Errorf("direct MCP servers don't support connect tokens")
}

// ListConnectedApps returns the single "app" that this direct server provides
func (d *DirectMCPProvider) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	// Direct MCP servers are typically single-purpose
	return []ConnectedApp{
		{
			App:      d.name,
			Name:     d.name,
			Provider: d.name,
		},
	}, nil
}
