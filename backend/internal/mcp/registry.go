package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ProviderFactory is a function that creates a provider from config
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

// Registry manages MCP providers
type Registry struct {
	providers       map[string]Provider
	factories       map[ProviderType]ProviderFactory
	defaultProvider string
	mu              sync.RWMutex
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
		factories: make(map[ProviderType]ProviderFactory),
	}

	// Register built-in provider factories
	r.RegisterFactory(ProviderTypePipedream, func(cfg ProviderConfig) (Provider, error) {
		clientID := cfg.Extra["clientId"]
		clientSecret := cfg.Extra["clientSecret"]
		env := cfg.Extra["environment"]
		if env == "" {
			env = "development"
		}
		return NewPipedreamProvider(clientID, clientSecret, cfg.ProjectID, env), nil
	})

	r.RegisterFactory(ProviderTypeComposio, func(cfg ProviderConfig) (Provider, error) {
		var authConfigIDs map[string]string
		if raw, ok := cfg.Extra["authConfigIds"]; ok && raw != "" {
			_ = json.Unmarshal([]byte(raw), &authConfigIDs)
		}
		return NewComposioProvider(cfg.APIKey, cfg.ProjectID, authConfigIDs), nil
	})

	r.RegisterFactory(ProviderTypeDirect, func(cfg ProviderConfig) (Provider, error) {
		return NewDirectMCPProvider(cfg.Name, cfg.BaseURL, cfg.APIKey), nil
	})

	return r
}

// RegisterFactory registers a factory for a provider type
func (r *Registry) RegisterFactory(providerType ProviderType, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[providerType] = factory
}

// CreateProvider creates and registers a provider from config
func (r *Registry) CreateProvider(cfg ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	factory, ok := r.factories[cfg.Type]
	if !ok {
		return fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	provider, err := factory(cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider %s: %w", cfg.Name, err)
	}

	r.providers[cfg.Name] = provider
	return nil
}

// AddProvider adds a pre-created provider
func (r *Registry) AddProvider(name string, provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// GetProvider returns a provider by name
func (r *Registry) GetProvider(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// SetDefaultProvider sets the default provider
func (r *Registry) SetDefaultProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider %s not found", name)
	}
	r.defaultProvider = name
	return nil
}

// GetDefaultProvider returns the default provider name
func (r *Registry) GetDefaultProvider() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultProvider
}

// ListProviders returns info about all registered providers
func (r *Registry) ListProviders() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ProviderInfo, 0, len(r.providers))
	for _, p := range r.providers {
		infos = append(infos, p.Info())
	}

	// Sort by name for consistent ordering
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// ProviderNames returns the names of all registered providers
func (r *Registry) ProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseProviderApp parses "provider:app" or "app" format
func (r *Registry) ParseProviderApp(app string) (providerName, appSlug string) {
	if idx := strings.Index(app, ":"); idx > 0 {
		return app[:idx], app[idx+1:]
	}
	return r.defaultProvider, app
}

// --- Provider interface implementation for Registry ---
// This allows Registry to be used as a Provider itself

func (r *Registry) Info() ProviderInfo {
	return ProviderInfo{
		Name:        "registry",
		Type:        "multi",
		Description: "Multi-provider registry",
	}
}

func (r *Registry) Name() string {
	return "registry"
}

func (r *Registry) ListTools(ctx context.Context, userID, app string) ([]Tool, error) {
	providerName, appSlug := r.ParseProviderApp(app)

	r.mu.RLock()
	provider, ok := r.providers[providerName]
	if !ok {
		provider, ok = r.providers[r.defaultProvider]
		if !ok {
			r.mu.RUnlock()
			return nil, fmt.Errorf("no provider found: %s", providerName)
		}
		providerName = r.defaultProvider
	}
	r.mu.RUnlock()

	tools, err := provider.ListTools(ctx, userID, appSlug)
	if err != nil {
		return nil, err
	}

	// Add provider info to tool descriptions for clarity
	for i := range tools {
		tools[i].Description = fmt.Sprintf("[%s] %s", providerName, tools[i].Description)
	}

	return tools, nil
}

func (r *Registry) CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error) {
	providerName, appSlug := r.ParseProviderApp(app)

	r.mu.RLock()
	provider, ok := r.providers[providerName]
	if !ok {
		provider, ok = r.providers[r.defaultProvider]
		if !ok {
			r.mu.RUnlock()
			return nil, fmt.Errorf("no provider found: %s", providerName)
		}
		providerName = r.defaultProvider
	}
	r.mu.RUnlock()

	return provider.CallTool(ctx, userID, appSlug, tool, input)
}

func (r *Registry) GetConnectToken(ctx context.Context, userID string) (string, error) {
	r.mu.RLock()
	provider, ok := r.providers[r.defaultProvider]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("default provider not configured")
	}

	return provider.GetConnectToken(ctx, userID)
}

func (r *Registry) GetConnectTokenForProvider(ctx context.Context, userID, providerName string) (string, error) {
	r.mu.RLock()
	provider, ok := r.providers[providerName]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("provider %s not found", providerName)
	}

	return provider.GetConnectToken(ctx, userID)
}

func (r *Registry) ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error) {
	r.mu.RLock()
	providers := make(map[string]Provider)
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	var allApps []ConnectedApp
	var errors []string

	for name, provider := range providers {
		apps, err := provider.ListConnectedApps(ctx, userID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		for i := range apps {
			apps[i].Provider = name
			apps[i].Name = fmt.Sprintf("[%s] %s", name, apps[i].Name)
		}
		allApps = append(allApps, apps...)
	}

	if len(allApps) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to list apps: %s", strings.Join(errors, "; "))
	}

	return allApps, nil
}
