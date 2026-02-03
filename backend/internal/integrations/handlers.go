package integrations

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

// CloudCredentialManager interface for storing database credentials
type CloudCredentialManager interface {
	StorePostgresCredentials(userID, name string, config interface{}) error
}

// Handlers contains HTTP handlers for integration operations
type Handlers struct {
	registry     *Registry
	frontendURL  string
	cloudManager CloudCredentialManager
}

// NewHandlers creates new integration handlers
func NewHandlers(registry *Registry, frontendURL string) *Handlers {
	return &Handlers{
		registry:    registry,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

// SetCloudManager sets the cloud manager for database credential storage
func (h *Handlers) SetCloudManager(cm CloudCredentialManager) {
	h.cloudManager = cm
}

// HandleListIntegrations lists all available integrations with connection status
func (h *Handlers) HandleListIntegrations(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	integrations := h.registry.GetAvailableIntegrations(userID)

	// Group by category
	grouped := make(map[Category][]IntegrationStatus)
	for _, i := range integrations {
		grouped[i.Category] = append(grouped[i.Category], i)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"integrations": integrations,
		"grouped":      grouped,
		"categories": []Category{
			CategoryDeveloperTools,
			CategoryCloud,
			CategoryProductivity,
			CategoryCommunication,
			CategoryMonitoring,
			CategoryData,
			CategorySecurity,
		},
	})
}

// HandleGetIntegration returns details for a specific integration
func (h *Handlers) HandleGetIntegration(w http.ResponseWriter, r *http.Request) {
	integrationID := chi.URLParam(r, "id")

	integration, ok := GetIntegration(integrationID)
	if !ok {
		http.Error(w, "Integration not found", http.StatusNotFound)
		return
	}

	userID := getUserID(r)
	ui, connected := h.registry.GetUserIntegration(userID, integrationID)

	response := map[string]interface{}{
		"integration": integration,
		"connected":   connected,
	}

	if connected && ui != nil {
		response["accountName"] = ui.AccountName
		response["accountId"] = ui.AccountID
		response["connectedAt"] = ui.ConnectedAt
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleConnectIntegration initiates connection for an integration
func (h *Handlers) HandleConnectIntegration(w http.ResponseWriter, r *http.Request) {
	integrationID := chi.URLParam(r, "id")
	userID := getUserID(r)

	integration, ok := GetIntegration(integrationID)
	if !ok {
		http.Error(w, "Integration not found", http.StatusNotFound)
		return
	}

	var req struct {
		// For API key auth
		APIKey string `json:"apiKey,omitempty"`

		// For token auth
		Token string `json:"token,omitempty"`

		// For OAuth2 (code exchange)
		Code  string `json:"code,omitempty"`
		State string `json:"state,omitempty"`

		// For OAuth2 via MCP (Pipedream/Composio completed flow)
		OAuthComplete string `json:"oauthComplete,omitempty"`

		// For IAM role
		RoleARN    string `json:"roleArn,omitempty"`
		ExternalID string `json:"externalId,omitempty"`
		Region     string `json:"region,omitempty"`

		// For AWS access keys (stored via cloud credentials endpoint)
		AccessKeyID     string `json:"accessKeyId,omitempty"`
		SecretAccessKey string `json:"secretAccessKey,omitempty"`

		// For service account
		ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`

		// For database connections
		Host     string `json:"host,omitempty"`
		Port     int    `json:"port,omitempty"`
		Database string `json:"database,omitempty"`
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		SSLMode  string `json:"sslMode,omitempty"`

		// Account info
		AccountName string `json:"accountName,omitempty"`
		AccountID   string `json:"accountId,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ui := &UserIntegration{
		AccountName: req.AccountName,
		AccountID:   req.AccountID,
	}

	switch integration.AuthType {
	case AuthAPIKey:
		if req.APIKey == "" {
			http.Error(w, "API key is required", http.StatusBadRequest)
			return
		}
		ui.APIKey = req.APIKey

	case AuthToken:
		if req.Token == "" {
			http.Error(w, "Token is required", http.StatusBadRequest)
			return
		}
		ui.OAuth2Token = &OAuth2Token{
			AccessToken: req.Token,
			TokenType:   "Bearer",
		}

	case AuthIAMRole:
		if req.RoleARN == "" {
			http.Error(w, "Role ARN is required", http.StatusBadRequest)
			return
		}
		ui.IAMRoleConfig = &IAMRoleConfig{
			RoleARN:    req.RoleARN,
			ExternalID: req.ExternalID,
			Region:     req.Region,
		}

	case AuthAWSAccessKey:
		// Primary mode: access keys (stored via /api/cloud/credentials/aws)
		// Secondary mode: IAM role (roleArn provided)
		if req.RoleARN != "" {
			ui.IAMRoleConfig = &IAMRoleConfig{
				RoleARN:    req.RoleARN,
				ExternalID: req.ExternalID,
				Region:     req.Region,
			}
		} else if req.AccountID == "" {
			http.Error(w, "accountId is required for AWS access keys", http.StatusBadRequest)
			return
		}

	case AuthServiceAccount:
		if req.ServiceAccountJSON == "" {
			http.Error(w, "Service account JSON is required", http.StatusBadRequest)
			return
		}
		ui.ServiceAccount = req.ServiceAccountJSON

	case AuthDatabase:
		if req.Host == "" || req.Database == "" || req.Username == "" {
			http.Error(w, "host, database, and username are required for database connection", http.StatusBadRequest)
			return
		}
		port := req.Port
		if port == 0 {
			port = 5432 // Default PostgreSQL port
		}
		sslMode := req.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		ui.DatabaseConfig = &DatabaseConfig{
			Host:     req.Host,
			Port:     port,
			Database: req.Database,
			Username: req.Username,
			Password: req.Password,
			SSLMode:  sslMode,
		}
		ui.AccountName = req.Database + "@" + req.Host
		
		// Also store in cloud manager for sandbox injection
		if h.cloudManager != nil {
			name := req.AccountName
			if name == "" {
				name = req.Database + "@" + req.Host
			}
			// Store as generic interface - cloud manager will handle the type
			if err := h.cloudManager.StorePostgresCredentials(userID, name, ui.DatabaseConfig); err != nil {
				log.Printf("Warning: Failed to store database credentials in cloud manager: %v", err)
			}
		}

	case AuthOAuth2:
		// For OAuth2, we expect either:
		// 1. oauthComplete - MCP provider (Pipedream/Composio) OAuth was completed
		// 2. code - direct OAuth2 code to exchange
		// 3. nothing - return auth URL
		if req.OAuthComplete == "true" {
			// MCP provider OAuth was completed via Pipedream/Composio connect UI
			// The actual token is managed by the MCP provider, we just track connection status
			log.Printf("OAuth2 connection completed for integration %s via MCP", integrationID)
			ui.AccountName = "Connected via " + integration.MCPProvider
		} else if req.Code != "" {
			// Exchange code for token
			handler, ok := h.registry.GetOAuth2Handler(integrationID)
			if !ok {
				http.Error(w, "OAuth2 not configured for this integration", http.StatusBadRequest)
				return
			}

			token, err := handler.ExchangeCode(r.Context(), req.Code)
			if err != nil {
				log.Printf("Failed to exchange OAuth2 code: %v", err)
				http.Error(w, "Failed to exchange code", http.StatusBadRequest)
				return
			}
			ui.OAuth2Token = token
		} else {
			// Return auth URL for OAuth2 flow
			handler, ok := h.registry.GetOAuth2Handler(integrationID)
			if !ok {
				if integration.ProviderType == ProviderMCP && integration.MCPProvider != "" {
					// Fall back to MCP provider connect flow
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"authType":    "oauth2",
						"mcpProvider": integration.MCPProvider,
						"message":     "Use MCP provider OAuth flow",
					})
					return
				}
				http.Error(w, "OAuth2 not configured for this integration", http.StatusBadRequest)
				return
			}

			state := generateState(userID, integrationID)
			authURL := handler.GetAuthURL(state)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authUrl": authURL,
				"state":   state,
			})
			return
		}

	case AuthGitHubApp:
		// GitHub App installation handled via /api/github/install + /api/github/callback
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authType": "github_app",
			"message":  "Use GitHub App installation flow",
		})
		return

	case AuthNone:
		// No auth needed

	default:
		http.Error(w, "Unsupported auth type", http.StatusBadRequest)
		return
	}

	if err := h.registry.ConnectIntegration(userID, integrationID, ui); err != nil {
		log.Printf("Failed to connect integration: %v", err)
		http.Error(w, "Failed to connect integration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Integration connected successfully",
		"connected": true,
	})
}

// HandleOAuthCallback completes an OAuth2 flow and stores tokens.
func (h *Handlers) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	errParam := query.Get("error")

	if errParam != "" {
		http.Redirect(w, r, h.frontendURL+"?oauth=error&message="+url.QueryEscape(errParam), http.StatusTemporaryRedirect)
		return
	}

	userID, integrationID, ok := parseState(state)
	if !ok || code == "" {
		http.Error(w, "Invalid OAuth callback", http.StatusBadRequest)
		return
	}

	handler, ok := h.registry.GetOAuth2Handler(integrationID)
	if !ok {
		http.Error(w, "OAuth2 not configured for this integration", http.StatusBadRequest)
		return
	}

	token, err := handler.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusBadRequest)
		return
	}

	ui := &UserIntegration{
		OAuth2Token: token,
	}

	if err := h.registry.ConnectIntegration(userID, integrationID, ui); err != nil {
		http.Error(w, "Failed to connect integration", http.StatusInternalServerError)
		return
	}

	redirect := h.frontendURL + "?oauth=success&app=" + url.QueryEscape(integrationID)
	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

// HandleDisconnectIntegration disconnects an integration
func (h *Handlers) HandleDisconnectIntegration(w http.ResponseWriter, r *http.Request) {
	integrationID := chi.URLParam(r, "id")
	userID := getUserID(r)

	if err := h.registry.DisconnectIntegration(userID, integrationID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Integration disconnected",
		"connected": false,
	})
}

// HandleGetAgentContext returns agent context for current user's integrations
func (h *Handlers) HandleGetAgentContext(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	ctx := h.registry.GenerateAgentContext(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}

// HandleGetSandboxConfig returns sandbox configuration for a user's integrations
func (h *Handlers) HandleGetSandboxConfig(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	configs, err := h.registry.GenerateSandboxConfig(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": configs,
	})
}

// Helper to get user ID from request (in production, from auth middleware)
func getUserID(r *http.Request) string {
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	return "default-user"
}

// Helper to generate OAuth2 state
func generateState(userID, integrationID string) string {
	return userID + ":" + integrationID
}

func parseState(state string) (userID, integrationID string, ok bool) {
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
