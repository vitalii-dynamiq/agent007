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
	composioAPIURL = "https://backend.composio.dev/api/v3"
)

// ComposioProvider implements the Provider interface for Composio
type ComposioProvider struct {
	apiKey     string
	projectID  string
	authConfigIDs map[string]string
	httpClient *http.Client
}

// NewComposioProvider creates a new Composio provider
func NewComposioProvider(apiKey, projectID string, authConfigIDs map[string]string) *ComposioProvider {
	return &ComposioProvider{
		apiKey:    apiKey,
		projectID: projectID,
		authConfigIDs: authConfigIDs,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Info returns provider metadata
func (c *ComposioProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        "composio",
		Type:        ProviderTypeComposio,
		Description: "Composio - 300+ app integrations with comprehensive API coverage",
		BaseURL:     composioAPIURL,
		Apps: []string{
			"gmail", "googlecalendar", "slack", "notion",
			"github", "linear", "jira", "asana", "trello",
			"hubspot", "airtable", "discord", "zoom",
		},
	}
}

// Name returns the provider name
func (c *ComposioProvider) Name() string {
	return "composio"
}

// ListTools lists available tools for an app/toolkit
func (c *ComposioProvider) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	// Map common app names to Composio toolkit slugs
	toolkitSlug := mapToComposioToolkit(app)

	url := fmt.Sprintf("%s/tools?toolkit_slug=%s&limit=100", composioAPIURL, toolkitSlug)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list tools: status=%d body=%s", resp.StatusCode, string(body))
	}

	var toolsResp struct {
		Items []struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Parameters  struct {
				Properties map[string]interface{} `json:"properties"`
				Required   []string               `json:"required"`
			} `json:"parameters"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &toolsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tools := make([]Tool, len(toolsResp.Items))
	for i, t := range toolsResp.Items {
		tools[i] = Tool{
			Name:        t.Slug,
			Description: t.Description,
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": t.Parameters.Properties,
				"required":   t.Parameters.Required,
			},
		}
	}

	return tools, nil
}

// CallTool calls a specific tool
func (c *ComposioProvider) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	// Execute the tool
	url := fmt.Sprintf("%s/tools/execute/%s", composioAPIURL, tool)

	reqBody := map[string]interface{}{
		"arguments": input,
		"user_id":   userID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to call tool: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var execResp struct {
		Data       interface{} `json:"data"`
		Successful bool        `json:"successful"`
		Error      string      `json:"error"`
	}

	if err := json.Unmarshal(respBody, &execResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !execResp.Successful || execResp.Error != "" {
		return &ToolResult{
			Content: execResp.Error,
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: execResp.Data,
		IsError: false,
	}, nil
}

// GetConnectToken gets a token for connecting an account (legacy fallback).
func (c *ComposioProvider) GetConnectToken(ctx context.Context, userID string) (string, error) {
	// Legacy flow - generate an integration URL
	url := fmt.Sprintf("%s/auth/session", composioAPIURL)

	reqBody := map[string]interface{}{
		"user_id":    userID,
		"project_id": c.projectID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to get connect token: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirectUrl"`
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Token != "" {
		return tokenResp.Token, nil
	}
	return tokenResp.RedirectURL, nil
}

// GetConnectLink creates a Composio Connect Link for a toolkit.
// Uses latest auth_config + connected_accounts flow with fallback to legacy auth/session.
func (c *ComposioProvider) GetConnectLink(ctx context.Context, userID, toolkitSlug, callbackURL string, connectionData map[string]interface{}) (string, error) {
	toolkitSlug = mapToComposioToolkit(toolkitSlug)

	authConfigID, err := c.getAuthConfigID(ctx, toolkitSlug)
	if err != nil {
		return "", err
	}

	connection := map[string]interface{}{
		"user_id": userID,
	}
	if callbackURL != "" {
		connection["callback_url"] = callbackURL
		connection["redirect_uri"] = callbackURL // Also set redirect_uri for compatibility
	}
	// Connection data (e.g. subdomain for Atlassian) goes in the "data" field
	if len(connectionData) > 0 {
		connection["data"] = connectionData
	}

	payload := map[string]interface{}{
		"auth_config": map[string]interface{}{
			"id": authConfigID,
		},
		"connection": connection,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", composioAPIURL+"/connected_accounts", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create connect link: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var connectResp struct {
		RedirectURL string `json:"redirect_url"`
		RedirectURI string `json:"redirect_uri"`
		RedirectUrl string `json:"redirectUrl"`
	}
	if err := json.Unmarshal(respBody, &connectResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if connectResp.RedirectURL != "" {
		return connectResp.RedirectURL, nil
	}
	if connectResp.RedirectURI != "" {
		return connectResp.RedirectURI, nil
	}
	if connectResp.RedirectUrl != "" {
		return connectResp.RedirectUrl, nil
	}
	return "", fmt.Errorf("no redirect url returned from composio")
}

func (c *ComposioProvider) getAuthConfigID(ctx context.Context, toolkitSlug string) (string, error) {
	if c.authConfigIDs != nil {
		key := strings.ToLower(toolkitSlug)
		if id, ok := c.authConfigIDs[key]; ok && id != "" {
			return id, nil
		}
		if id, ok := c.authConfigIDs["default"]; ok && id != "" {
			return id, nil
		}
	}
	url := fmt.Sprintf("%s/auth_configs?toolkit_slug=%s&limit=1", composioAPIURL, toolkitSlug)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list auth configs: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var configs struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(respBody, &configs); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	if len(configs.Items) > 0 && configs.Items[0].ID != "" {
		return configs.Items[0].ID, nil
	}

	// If none exist, create a Composio-managed auth config on demand
	return c.createAuthConfig(ctx, toolkitSlug)
}

func (c *ComposioProvider) createAuthConfig(ctx context.Context, toolkitSlug string) (string, error) {
	// Use Composio-managed auth only
	return c.createAuthConfigWithOptions(ctx, toolkitSlug, map[string]interface{}{
		"type": "use_composio_managed_auth",
	})
}

func (c *ComposioProvider) createAuthConfigWithOptions(ctx context.Context, toolkitSlug string, authConfig map[string]interface{}) (string, error) {
	payload := map[string]interface{}{
		"toolkit": map[string]string{
			"slug": toolkitSlug,
		},
		"auth_config": authConfig,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", composioAPIURL+"/auth_configs", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create auth config: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	if id := extractAuthConfigID(respBody); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("no auth config id returned for %s", toolkitSlug)
}

func extractAuthConfigID(respBody []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return ""
	}
	if ac, ok := payload["auth_config"].(map[string]interface{}); ok {
		if id, ok := ac["id"].(string); ok {
			return id
		}
	}
	if id, ok := payload["id"].(string); ok {
		return id
	}
	return ""
}

// ListConnectedApps lists apps connected by a user
func (c *ComposioProvider) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	url := fmt.Sprintf("%s/connected_accounts?user_id=%s", composioAPIURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list connected apps: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var appsResp struct {
		Items []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			UserID  string `json:"user_id"`
			Toolkit struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"toolkit"`
		} `json:"items"`
	}

	if err := json.Unmarshal(respBody, &appsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	apps := make([]ConnectedApp, 0, len(appsResp.Items))
	for _, a := range appsResp.Items {
		if a.Status == "ACTIVE" || a.Status == "active" {
			name := a.Toolkit.Name
			if name == "" {
				name = a.Toolkit.Slug
			}
			apps = append(apps, ConnectedApp{
				AccountID: a.ID,
				App:       a.Toolkit.Slug,
				Name:      name,
			})
		}
	}

	return apps, nil
}

// mapToComposioToolkit maps common app names to Composio toolkit slugs
func mapToComposioToolkit(app string) string {
	app = strings.ToLower(app)
	
	// Composio uses uppercase toolkit names
	mappings := map[string]string{
		"gmail":           "gmail",
		"google_calendar": "googlecalendar",
		"googlecalendar":  "googlecalendar",
		"slack":           "slack",
		"notion":          "notion",
		"github":          "github",
		"linear":          "linear",
		"linear_app":      "linear",
		"discord":         "discord",
		"twitter":         "twitter",
		"airtable":        "airtable",
		"hubspot":         "hubspot",
		"jira":            "jira",
		"asana":           "asana",
		"trello":          "trello",
		"google_drive":    "googledrive",
		"googledrive":     "googledrive",
		"dropbox":         "dropbox",
		"zoom":            "zoom",
	}

	if mapped, ok := mappings[app]; ok {
		return mapped
	}
	return app
}
