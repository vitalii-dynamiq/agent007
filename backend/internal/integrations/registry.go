package integrations

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Registry manages user integrations and generates agent context
type Registry struct {
	// User integrations: userID -> integrationID -> UserIntegration
	userIntegrations map[string]map[string]*UserIntegration
	mu               sync.RWMutex

	// OAuth2 handlers for different providers
	oauth2Handlers map[string]OAuth2Handler

	// Encryption key for storing credentials
	encryptionKey string

	// SQLite store for persistence (optional)
	store *SQLiteStore
}

// OAuth2Handler handles OAuth2 flows for a service
type OAuth2Handler interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*OAuth2Token, error)
	RefreshToken(ctx context.Context, refreshToken string) (*OAuth2Token, error)
}

// NewRegistry creates a new integration registry (in-memory only)
func NewRegistry(encryptionKey string) *Registry {
	return &Registry{
		userIntegrations: make(map[string]map[string]*UserIntegration),
		oauth2Handlers:   make(map[string]OAuth2Handler),
		encryptionKey:    encryptionKey,
	}
}

// NewRegistryWithStore creates a new integration registry with SQLite persistence
func NewRegistryWithStore(encryptionKey string, dataDir string) (*Registry, error) {
	store, err := NewSQLiteStore(dataDir, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	// Load existing integrations from the database
	userIntegrations := store.GetAllUserIntegrations()

	count := 0
	for _, integrations := range userIntegrations {
		count += len(integrations)
	}
	log.Printf("Loaded %d user integrations from SQLite store", count)

	return &Registry{
		userIntegrations: userIntegrations,
		oauth2Handlers:   make(map[string]OAuth2Handler),
		encryptionKey:    encryptionKey,
		store:            store,
	}, nil
}

// Close closes the registry and its underlying store
func (r *Registry) Close() error {
	if r.store != nil {
		return r.store.Close()
	}
	return nil
}

// RegisterOAuth2Handler registers an OAuth2 handler for an integration
func (r *Registry) RegisterOAuth2Handler(integrationID string, handler OAuth2Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.oauth2Handlers[integrationID] = handler
}

// ConnectIntegration connects an integration for a user
func (r *Registry) ConnectIntegration(userID, integrationID string, ui *UserIntegration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := Catalog[integrationID]; !ok {
		return fmt.Errorf("unknown integration: %s", integrationID)
	}

	if r.userIntegrations[userID] == nil {
		r.userIntegrations[userID] = make(map[string]*UserIntegration)
	}

	ui.UserID = userID
	ui.IntegrationID = integrationID
	ui.ConnectedAt = time.Now()
	ui.Enabled = true

	r.userIntegrations[userID][integrationID] = ui

	// Persist to SQLite store if available
	if r.store != nil {
		if err := r.store.SaveUserIntegration(ui); err != nil {
			log.Printf("Warning: Failed to persist integration to SQLite: %v", err)
			// Don't fail the operation, just log the warning
		}
	}

	return nil
}

// DisconnectIntegration disconnects an integration for a user
func (r *Registry) DisconnectIntegration(userID, integrationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.userIntegrations[userID] == nil {
		return fmt.Errorf("no integrations for user %s", userID)
	}

	delete(r.userIntegrations[userID], integrationID)

	// Delete from SQLite store if available
	if r.store != nil {
		if err := r.store.DeleteUserIntegration(userID, integrationID); err != nil {
			log.Printf("Warning: Failed to delete integration from SQLite: %v", err)
			// Don't fail the operation, just log the warning
		}
	}

	return nil
}

// GetUserIntegration returns a user's integration
func (r *Registry) GetUserIntegration(userID, integrationID string) (*UserIntegration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userIntegrations[userID] == nil {
		return nil, false
	}

	ui, ok := r.userIntegrations[userID][integrationID]
	return ui, ok
}

// ListUserIntegrations returns all integrations for a user
func (r *Registry) ListUserIntegrations(userID string) []*UserIntegration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userIntegrations[userID] == nil {
		return nil
	}

	result := make([]*UserIntegration, 0, len(r.userIntegrations[userID]))
	for _, ui := range r.userIntegrations[userID] {
		result = append(result, ui)
	}
	return result
}

// GetEnabledIntegrationsForUser returns enabled integrations for a user
func (r *Registry) GetEnabledIntegrationsForUser(userID string) []*Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userIntegrations[userID] == nil {
		return nil
	}

	var result []*Integration
	for integrationID, ui := range r.userIntegrations[userID] {
		if ui.Enabled {
			if integration, ok := Catalog[integrationID]; ok {
				result = append(result, integration)
			}
		}
	}
	return result
}

// GenerateAgentContext generates context for the agent based on user's enabled integrations
func (r *Registry) GenerateAgentContext(userID string) *AgentContext {
	integrations := r.GetEnabledIntegrationsForUser(userID)

	ctx := &AgentContext{
		MCPTools:  make([]IntegrationInfo, 0),
		CLITools:  make([]IntegrationInfo, 0),
		CloudCLIs: make([]IntegrationInfo, 0),
		APITools:  make([]IntegrationInfo, 0),
		DirectMCP: make([]IntegrationInfo, 0),
	}

	for _, i := range integrations {
		info := IntegrationInfo{
			ID:           i.ID,
			Name:         i.Name,
			CLICommand:   i.CLICommand,
			Instructions: i.AgentInstructions,
			Capabilities: i.Capabilities,
		}

		switch i.ProviderType {
		case ProviderMCP:
			ctx.MCPTools = append(ctx.MCPTools, info)
		case ProviderCLI:
			ctx.CLITools = append(ctx.CLITools, info)
		case ProviderCloudCLI:
			ctx.CloudCLIs = append(ctx.CloudCLIs, info)
		case ProviderAPI:
			ctx.APITools = append(ctx.APITools, info)
		case ProviderDirectMCP:
			ctx.DirectMCP = append(ctx.DirectMCP, info)
		}
	}

	ctx.SystemPromptAddition = r.generateSystemPrompt(ctx)
	return ctx
}

// generateSystemPrompt generates the system prompt addition for the agent
func (r *Registry) generateSystemPrompt(ctx *AgentContext) string {
	var sb strings.Builder

	sb.WriteString("\n\n## Available Integrations\n\n")

	// MCP Tools section
	if len(ctx.MCPTools) > 0 {
		sb.WriteString("### MCP-Based Tools\n")
		sb.WriteString("These services are accessed via MCP (Model Context Protocol). Use the MCP tools:\n")
		sb.WriteString("- `list_app_tools(app=\"provider:app\")` to discover available actions\n")
		sb.WriteString("- `call_app_tool(app=\"provider:app\", tool=\"tool-name\", input={...})` to execute\n\n")

		for _, tool := range ctx.MCPTools {
			sb.WriteString(fmt.Sprintf("**%s** (%s)\n", tool.Name, tool.ID))
			if tool.Instructions != "" {
				sb.WriteString(tool.Instructions + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// CLI Tools section
	if len(ctx.CLITools) > 0 {
		sb.WriteString("### CLI Tools\n")
		sb.WriteString("These services have official CLIs that are pre-installed and authenticated:\n\n")

		for _, tool := range ctx.CLITools {
			sb.WriteString(fmt.Sprintf("**%s** - CLI: `%s`\n", tool.Name, tool.CLICommand))
			if tool.Instructions != "" {
				sb.WriteString(tool.Instructions + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// Cloud CLIs section
	if len(ctx.CloudCLIs) > 0 {
		sb.WriteString("### Cloud Provider CLIs\n")
		sb.WriteString("These cloud CLIs are pre-authenticated with temporary credentials:\n\n")

		for _, tool := range ctx.CloudCLIs {
			sb.WriteString(fmt.Sprintf("**%s** - CLI: `%s`\n", tool.Name, tool.CLICommand))
			if tool.Instructions != "" {
				sb.WriteString(tool.Instructions + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// API Tools section
	if len(ctx.APITools) > 0 {
		sb.WriteString("### Direct API Access\n")
		sb.WriteString("These services are accessed via their REST APIs. API keys are available as environment variables:\n\n")

		for _, tool := range ctx.APITools {
			sb.WriteString(fmt.Sprintf("**%s**\n", tool.Name))
			if tool.Instructions != "" {
				sb.WriteString(tool.Instructions + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// Direct MCP section
	if len(ctx.DirectMCP) > 0 {
		sb.WriteString("### Direct MCP Servers\n")
		sb.WriteString("These services provide their own MCP servers:\n\n")

		for _, tool := range ctx.DirectMCP {
			sb.WriteString(fmt.Sprintf("**%s**\n", tool.Name))
			if tool.Instructions != "" {
				sb.WriteString(tool.Instructions + "\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// GenerateSandboxConfig generates sandbox configuration for a user's integrations
func (r *Registry) GenerateSandboxConfig(userID string) ([]*SandboxConfig, error) {
	integrations := r.GetEnabledIntegrationsForUser(userID)

	r.mu.RLock()
	userIntegrations := r.userIntegrations[userID]
	r.mu.RUnlock()

	var configs []*SandboxConfig

	for _, integration := range integrations {
		ui := userIntegrations[integration.ID]
		if ui == nil {
			continue
		}

		config := &SandboxConfig{
			IntegrationID: integration.ID,
			ProviderType:  integration.ProviderType,
			EnvVars:       make(map[string]string),
			Files:         make(map[string]string),
			Scripts:       make(map[string]string),
			SetupCommands: make([]string, 0),
		}

		switch integration.ProviderType {
		case ProviderCLI:
			r.configureCLIIntegration(config, integration, ui)
		case ProviderCloudCLI:
			// Handled by cloud package
		case ProviderAPI:
			r.configureAPIIntegration(config, integration, ui)
		case ProviderMCP, ProviderDirectMCP:
			// MCP tools don't need special sandbox config - they go through the agent
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// configureCLIIntegration sets up a CLI-based integration
func (r *Registry) configureCLIIntegration(config *SandboxConfig, integration *Integration, ui *UserIntegration) {
	switch integration.ID {
	case "github":
		if ui.OAuth2Token != nil {
			// Create gh CLI config
			config.Files["/root/.config/gh/hosts.yml"] = fmt.Sprintf(`github.com:
    oauth_token: %s
    user: %s
    git_protocol: https
`, ui.OAuth2Token.AccessToken, ui.AccountID)
		}
		// Install gh CLI if needed
		if integration.CLIInstallCmd != "" {
			config.SetupCommands = append(config.SetupCommands,
				"which gh || ("+integration.CLIInstallCmd+")")
		}
	}
}

// configureAPIIntegration sets up an API-based integration
func (r *Registry) configureAPIIntegration(config *SandboxConfig, integration *Integration, ui *UserIntegration) {
	switch integration.ID {
	case "datadog":
		if ui.APIKey != "" {
			config.EnvVars["DATADOG_API_KEY"] = ui.APIKey
			// App key might be stored differently
		}
	case "newrelic":
		if ui.APIKey != "" {
			config.EnvVars["NEW_RELIC_API_KEY"] = ui.APIKey
		}
	case "pagerduty":
		if ui.APIKey != "" {
			config.EnvVars["PAGERDUTY_API_KEY"] = ui.APIKey
		}
	case "splunk":
		if ui.OAuth2Token != nil {
			config.EnvVars["SPLUNK_TOKEN"] = ui.OAuth2Token.AccessToken
		}
	}
}

// GetAvailableIntegrations returns all integrations with their connection status for a user
func (r *Registry) GetAvailableIntegrations(userID string) []IntegrationStatus {
	r.mu.RLock()
	userIntegrations := r.userIntegrations[userID]
	r.mu.RUnlock()

	var result []IntegrationStatus

	for _, integration := range Catalog {
		if !integration.Enabled {
			continue
		}

		status := IntegrationStatus{
			Integration: integration,
			Connected:   false,
		}

		if userIntegrations != nil {
			if ui, ok := userIntegrations[integration.ID]; ok && ui.Enabled {
				status.Connected = true
				status.AccountName = ui.AccountName
				status.AccountID = ui.AccountID
				status.ConnectedAt = ui.ConnectedAt
			}
		}

		result = append(result, status)
	}

	return result
}

// IntegrationStatus combines integration definition with user's connection status
type IntegrationStatus struct {
	*Integration
	Connected   bool      `json:"connected"`
	AccountName string    `json:"accountName,omitempty"`
	AccountID   string    `json:"accountId,omitempty"`
	ConnectedAt time.Time `json:"connectedAt,omitempty"`
}
