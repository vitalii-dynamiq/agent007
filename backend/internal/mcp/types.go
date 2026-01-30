package mcp

import (
	"context"
)

// ProviderType represents the type of MCP provider
type ProviderType string

const (
	ProviderTypePipedream ProviderType = "pipedream"
	ProviderTypeComposio  ProviderType = "composio"
	ProviderTypeDirect    ProviderType = "direct" // Direct MCP server (JSON-RPC)
)

// Tool represents an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	Content interface{} `json:"content"`
	IsError bool        `json:"isError"`
}

// ConnectedApp represents a connected app for a user
type ConnectedApp struct {
	App       string `json:"app"`
	AccountID string `json:"accountId,omitempty"`
	Name      string `json:"name"`
	Provider  string `json:"provider,omitempty"`
}

// ProviderInfo contains metadata about a provider
type ProviderInfo struct {
	Name        string       `json:"name"`
	Type        ProviderType `json:"type"`
	Description string       `json:"description,omitempty"`
	BaseURL     string       `json:"baseUrl,omitempty"`
	Apps        []string     `json:"apps,omitempty"` // List of supported apps (if known)
}

// Provider interface for MCP tool providers
type Provider interface {
	// Info returns metadata about the provider
	Info() ProviderInfo

	// Name returns the provider name
	Name() string

	// ListTools lists available tools for an app
	ListTools(ctx context.Context, userID, app string) ([]Tool, error)

	// CallTool calls a tool
	CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error)

	// GetConnectToken gets a token for connecting an account (optional - may return empty)
	GetConnectToken(ctx context.Context, userID string) (string, error)

	// ListConnectedApps lists apps connected by a user
	ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error)
}

// ProviderConfig is the configuration for creating a provider
type ProviderConfig struct {
	Type      ProviderType      `json:"type"`
	Name      string            `json:"name"`       // Unique identifier for this provider instance
	BaseURL   string            `json:"baseUrl"`    // For direct MCP servers
	APIKey    string            `json:"apiKey"`     // API key/token
	ProjectID string            `json:"projectId"`  // Project ID (Pipedream/Composio)
	Extra     map[string]string `json:"extra"`      // Provider-specific config
}

// ProxyRequest represents a request from the MCP CLI
type ProxyRequest struct {
	Method   string                 `json:"method"` // "list_tools", "call_tool", "list_apps"
	App      string                 `json:"app,omitempty"`
	Tool     string                 `json:"tool,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Provider string                 `json:"provider,omitempty"` // Optional: target specific provider
}

// ProxyResponse represents a response to the MCP CLI
type ProxyResponse struct {
	Success  bool        `json:"success"`
	Data     interface{} `json:"data,omitempty"`
	Error    string      `json:"error,omitempty"`
	Provider string      `json:"provider,omitempty"`
}
