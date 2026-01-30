package mcp

import (
	"context"
	"testing"
)

// MockProvider implements Provider for testing
type MockProvider struct {
	name  string
	tools []Tool
	apps  []ConnectedApp
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		tools: []Tool{
			{Name: "mock-tool-1", Description: "Mock tool 1"},
			{Name: "mock-tool-2", Description: "Mock tool 2"},
		},
		apps: []ConnectedApp{
			{App: "mock-app", Name: "Mock App", Provider: name},
		},
	}
}

func (m *MockProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        m.name,
		Type:        "mock",
		Description: "Mock provider for testing",
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	return m.tools, nil
}

func (m *MockProvider) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	return &ToolResult{
		Content: map[string]interface{}{
			"provider": m.name,
			"app":      app,
			"tool":     tool,
			"input":    input,
		},
		IsError: false,
	}, nil
}

func (m *MockProvider) GetConnectToken(ctx context.Context, userID string) (string, error) {
	return "mock-token-" + m.name, nil
}

func (m *MockProvider) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	return m.apps, nil
}

func TestRegistryBasics(t *testing.T) {
	registry := NewRegistry()

	// Test adding providers
	mock1 := NewMockProvider("mock1")
	mock2 := NewMockProvider("mock2")

	registry.AddProvider("mock1", mock1)
	registry.AddProvider("mock2", mock2)

	// Test listing providers
	names := registry.ProviderNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(names))
	}

	// Test getting provider
	p, ok := registry.GetProvider("mock1")
	if !ok {
		t.Error("Expected to find mock1 provider")
	}
	if p.Name() != "mock1" {
		t.Errorf("Expected mock1, got %s", p.Name())
	}

	// Test non-existent provider
	_, ok = registry.GetProvider("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent provider")
	}
}

func TestRegistryDefaultProvider(t *testing.T) {
	registry := NewRegistry()

	mock1 := NewMockProvider("mock1")
	mock2 := NewMockProvider("mock2")

	registry.AddProvider("mock1", mock1)
	registry.AddProvider("mock2", mock2)

	// Set default
	err := registry.SetDefaultProvider("mock1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if registry.GetDefaultProvider() != "mock1" {
		t.Error("Expected mock1 as default")
	}

	// Try to set non-existent default
	err = registry.SetDefaultProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestRegistryParseProviderApp(t *testing.T) {
	registry := NewRegistry()
	registry.AddProvider("mock1", NewMockProvider("mock1"))
	registry.AddProvider("mock2", NewMockProvider("mock2"))
	registry.SetDefaultProvider("mock1")

	tests := []struct {
		input            string
		expectedProvider string
		expectedApp      string
	}{
		{"gmail", "mock1", "gmail"},           // Uses default
		{"mock1:gmail", "mock1", "gmail"},     // Explicit provider
		{"mock2:github", "mock2", "github"},   // Different provider
		{"composio:slack", "composio", "slack"}, // Provider not in registry (will fallback)
	}

	for _, tc := range tests {
		provider, app := registry.ParseProviderApp(tc.input)
		if provider != tc.expectedProvider {
			t.Errorf("For %s: expected provider %s, got %s", tc.input, tc.expectedProvider, provider)
		}
		if app != tc.expectedApp {
			t.Errorf("For %s: expected app %s, got %s", tc.input, tc.expectedApp, app)
		}
	}
}

func TestRegistryListTools(t *testing.T) {
	registry := NewRegistry()
	registry.AddProvider("mock1", NewMockProvider("mock1"))
	registry.AddProvider("mock2", NewMockProvider("mock2"))
	registry.SetDefaultProvider("mock1")

	ctx := context.Background()

	// Test default provider
	tools, err := registry.ListTools(ctx, "user1", "gmail")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Test explicit provider
	tools, err = registry.ListTools(ctx, "user1", "mock2:gmail")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestRegistryCallTool(t *testing.T) {
	registry := NewRegistry()
	registry.AddProvider("mock1", NewMockProvider("mock1"))
	registry.AddProvider("mock2", NewMockProvider("mock2"))
	registry.SetDefaultProvider("mock1")

	ctx := context.Background()

	// Test calling tool via default provider
	result, err := registry.CallTool(ctx, "user1", "gmail", "send-email", map[string]interface{}{
		"to": "test@example.com",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result")
	}

	content := result.Content.(map[string]interface{})
	if content["provider"] != "mock1" {
		t.Errorf("Expected mock1 provider, got %s", content["provider"])
	}

	// Test calling tool via explicit provider
	result, err = registry.CallTool(ctx, "user1", "mock2:github", "create-issue", map[string]interface{}{
		"title": "Test",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	content = result.Content.(map[string]interface{})
	if content["provider"] != "mock2" {
		t.Errorf("Expected mock2 provider, got %s", content["provider"])
	}
}

func TestRegistryListConnectedApps(t *testing.T) {
	registry := NewRegistry()
	registry.AddProvider("mock1", NewMockProvider("mock1"))
	registry.AddProvider("mock2", NewMockProvider("mock2"))
	registry.SetDefaultProvider("mock1")

	ctx := context.Background()

	apps, err := registry.ListConnectedApps(ctx, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should have apps from both providers
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps (1 from each provider), got %d", len(apps))
	}
}

func TestRegistryGetConnectToken(t *testing.T) {
	registry := NewRegistry()
	registry.AddProvider("mock1", NewMockProvider("mock1"))
	registry.AddProvider("mock2", NewMockProvider("mock2"))
	registry.SetDefaultProvider("mock1")

	ctx := context.Background()

	// Test default provider token
	token, err := registry.GetConnectToken(ctx, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token != "mock-token-mock1" {
		t.Errorf("Expected mock-token-mock1, got %s", token)
	}

	// Test specific provider token
	token, err = registry.GetConnectTokenForProvider(ctx, "user1", "mock2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token != "mock-token-mock2" {
		t.Errorf("Expected mock-token-mock2, got %s", token)
	}
}

func TestProviderConfig(t *testing.T) {
	registry := NewRegistry()

	// Test creating Pipedream provider via config
	cfg := ProviderConfig{
		Type:      ProviderTypePipedream,
		Name:      "test-pipedream",
		ProjectID: "proj_test",
		Extra: map[string]string{
			"clientId":     "test-id",
			"clientSecret": "test-secret",
			"environment":  "test",
		},
	}

	err := registry.CreateProvider(cfg)
	if err != nil {
		t.Errorf("Failed to create Pipedream provider: %v", err)
	}

	p, ok := registry.GetProvider("test-pipedream")
	if !ok {
		t.Error("Expected to find test-pipedream provider")
	}
	if p.Name() != "pipedream" {
		t.Errorf("Expected pipedream, got %s", p.Name())
	}

	// Test creating Composio provider via config
	cfg = ProviderConfig{
		Type:      ProviderTypeComposio,
		Name:      "test-composio",
		APIKey:    "test-key",
		ProjectID: "test-project",
	}

	err = registry.CreateProvider(cfg)
	if err != nil {
		t.Errorf("Failed to create Composio provider: %v", err)
	}

	p, ok = registry.GetProvider("test-composio")
	if !ok {
		t.Error("Expected to find test-composio provider")
	}

	// Test creating Direct MCP provider via config
	cfg = ProviderConfig{
		Type:    ProviderTypeDirect,
		Name:    "test-direct",
		BaseURL: "http://localhost:3000/mcp",
		APIKey:  "test-key",
	}

	err = registry.CreateProvider(cfg)
	if err != nil {
		t.Errorf("Failed to create Direct MCP provider: %v", err)
	}

	p, ok = registry.GetProvider("test-direct")
	if !ok {
		t.Error("Expected to find test-direct provider")
	}
}

func TestUnknownProviderType(t *testing.T) {
	registry := NewRegistry()

	cfg := ProviderConfig{
		Type: "unknown",
		Name: "test-unknown",
	}

	err := registry.CreateProvider(cfg)
	if err == nil {
		t.Error("Expected error for unknown provider type")
	}
}
