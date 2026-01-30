package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MultiProvider wraps multiple MCP providers and routes requests appropriately
type MultiProvider struct {
	providers       map[string]Provider
	defaultProvider string
	mu              sync.RWMutex
}

// NewMultiProvider creates a new multi-provider
func NewMultiProvider(defaultProvider string) *MultiProvider {
	return &MultiProvider{
		providers:       make(map[string]Provider),
		defaultProvider: defaultProvider,
	}
}

// AddProvider adds a provider to the multi-provider
func (m *MultiProvider) AddProvider(name string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = provider
}

// GetProvider returns a specific provider by name
func (m *MultiProvider) GetProvider(name string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	return p, ok
}

// Name returns the multi-provider name
func (m *MultiProvider) Name() string {
	return "multi"
}

// parseAppProvider parses an app string that may include a provider prefix
// Format: "provider:app" or just "app" (uses default provider)
func (m *MultiProvider) parseAppProvider(app string) (provider, appSlug string) {
	if idx := strings.Index(app, ":"); idx > 0 {
		return app[:idx], app[idx+1:]
	}
	return m.defaultProvider, app
}

// ListTools lists tools from the appropriate provider
func (m *MultiProvider) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	providerName, appSlug := m.parseAppProvider(app)

	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()

	if !ok {
		// Fall back to default provider
		m.mu.RLock()
		provider, ok = m.providers[m.defaultProvider]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("no provider found for %s", providerName)
		}
		appSlug = app // Use original app name
	}

	tools, err := provider.ListTools(ctx, userID, appSlug)
	if err != nil {
		return nil, err
	}

	// Prefix tool names with provider for disambiguation when using multi-provider
	if len(m.providers) > 1 {
		for i := range tools {
			// Keep original name but add provider info to description
			tools[i].Description = fmt.Sprintf("[%s] %s", providerName, tools[i].Description)
		}
	}

	return tools, nil
}

// CallTool calls a tool on the appropriate provider
func (m *MultiProvider) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	providerName, appSlug := m.parseAppProvider(app)

	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()

	if !ok {
		// Fall back to default provider
		m.mu.RLock()
		provider, ok = m.providers[m.defaultProvider]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("no provider found for %s", providerName)
		}
		appSlug = app
	}

	return provider.CallTool(ctx, userID, appSlug, tool, input)
}

// GetConnectToken gets a connect token from the default provider
func (m *MultiProvider) GetConnectToken(ctx context.Context, userID string) (string, error) {
	m.mu.RLock()
	provider, ok := m.providers[m.defaultProvider]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("default provider %s not found", m.defaultProvider)
	}

	return provider.GetConnectToken(ctx, userID)
}

// GetConnectTokenForProvider gets a connect token from a specific provider
func (m *MultiProvider) GetConnectTokenForProvider(ctx context.Context, userID, providerName string) (string, error) {
	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("provider %s not found", providerName)
	}

	return provider.GetConnectToken(ctx, userID)
}

// ListConnectedApps lists connected apps from all providers
func (m *MultiProvider) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	m.mu.RLock()
	providers := make(map[string]Provider)
	for k, v := range m.providers {
		providers[k] = v
	}
	m.mu.RUnlock()

	var allApps []ConnectedApp
	var errs []string

	for name, provider := range providers {
		apps, err := provider.ListConnectedApps(ctx, userID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}
		// Prefix app names with provider for disambiguation
		for i := range apps {
			apps[i].Name = fmt.Sprintf("[%s] %s", name, apps[i].Name)
		}
		allApps = append(allApps, apps...)
	}

	if len(allApps) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("failed to list connected apps: %s", strings.Join(errs, "; "))
	}

	return allApps, nil
}

// ListProviders returns the names of all configured providers
func (m *MultiProvider) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}
