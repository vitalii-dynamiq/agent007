package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Port        string
	FrontendURL string
	BackendURL  string

	// LLM
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string
	LLMBaseURL  string

	// E2B
	E2BAPIKey     string
	E2BTemplateID string // Custom template ID for faster startup

	// MCP Provider Selection
	MCPProvider string // "auto" (default), "pipedream", "composio"

	// Pipedream
	PipedreamClientID     string
	PipedreamClientSecret string
	PipedreamProjectID    string
	PipedreamEnvironment  string

	// Composio
	ComposioAPIKey      string
	ComposioProjectID   string
	ComposioAuthConfigs map[string]string

	// GitHub App (server-to-server)
	GitHubAppID         string
	GitHubAppSlug       string
	GitHubAppPrivateKey string

	// AWS - for assuming roles on behalf of users
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string

	// Security
	JWTSecret string

	// Integration OAuth Credentials
	Integrations IntegrationCredentials
}

// IntegrationCredentials holds OAuth credentials for various integrations
type IntegrationCredentials struct {
	// GitHub
	GitHubClientID     string
	GitHubClientSecret string

	// Google (Gmail, Calendar, Drive)
	GoogleClientID     string
	GoogleClientSecret string

	// Slack
	SlackClientID     string
	SlackClientSecret string

	// Notion
	NotionClientID     string
	NotionClientSecret string

	// Linear
	LinearClientID     string
	LinearClientSecret string

	// Atlassian (Jira, Confluence)
	AtlassianClientID     string
	AtlassianClientSecret string

	// Microsoft (Teams, Outlook)
	MicrosoftClientID     string
	MicrosoftClientSecret string
	MicrosoftTenantID     string

	// HubSpot
	HubSpotClientID     string
	HubSpotClientSecret string

	// Asana
	AsanaClientID     string
	AsanaClientSecret string

	// Sentry
	SentryClientID     string
	SentryClientSecret string
	SentryRedirectURL  string

	// Canva
	CanvaClientID     string
	CanvaClientSecret string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		BackendURL:  getEnv("BACKEND_URL", "http://localhost:8080"),

		LLMProvider: getEnv("LLM_PROVIDER", "openai"),
		LLMAPIKey:   getEnv("LLM_API_KEY", ""),
		LLMModel:    getEnv("LLM_MODEL", "gpt-5.2"),
		LLMBaseURL:  getEnv("LLM_BASE_URL", ""),

		E2BAPIKey:     getEnv("E2B_API_KEY", ""),
		E2BTemplateID: getEnv("E2B_TEMPLATE_ID", "base"), // Use "dynamiq-agent-sandbox" after building custom template

		MCPProvider: getEnv("MCP_PROVIDER", "auto"), // Default to auto

		PipedreamClientID:     getEnv("PIPEDREAM_CLIENT_ID", ""),
		PipedreamClientSecret: getEnv("PIPEDREAM_CLIENT_SECRET", ""),
		PipedreamProjectID:    getEnv("PIPEDREAM_PROJECT_ID", ""),
		PipedreamEnvironment:  getEnv("PIPEDREAM_ENVIRONMENT", "development"),

		ComposioAPIKey:      getEnv("COMPOSIO_API_KEY", ""),
		ComposioProjectID:   getEnv("COMPOSIO_PROJECT_ID", ""),
		ComposioAuthConfigs: parseJSONMap(getEnv("COMPOSIO_AUTH_CONFIGS", "")),

		GitHubAppID:         getEnv("GITHUB_APP_ID", ""),
		GitHubAppSlug:       getEnv("GITHUB_APP_SLUG", ""),
		GitHubAppPrivateKey: getEnv("GITHUB_APP_PRIVATE_KEY", ""),

		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),

		JWTSecret: getEnv("JWT_SECRET", "default-secret-change-me"),

		Integrations: IntegrationCredentials{
			GitHubClientID:        getEnv("GITHUB_CLIENT_ID", ""),
			GitHubClientSecret:    getEnv("GITHUB_CLIENT_SECRET", ""),
			GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
			SlackClientID:         getEnv("SLACK_CLIENT_ID", ""),
			SlackClientSecret:     getEnv("SLACK_CLIENT_SECRET", ""),
			NotionClientID:        getEnv("NOTION_CLIENT_ID", ""),
			NotionClientSecret:    getEnv("NOTION_CLIENT_SECRET", ""),
			LinearClientID:        getEnv("LINEAR_CLIENT_ID", ""),
			LinearClientSecret:    getEnv("LINEAR_CLIENT_SECRET", ""),
			AtlassianClientID:     getEnv("ATLASSIAN_CLIENT_ID", ""),
			AtlassianClientSecret: getEnv("ATLASSIAN_CLIENT_SECRET", ""),
			MicrosoftClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
			MicrosoftClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
			MicrosoftTenantID:     getEnv("MICROSOFT_TENANT_ID", ""),
			HubSpotClientID:       getEnv("HUBSPOT_CLIENT_ID", ""),
			HubSpotClientSecret:   getEnv("HUBSPOT_CLIENT_SECRET", ""),
			AsanaClientID:         getEnv("ASANA_CLIENT_ID", ""),
			AsanaClientSecret:     getEnv("ASANA_CLIENT_SECRET", ""),
			SentryClientID:        getEnv("SENTRY_CLIENT_ID", ""),
			SentryClientSecret:    getEnv("SENTRY_CLIENT_SECRET", ""),
			SentryRedirectURL:     getEnv("SENTRY_REDIRECT_URL", ""),
			CanvaClientID:         getEnv("CANVA_CLIENT_ID", ""),
			CanvaClientSecret:     getEnv("CANVA_CLIENT_SECRET", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseJSONMap(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed
}
