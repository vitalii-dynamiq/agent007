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

const (
	pipedreamMCPURL   = "https://remote.mcp.pipedream.net"
	pipedreamTokenURL = "https://api.pipedream.com/v1/oauth/token"
	pipedreamAPIURL   = "https://api.pipedream.com/v1"
)

// PipedreamProvider implements the Provider interface for Pipedream
type PipedreamProvider struct {
	clientID     string
	clientSecret string
	projectID    string
	environment  string
	httpClient   *http.Client

	// Cached access token
	accessToken   string
	tokenExpiry   time.Time
}

// NewPipedreamProvider creates a new Pipedream provider
func NewPipedreamProvider(clientID, clientSecret, projectID, environment string) *PipedreamProvider {
	return &PipedreamProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		projectID:    projectID,
		environment:  environment,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Info returns provider metadata
func (p *PipedreamProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        "pipedream",
		Type:        ProviderTypePipedream,
		Description: "Pipedream MCP - Connect to 2000+ apps",
		BaseURL:     pipedreamMCPURL,
		Apps: []string{
			"gmail", "google_calendar", "slack", "notion",
			"github", "linear_app", "discord", "twitter",
		},
	}
}

// Name returns the provider name
func (p *PipedreamProvider) Name() string {
	return "pipedream"
}

// getAccessToken gets or refreshes the access token
func (p *PipedreamProvider) getAccessToken(ctx context.Context) (string, error) {
	// Check if we have a valid cached token
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	// Request new token
	reqBody := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", pipedreamTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// Cache the token with some buffer before expiry
	p.accessToken = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return p.accessToken, nil
}

// mcpRequest makes a JSON-RPC request to the MCP server
func (p *PipedreamProvider) mcpRequest(ctx context.Context, userID, app, method string, params interface{}) (json.RawMessage, error) {
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

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

	req, err := http.NewRequestWithContext(ctx, "POST", pipedreamMCPURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Determine environment - default to "production" if not set
	env := p.environment
	if env == "" {
		env = "production"
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("x-pd-project-id", p.projectID)
	req.Header.Set("x-pd-environment", env)
	req.Header.Set("x-pd-external-user-id", userID)
	if app != "" {
		req.Header.Set("x-pd-app-slug", app)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// Check if response is SSE format and parse it
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		return p.parseSSEResponse(respBody)
	}

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
func (p *PipedreamProvider) parseSSEResponse(body []byte) (json.RawMessage, error) {
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
				continue // Try next line
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

// ListTools lists available tools for an app
func (p *PipedreamProvider) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	result, err := p.mcpRequest(ctx, userID, app, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var toolsResp struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(result, &toolsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tools: %w", err)
	}

	tools := make([]Tool, len(toolsResp.Tools))
	for i, t := range toolsResp.Tools {
		tools[i] = Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	return tools, nil
}

// CallTool calls a tool
func (p *PipedreamProvider) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"name":      tool,
		"arguments": input,
	}

	result, err := p.mcpRequest(ctx, userID, app, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var callResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(result, &callResp); err != nil {
		return nil, fmt.Errorf("failed to decode tool result: %w", err)
	}

	// Extract text content
	var content interface{}
	if len(callResp.Content) > 0 {
		if len(callResp.Content) == 1 {
			content = callResp.Content[0].Text
		} else {
			texts := make([]string, len(callResp.Content))
			for i, c := range callResp.Content {
				texts[i] = c.Text
			}
			content = texts
		}
	}

	return &ToolResult{
		Content: content,
		IsError: callResp.IsError,
	}, nil
}

// ConnectTokenResponse contains the parsed connect token response
type ConnectTokenResponse struct {
	Token          string `json:"token"`
	ConnectLinkURL string `json:"connect_link_url"`
	ExpiresAt      string `json:"expires_at"`
}

// GetConnectToken gets a token for connecting an account
func (p *PipedreamProvider) GetConnectToken(ctx context.Context, userID string) (string, error) {
	resp, err := p.GetConnectTokenFull(ctx, userID)
	if err != nil {
		return "", err
	}
	
	// Return in format: token|connect_link_url|expires_at
	if resp.ConnectLinkURL != "" {
		return resp.Token + "|" + resp.ConnectLinkURL + "|" + resp.ExpiresAt, nil
	}
	return resp.Token, nil
}

// GetConnectTokenFull gets a token with full response data
func (p *PipedreamProvider) GetConnectTokenFull(ctx context.Context, userID string) (*ConnectTokenResponse, error) {
	return p.GetConnectTokenWithRedirects(ctx, userID, "", "")
}

// GetConnectTokenWithRedirects gets a token with optional redirect URIs for OAuth completion
func (p *PipedreamProvider) GetConnectTokenWithRedirects(ctx context.Context, userID, successRedirectURI, errorRedirectURI string) (*ConnectTokenResponse, error) {
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Determine environment - default to "development" if not set
	env := p.environment
	if env == "" {
		env = "development"
	}

	reqBody := map[string]interface{}{
		"external_user_id": userID,
	}
	
	// Add redirect URIs if provided (for Connect Link flow)
	if successRedirectURI != "" {
		reqBody["success_redirect_uri"] = successRedirectURI
	}
	if errorRedirectURI != "" {
		reqBody["error_redirect_uri"] = errorRedirectURI
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/connect/%s/tokens", pipedreamAPIURL, p.projectID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-PD-Environment", env)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get connect token: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var tokenResp ConnectTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// ListConnectedApps lists apps connected by a user
func (p *PipedreamProvider) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Determine environment - default to "production" if not set
	env := p.environment
	if env == "" {
		env = "production"
	}

	// Correct Pipedream API: GET /v1/connect/{project_id}/accounts?external_user_id={userID}
	url := fmt.Sprintf("%s/connect/%s/accounts?external_user_id=%s", pipedreamAPIURL, p.projectID, userID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-PD-Environment", env)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list connected apps: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var appsResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			App  struct {
				NameSlug string `json:"name_slug"`
				Name     string `json:"name"`
			} `json:"app"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&appsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	apps := make([]ConnectedApp, len(appsResp.Data))
	for i, a := range appsResp.Data {
		apps[i] = ConnectedApp{
			App:       a.App.NameSlug,
			AccountID: a.ID,
			Name:      a.Name,
		}
	}

	return apps, nil
}
