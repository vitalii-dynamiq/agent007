// Package integrations provides a unified abstraction for external service integrations.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Integration Registry                             │
//	│                                                                          │
//	│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
//	│  │  GitHub  │  │   AWS    │  │  Gmail   │  │ Datadog  │  │  Sentry  │  │
//	│  │  (CLI)   │  │(CloudCLI)│  │  (MCP)   │  │  (API)   │  │(DirectMCP│  │
//	│  │  OAuth2  │  │ IAM Role │  │  OAuth2  │  │ API Key  │  │  OAuth2  │  │
//	│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
//	└─────────────────────────────────────────────────────────────────────────┘
//	                                    │
//	                                    ▼
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Sandbox Environment                              │
//	│                                                                          │
//	│  - GitHub CLI (gh) with OAuth token                                     │
//	│  - AWS CLI with credential_process                                       │
//	│  - GCP CLI with workload identity                                        │
//	│  - MCP CLI for Pipedream/Composio services                              │
//	│  - Direct API access with injected tokens/keys                          │
//	└─────────────────────────────────────────────────────────────────────────┘
package integrations

import (
	"time"
)

// ProviderType defines how a service is accessed
type ProviderType string

const (
	// ProviderMCP - Service accessed via MCP protocol (Pipedream, Composio)
	ProviderMCP ProviderType = "mcp"

	// ProviderDirectMCP - Service has its own MCP server
	ProviderDirectMCP ProviderType = "direct_mcp"

	// ProviderCLI - Service accessed via official CLI (gh, jira, etc.)
	ProviderCLI ProviderType = "cli"

	// ProviderCloudCLI - Cloud provider CLI with credential injection (aws, gcloud)
	ProviderCloudCLI ProviderType = "cloud_cli"

	// ProviderAPI - Direct API access with API key/token
	ProviderAPI ProviderType = "api"
)

// AuthType defines how a user authenticates with a service
type AuthType string

const (
	// AuthOAuth2 - OAuth2 flow (GitHub, Google, Slack, etc.)
	AuthOAuth2 AuthType = "oauth2"

	// AuthAPIKey - Simple API key
	AuthAPIKey AuthType = "api_key"

	// AuthServiceAccount - GCP-style service account JSON
	AuthServiceAccount AuthType = "service_account"

	// AuthIAMRole - AWS IAM role assumption
	AuthIAMRole AuthType = "iam_role"

	// AuthAWSAccessKey - AWS access key + secret key
	AuthAWSAccessKey AuthType = "aws_access_key"

	// AuthGitHubApp - GitHub App installation flow
	AuthGitHubApp AuthType = "github_app"

	// AuthToken - Pre-generated token (personal access tokens)
	AuthToken AuthType = "token"

	// AuthDatabase - Database connection string/credentials
	AuthDatabase AuthType = "database"

	// AuthNone - No auth required or handled externally
	AuthNone AuthType = "none"
)

// Category groups related services
type Category string

const (
	CategoryDeveloperTools Category = "developer_tools"
	CategoryProductivity   Category = "productivity"
	CategoryCommunication  Category = "communication"
	CategoryCloud          Category = "cloud"
	CategoryMonitoring     Category = "monitoring"
	CategoryData           Category = "data"
	CategorySecurity       Category = "security"
)

// Integration defines a service/tool that can be used by the agent
type Integration struct {
	// Identity
	ID          string   `json:"id"`          // Unique identifier (e.g., "github", "aws", "gmail")
	Name        string   `json:"name"`        // Display name (e.g., "GitHub", "AWS", "Gmail")
	Description string   `json:"description"` // Brief description
	Category    Category `json:"category"`    // Category for grouping
	Icon        string   `json:"icon"`        // Icon URL or emoji

	// Access configuration
	ProviderType ProviderType `json:"providerType"` // How to access this service
	AuthType     AuthType     `json:"authType"`     // How user authenticates

	// MCP-specific (for ProviderMCP)
	MCPProvider string `json:"mcpProvider,omitempty"` // "pipedream", "composio", etc.
	MCPAppSlug  string `json:"mcpAppSlug,omitempty"`  // App slug in MCP provider

	// Direct MCP (for ProviderDirectMCP)
	MCPServerURL string `json:"mcpServerUrl,omitempty"` // URL of MCP server

	// CLI-specific (for ProviderCLI, ProviderCloudCLI)
	CLICommand    string   `json:"cliCommand,omitempty"`    // CLI binary name (e.g., "gh", "aws")
	CLIInstallCmd string   `json:"cliInstallCmd,omitempty"` // Install command
	CLIAuthCmd    string   `json:"cliAuthCmd,omitempty"`    // Auth setup command

	// API-specific (for ProviderAPI)
	APIBaseURL string `json:"apiBaseUrl,omitempty"` // API base URL
	APIDocsURL string `json:"apiDocsUrl,omitempty"` // API documentation

	// OAuth2 configuration (for AuthOAuth2)
	OAuth2Config *OAuth2Config `json:"oauth2Config,omitempty"`

	// Agent instructions
	AgentInstructions string `json:"agentInstructions,omitempty"` // How agent should use this

	// Capabilities
	Capabilities []string `json:"capabilities,omitempty"` // List of things this can do

	// Status
	Enabled bool `json:"enabled"` // Is this integration available?
	Beta    bool `json:"beta"`    // Is this in beta?
}

// OAuth2Config contains OAuth2 configuration for a service
type OAuth2Config struct {
	AuthURL      string   `json:"authUrl"`
	TokenURL     string   `json:"tokenUrl"`
	Scopes       []string `json:"scopes"`
	ClientID     string   `json:"-"` // Don't expose in JSON
	ClientSecret string   `json:"-"` // Don't expose in JSON
}

// UserIntegration represents a user's configured integration
type UserIntegration struct {
	UserID        string    `json:"userId"`
	IntegrationID string    `json:"integrationId"`
	Enabled       bool      `json:"enabled"`
	ConnectedAt   time.Time `json:"connectedAt"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"`

	// Stored credentials (encrypted) - one of these based on AuthType
	OAuth2Token    *OAuth2Token    `json:"-"` // Don't expose
	APIKey         string          `json:"-"` // Don't expose
	ServiceAccount string          `json:"-"` // Don't expose (JSON string)
	IAMRoleConfig  *IAMRoleConfig  `json:"-"` // Don't expose
	DatabaseConfig *DatabaseConfig `json:"-"` // Don't expose

	// Display info (safe to expose)
	AccountName  string `json:"accountName,omitempty"`  // e.g., "john@example.com"
	AccountID    string `json:"accountId,omitempty"`    // e.g., GitHub username
	Organization string `json:"organization,omitempty"` // e.g., org name

	// GitHub App installation info (internal)
	GitHubInstallationID int64 `json:"-"`
}

// OAuth2Token represents stored OAuth2 credentials
type OAuth2Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes,omitempty"`
}

// IAMRoleConfig represents AWS IAM role configuration
type IAMRoleConfig struct {
	RoleARN    string `json:"roleArn"`
	ExternalID string `json:"externalId,omitempty"`
	Region     string `json:"region,omitempty"`
}

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"sslMode,omitempty"` // disable, require, verify-ca, verify-full
}

// SandboxConfig contains configuration for setting up an integration in a sandbox
type SandboxConfig struct {
	IntegrationID string            `json:"integrationId"`
	ProviderType  ProviderType      `json:"providerType"`
	EnvVars       map[string]string `json:"envVars,omitempty"`       // Environment variables to set
	Files         map[string]string `json:"files,omitempty"`         // Files to create (path -> content)
	Scripts       map[string]string `json:"scripts,omitempty"`       // Scripts to install (path -> content)
	SetupCommands []string          `json:"setupCommands,omitempty"` // Commands to run during setup
}

// AgentContext contains information for generating agent prompts
type AgentContext struct {
	// Grouped integrations by how agent should use them
	MCPTools     []IntegrationInfo `json:"mcpTools"`     // Use via MCP CLI
	CLITools     []IntegrationInfo `json:"cliTools"`     // Use via official CLI
	CloudCLIs    []IntegrationInfo `json:"cloudClis"`    // AWS/GCP style CLIs
	APITools     []IntegrationInfo `json:"apiTools"`     // Direct API access
	DirectMCP    []IntegrationInfo `json:"directMcp"`    // Direct MCP servers

	// Generated instructions
	SystemPromptAddition string `json:"systemPromptAddition"`
}

// IntegrationInfo is a simplified view for agent context
type IntegrationInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	CLICommand   string   `json:"cliCommand,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}
