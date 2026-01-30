// Package mcp provides a unified interface for MCP (Model Context Protocol) providers.
//
// The package supports multiple provider types:
//   - Pipedream: Cloud-based MCP with 2000+ app integrations
//   - Composio: 300+ app integrations with comprehensive API coverage
//   - Direct: Standard MCP servers using JSON-RPC 2.0 protocol
//
// Use the Registry to manage multiple providers:
//
//	registry := mcp.NewRegistry()
//	registry.CreateProvider(mcp.ProviderConfig{
//	    Type:      mcp.ProviderTypePipedream,
//	    Name:      "pipedream",
//	    ProjectID: "proj_xxx",
//	    Extra:     map[string]string{"clientId": "...", "clientSecret": "..."},
//	})
//	registry.SetDefaultProvider("pipedream")
//
// Tools can be accessed using "provider:app" syntax:
//   - "gmail" uses the default provider
//   - "pipedream:gmail" explicitly uses Pipedream
//   - "composio:github" explicitly uses Composio
package mcp

// Note: Types are defined in types.go
// - Tool, ToolResult, ConnectedApp
// - Provider interface
// - ProviderConfig, ProviderInfo
// - ProxyRequest, ProxyResponse
