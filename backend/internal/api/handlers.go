package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dynamiq/manus-like/internal/agent"
	"github.com/dynamiq/manus-like/internal/auth"
	"github.com/dynamiq/manus-like/internal/cloud"
	"github.com/dynamiq/manus-like/internal/config"
	"github.com/dynamiq/manus-like/internal/github"
	"github.com/dynamiq/manus-like/internal/integrations"
	"github.com/dynamiq/manus-like/internal/llm"
	"github.com/dynamiq/manus-like/internal/mcp"
	"github.com/dynamiq/manus-like/internal/store"
	"github.com/go-chi/chi/v5"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	config              *config.Config
	store               *store.MemoryStore
	llmClient           llm.Client
	mcpProvider         mcp.Provider
	mcpRegistry         *mcp.Registry // For accessing individual providers
	githubApp           *github.AppClient
	agentClient         *agent.Client // Python agent service
	tokenManager        *auth.TokenManager
	cloudManager        *cloud.Manager
	cloudHandlers       *cloud.Handlers
	integrationRegistry *integrations.Registry
	integrationHandlers *integrations.Handlers
}

// NewHandlers creates new handlers
func NewHandlers(cfg *config.Config) (*Handlers, error) {
	// Initialize LLM client
	llmClient, err := llm.NewClient(llm.Config{
		Provider: cfg.LLMProvider,
		APIKey:   cfg.LLMAPIKey,
		Model:    cfg.LLMModel,
		BaseURL:  cfg.LLMBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Initialize MCP provider registry
	registry := mcp.NewRegistry()

	// Initialize GitHub App client if configured
	var githubApp *github.AppClient
	if cfg.GitHubAppID != "" && cfg.GitHubAppSlug != "" && cfg.GitHubAppPrivateKey != "" {
		appClient, err := github.NewAppClient(cfg.GitHubAppID, cfg.GitHubAppSlug, cfg.GitHubAppPrivateKey)
		if err != nil {
			log.Printf("Warning: Failed to initialize GitHub App client: %v", err)
		} else {
			githubApp = appClient
			log.Printf("Initialized GitHub App client: %s", cfg.GitHubAppSlug)
		}
	}

	// Add Pipedream provider if configured
	if cfg.PipedreamClientID != "" && cfg.PipedreamClientSecret != "" {
		err := registry.CreateProvider(mcp.ProviderConfig{
			Type:      mcp.ProviderTypePipedream,
			Name:      "pipedream",
			ProjectID: cfg.PipedreamProjectID,
			Extra: map[string]string{
				"clientId":     cfg.PipedreamClientID,
				"clientSecret": cfg.PipedreamClientSecret,
				"environment":  cfg.PipedreamEnvironment,
			},
		})
		if err != nil {
			log.Printf("Warning: Failed to create Pipedream provider: %v", err)
		} else {
			log.Printf("Initialized Pipedream MCP provider (project: %s)", cfg.PipedreamProjectID)
		}
	}

	// Add Composio provider if configured
	if cfg.ComposioAPIKey != "" {
		extra := map[string]string{}
		if cfg.ComposioAuthConfigs != nil {
			if bytes, err := json.Marshal(cfg.ComposioAuthConfigs); err == nil {
				extra["authConfigIds"] = string(bytes)
			}
		}
		err := registry.CreateProvider(mcp.ProviderConfig{
			Type:      mcp.ProviderTypeComposio,
			Name:      "composio",
			APIKey:    cfg.ComposioAPIKey,
			ProjectID: cfg.ComposioProjectID,
			Extra:     extra,
		})
		if err != nil {
			log.Printf("Warning: Failed to create Composio provider: %v", err)
		} else {
			log.Printf("Initialized Composio MCP provider (project: %s)", cfg.ComposioProjectID)
		}
	}

	// Set default provider
	providers := registry.ProviderNames()
	if len(providers) == 0 {
		log.Printf("WARNING: No MCP providers configured. Set PIPEDREAM_* or COMPOSIO_* env vars.")
	} else {
		// Try to set the configured default, fall back to first available
		defaultProvider := strings.ToLower(strings.TrimSpace(cfg.MCPProvider))
		if defaultProvider == "" || defaultProvider == "auto" {
			if _, ok := registry.GetProvider("pipedream"); ok {
				defaultProvider = "pipedream"
			} else if _, ok := registry.GetProvider("composio"); ok {
				defaultProvider = "composio"
			} else {
				defaultProvider = providers[0]
			}
		}
		if _, ok := registry.GetProvider(defaultProvider); !ok {
			defaultProvider = providers[0]
		}
		registry.SetDefaultProvider(defaultProvider)
		log.Printf("MCP providers available: %v (default: %s)", providers, defaultProvider)
	}

	// Initialize token manager (5 minute TTL for session tokens)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, 5*time.Minute)

	// Initialize cloud credential manager
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}
	cloudManager, err := cloud.NewManager(cfg.JWTSecret, tokenManager, backendURL)
	if err != nil {
		log.Printf("Warning: Failed to initialize cloud manager: %v", err)
	} else {
		// Set default AWS credentials if available
		if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
			cloudManager.SetAWSDefaultCredentials(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey)
			log.Printf("Initialized AWS credential provider with default credentials")
		}
		log.Printf("Cloud credential manager initialized")
	}

	var cloudHandlers *cloud.Handlers
	if cloudManager != nil {
		cloudHandlers = cloud.NewHandlers(cloudManager)
	}

	// Initialize integration registry
	integrationRegistry := integrations.NewRegistry(cfg.JWTSecret)
	integrationHandlers := integrations.NewHandlers(integrationRegistry, cfg.FrontendURL)
	log.Printf("Integration registry initialized with %d available integrations", len(integrations.GetEnabledIntegrations()))

	// Register OAuth2 handler for GitHub (CLI-based OAuth flow)
	if githubIntegration, ok := integrations.Catalog["github"]; ok && githubIntegration.OAuth2Config != nil {
		if cfg.Integrations.GitHubClientID != "" && cfg.Integrations.GitHubClientSecret != "" {
			redirectURL := strings.TrimRight(cfg.BackendURL, "/") + "/api/integrations/oauth/callback"
			oauthHandler := integrations.NewOAuth2Handler(integrations.OAuth2HandlerConfig{
				ClientID:     cfg.Integrations.GitHubClientID,
				ClientSecret: cfg.Integrations.GitHubClientSecret,
				AuthURL:      githubIntegration.OAuth2Config.AuthURL,
				TokenURL:     githubIntegration.OAuth2Config.TokenURL,
				RedirectURL:  redirectURL,
				Scopes:       githubIntegration.OAuth2Config.Scopes,
			})
			integrationRegistry.RegisterOAuth2Handler("github", oauthHandler)
		} else {
			log.Printf("GitHub OAuth2 not configured: missing GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET")
		}
	}

	// Register OAuth2 handlers for direct MCP integrations (Sentry)
	if sentry, ok := integrations.Catalog["sentry"]; ok && sentry.OAuth2Config != nil {
		if cfg.Integrations.SentryClientID != "" && cfg.Integrations.SentryClientSecret != "" {
			redirectURL := cfg.Integrations.SentryRedirectURL
			if redirectURL == "" {
				redirectURL = strings.TrimRight(cfg.BackendURL, "/") + "/api/integrations/oauth/callback"
			}
			oauthHandler := integrations.NewOAuth2Handler(integrations.OAuth2HandlerConfig{
				ClientID:     cfg.Integrations.SentryClientID,
				ClientSecret: cfg.Integrations.SentryClientSecret,
				AuthURL:      sentry.OAuth2Config.AuthURL,
				TokenURL:     sentry.OAuth2Config.TokenURL,
				RedirectURL:  redirectURL,
				Scopes:       sentry.OAuth2Config.Scopes,
			})
			integrationRegistry.RegisterOAuth2Handler("sentry", oauthHandler)
		} else {
			log.Printf("Sentry OAuth2 not configured: missing SENTRY_CLIENT_ID or SENTRY_CLIENT_SECRET")
		}
	}

	// Initialize Python agent client
	agentURL := os.Getenv("AGENT_URL")
	if agentURL == "" {
		agentURL = "http://localhost:8081"
	}
	agentClient := agent.NewClient(agentURL)
	log.Printf("Agent client configured for: %s", agentURL)

	// Register direct MCP providers (official hosted MCP servers like Sentry)
	if sentry, ok := integrations.Catalog["sentry"]; ok && sentry.ProviderType == integrations.ProviderDirectMCP {
		if sentry.MCPServerURL != "" {
			sentryProvider := mcp.NewDirectMCPProvider(sentry.ID, sentry.MCPServerURL, "")
			sentryProvider.SetTokenProvider(func(ctx context.Context, userID string) (string, error) {
				return integrationRegistry.GetOAuth2AccessToken(ctx, userID, sentry.ID)
			})
			registry.AddProvider(sentry.ID, sentryProvider)
			log.Printf("Registered direct MCP provider: %s", sentry.ID)
		}
	}

	return &Handlers{
		config:              cfg,
		store:               store.NewMemoryStore(),
		llmClient:           llmClient,
		mcpProvider:         registry,
		mcpRegistry:         registry,
		githubApp:           githubApp,
		agentClient:         agentClient,
		tokenManager:        tokenManager,
		cloudManager:        cloudManager,
		cloudHandlers:       cloudHandlers,
		integrationRegistry: integrationRegistry,
		integrationHandlers: integrationHandlers,
	}, nil
}

// Health check handler
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListConversations lists all conversations
func (h *Handlers) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	conversations := h.store.ListConversations(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversations)
}

// CreateConversation creates a new conversation
func (h *Handlers) CreateConversation(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	var req struct {
		Title string `json:"title"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	conv := h.store.CreateConversation(userID, req.Title)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(conv)
}

// GetConversation gets a conversation by ID
func (h *Handlers) GetConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conv := h.store.GetConversation(id)

	if conv == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conv)
}

// DeleteConversation deletes a conversation
func (h *Handlers) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.store.DeleteConversation(id)
	w.WriteHeader(http.StatusNoContent)
}

// UpdateConversation updates a conversation
func (h *Handlers) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	conv := h.store.GetConversation(id)
	if conv == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	var req struct {
		Title        string   `json:"title,omitempty"`
		EnabledTools []string `json:"enabledTools,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.store.UpdateConversation(id, req.Title, req.EnabledTools)

	// Return updated conversation
	conv = h.store.GetConversation(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conv)
}

// GetConversationTools gets the enabled tools for a conversation
func (h *Handlers) GetConversationTools(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	conv := h.store.GetConversation(id)
	if conv == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabledTools": conv.EnabledTools,
	})
}

// SetConversationTools sets the enabled tools for a conversation
func (h *Handlers) SetConversationTools(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	conv := h.store.GetConversation(id)
	if conv == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	var req struct {
		EnabledTools []string `json:"enabledTools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.store.SetEnabledTools(id, req.EnabledTools)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabledTools": req.EnabledTools,
	})
}

// SendMessage sends a message and streams the response via Python agent
func (h *Handlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	userID := getUserID(r)

	conv := h.store.GetConversation(convID)
	if conv == nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Add user message
	h.store.AddMessage(convID, store.Message{
		Role:    "user",
		Content: req.Content,
	})

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Generate session token for MCP proxy access
	sessionToken, err := h.tokenManager.GenerateSessionToken(userID, convID, "mcp")
	if err != nil {
		sendSSEEvent(w, flusher, "error", map[string]string{"message": "Failed to generate session token"})
		sendSSEEvent(w, flusher, "done", nil)
		return
	}

	// Get MCP proxy URL (must be accessible from E2B sandbox)
	// Prefer MCP_PROXY_URL if explicitly set, otherwise derive from BackendURL.
	mcpProxyURL := os.Getenv("MCP_PROXY_URL")
	if mcpProxyURL == "" {
		backendURL := strings.TrimRight(h.config.BackendURL, "/")
		if backendURL == "" {
			backendURL = strings.TrimRight(os.Getenv("BACKEND_URL"), "/")
		}
		if backendURL != "" {
			mcpProxyURL = backendURL + "/api/mcp/proxy"
		}
	}

	if mcpProxyURL == "" {
		log.Printf("[Agent] WARNING: MCP_PROXY_URL not set - MCP tools won't work from sandbox")
	}

	// Call Python agent service with SSE streaming
	eventChan := make(chan agent.Event)
	go func() {
		err := h.agentClient.RunStream(ctx, agent.RunRequest{
			Message:        req.Content,
			UserID:         userID,
			SessionToken:   sessionToken,
			ConversationID: convID,
			MCPProxyURL:    mcpProxyURL,
		}, eventChan)
		if err != nil {
			log.Printf("[Agent] Stream error: %v", err)
		}
	}()

	var assistantContent string
	var toolCalls []store.ToolCall
	stringify := func(value interface{}) string {
		if value == nil {
			return ""
		}
		if s, ok := value.(string); ok {
			return s
		}
		if bytes, err := json.Marshal(value); err == nil {
			return string(bytes)
		}
		return fmt.Sprintf("%v", value)
	}

	for event := range eventChan {
		// Forward event to frontend
		sendSSEEvent(w, flusher, event.Type, event.Content)

		// Track for storage
		switch event.Type {
		case "message":
			if content, ok := event.Content.(map[string]interface{}); ok {
				if c, ok := content["content"].(string); ok {
					assistantContent = c
				}
			}
		case "tool_call":
			if tc, ok := event.Content.(map[string]interface{}); ok {
				argsValue, hasArgs := tc["arguments"]
				if !hasArgs {
					argsValue = tc["args"]
				}
				toolCalls = append(toolCalls, store.ToolCall{
					ID:        stringify(tc["id"]),
					Name:      stringify(tc["name"]),
					Arguments: stringify(argsValue),
				})
			}
		case "tool_result":
			if tr, ok := event.Content.(map[string]interface{}); ok {
				result := stringify(tr["result"])
				id := stringify(tr["id"])
				name := stringify(tr["name"])
				if id != "" {
					for i := range toolCalls {
						if toolCalls[i].ID == id {
							toolCalls[i].Result = result
							break
						}
					}
				} else if name != "" {
					for i := range toolCalls {
						if toolCalls[i].Name == name && toolCalls[i].Result == "" {
							toolCalls[i].Result = result
							break
						}
					}
				}
			}
		case "status":
			if status, ok := event.Content.(map[string]interface{}); ok {
				if sandboxID, ok := status["sandbox_id"].(string); ok && sandboxID != "" {
					h.store.SetSandboxID(convID, sandboxID)
				}
			}
		}
	}

	// Save assistant message
	if assistantContent != "" || len(toolCalls) > 0 {
		h.store.AddMessage(convID, store.Message{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: toolCalls,
		})
	}

	sendSSEEvent(w, flusher, "done", nil)
}

// MCPProxy proxies MCP requests from the sandbox CLI
// Security: Only accepts short-lived tokens with appropriate scopes
func (h *Handlers) MCPProxy(w http.ResponseWriter, r *http.Request) {
	// Validate session token from header
	token := r.Header.Get("X-Session-Token")
	if token == "" {
		token = r.Header.Get("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		}
	}

	if token == "" {
		log.Printf("MCP Proxy: No token provided")
		http.Error(w, "Session token required", http.StatusUnauthorized)
		return
	}

	claims, err := h.tokenManager.ValidateSessionToken(token)
	if err != nil {
		log.Printf("MCP Proxy: Invalid token: %v", err)
		http.Error(w, "Invalid or expired session token", http.StatusUnauthorized)
		return
	}

	var req mcp.ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the request (sanitized - no sensitive data)
	log.Printf("MCP Proxy: user=%s method=%s app=%s tool=%s", claims.UserID, req.Method, req.App, req.Tool)

	ctx := r.Context()
	var resp mcp.ProxyResponse

	switch req.Method {
	case "list_tools":
		// Check scope
		if !claims.HasScope(auth.ScopeListTools) && !claims.HasScope(auth.ScopeAll) {
			resp = mcp.ProxyResponse{Success: false, Error: "Insufficient permissions for list_tools"}
			break
		}
		appName := resolveMCPAppProvider(req.App)
		tools, err := h.mcpProvider.ListTools(ctx, claims.UserID, appName)
		if err != nil {
			resp = mcp.ProxyResponse{Success: false, Error: err.Error()}
		} else {
			resp = mcp.ProxyResponse{Success: true, Data: tools}
		}

	case "call_tool":
		// Check scope
		if !claims.HasScope(auth.ScopeCallTools) && !claims.HasScope(auth.ScopeAll) {
			resp = mcp.ProxyResponse{Success: false, Error: "Insufficient permissions for call_tool"}
			break
		}
		appName := resolveMCPAppProvider(req.App)
		result, err := h.mcpProvider.CallTool(ctx, claims.UserID, appName, req.Tool, req.Input)
		if err != nil {
			resp = mcp.ProxyResponse{Success: false, Error: err.Error()}
		} else {
			resp = mcp.ProxyResponse{Success: true, Data: result}
		}

	case "list_apps":
		// Check scope
		if !claims.HasScope(auth.ScopeListApps) && !claims.HasScope(auth.ScopeAll) {
			resp = mcp.ProxyResponse{Success: false, Error: "Insufficient permissions for list_apps"}
			break
		}
		apps, err := h.mcpProvider.ListConnectedApps(ctx, claims.UserID)
		if err != nil {
			log.Printf("MCP Proxy list_apps error: %v", err)
			resp = mcp.ProxyResponse{Success: false, Error: err.Error()}
		} else {
			log.Printf("MCP Proxy list_apps result: %d apps found for user %s", len(apps), claims.UserID)
			for i, app := range apps {
				log.Printf("  App %d: %+v", i, app)
			}
			resp = mcp.ProxyResponse{Success: true, Data: apps}
		}

	default:
		resp = mcp.ProxyResponse{Success: false, Error: "Unknown method: " + req.Method}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetConnectToken gets a connect token for OAuth
func (h *Handlers) GetConnectToken(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	providerName := r.URL.Query().Get("provider")
	resolvedProvider := providerName
	if resolvedProvider == "" && h.mcpRegistry != nil {
		resolvedProvider = h.mcpRegistry.GetDefaultProvider()
	}
	app := r.URL.Query().Get("app") // Optional app slug for redirect

	var connectionData map[string]interface{}
	if r.Body != nil {
		var payload struct {
			ConnectionData map[string]interface{} `json:"connectionData"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		connectionData = payload.ConnectionData
	}

	// Build redirect URLs using the backend URL
	backendURL := h.config.BackendURL
	successRedirectURI := backendURL + "/api/auth/oauth/callback?status=success&app=" + app
	errorRedirectURI := backendURL + "/api/auth/oauth/callback?status=error&app=" + app

	var tokenData string
	var err error

	// Try to get token with redirect URIs for better OAuth flow
	if resolvedProvider == "pipedream" {
		var pdProvider *mcp.PipedreamProvider
		if h.mcpRegistry != nil {
			if provider, ok := h.mcpRegistry.GetProvider("pipedream"); ok {
				if casted, ok := provider.(*mcp.PipedreamProvider); ok {
					pdProvider = casted
				}
			}
		}
		if pdProvider == nil {
			if casted, ok := h.mcpProvider.(*mcp.PipedreamProvider); ok {
				pdProvider = casted
			}
		}
		if pdProvider != nil {
			resp, err2 := pdProvider.GetConnectTokenWithRedirects(r.Context(), userID, successRedirectURI, errorRedirectURI)
			if err2 != nil {
				err = err2
			} else {
				// Format: token|connect_link_url|expires_at
				tokenData = resp.Token + "|" + resp.ConnectLinkURL + "|" + resp.ExpiresAt
			}
		}
	} else if resolvedProvider == "composio" {
		if h.mcpRegistry == nil {
			err = fmt.Errorf("composio provider not configured")
		} else if composioProvider, ok := h.mcpRegistry.GetProvider("composio"); ok {
			if cp, ok := composioProvider.(*mcp.ComposioProvider); ok {
				redirectURL, err2 := cp.GetConnectLink(r.Context(), userID, app, successRedirectURI, connectionData)
				if err2 != nil {
					err = err2
				} else {
					tokenData = "|" + redirectURL
				}
			} else {
				err = fmt.Errorf("composio provider not configured")
			}
		} else {
			err = fmt.Errorf("composio provider not configured")
		}
	} else if resolvedProvider != "" && h.mcpRegistry != nil {
		tokenData, err = h.mcpRegistry.GetConnectTokenForProvider(r.Context(), userID, resolvedProvider)
	} else {
		tokenData, err = h.mcpProvider.GetConnectToken(r.Context(), userID)
	}

	if err != nil {
		log.Printf("Failed to get connect token: %v", err)
		http.Error(w, "Failed to get connect token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse token data - may contain "token|connect_link_url|expires_at"
	token := tokenData
	connectLinkURL := ""
	expiresAt := ""
	parts := strings.Split(tokenData, "|")
	if len(parts) >= 2 {
		token = parts[0]
		connectLinkURL = parts[1]
	}
	if len(parts) >= 3 {
		expiresAt = parts[2]
	}

	// If no expiresAt, default to 10 minutes from now
	if expiresAt == "" {
		expiresAt = time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	}

	log.Printf("Connect token generated for user %s, provider=%s, backendURL=%s, hasConnectLink=%v",
		userID, resolvedProvider, backendURL, connectLinkURL != "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":          token,
		"connectLinkUrl": connectLinkURL,
		"expiresAt":      expiresAt,
		"provider":       resolvedProvider,
	})
}

// HandleOAuthCallback handles OAuth redirects from Pipedream/Composio
func (h *Handlers) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	app := r.URL.Query().Get("app")
	errorMsg := r.URL.Query().Get("error")

	log.Printf("OAuth callback received: status=%s, app=%s, error=%s", status, app, errorMsg)

	// Build the redirect URL to the frontend
	frontendURL := h.config.FrontendURL

	if status == "success" {
		// Redirect to frontend with success status
		redirectURL := frontendURL + "?oauth=success&app=" + app
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	} else {
		// Redirect to frontend with error status
		redirectURL := frontendURL + "?oauth=error&app=" + app + "&error=" + errorMsg
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

// ListMCPProviders lists available MCP providers
func (h *Handlers) ListMCPProviders(w http.ResponseWriter, r *http.Request) {
	providerInfos := h.mcpRegistry.ListProviders()
	providerNames := h.mcpRegistry.ProviderNames()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers":     providerNames,
		"providerInfos": providerInfos,
		"default":       h.mcpRegistry.GetDefaultProvider(),
	})
}

// GetSessionToken gets a session token for sandbox use
func (h *Handlers) GetSessionToken(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	convID := r.URL.Query().Get("conversationId")

	token, err := h.tokenManager.GenerateSessionToken(userID, convID, "")
	if err != nil {
		http.Error(w, "Failed to generate session token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// ListConnectedApps lists connected apps for the user
func (h *Handlers) ListConnectedApps(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	apps, err := h.mcpProvider.ListConnectedApps(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to list connected apps: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

// Helper functions

func getUserID(r *http.Request) string {
	// For now, use a default user ID (in production, extract from auth header)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}
	return userID
}

func resolveMCPAppProvider(app string) string {
	if app == "" || strings.Contains(app, ":") {
		return app
	}
	for _, integration := range integrations.Catalog {
		switch integration.ProviderType {
		case integrations.ProviderMCP:
			if integration.MCPProvider == "" {
				continue
			}
			slug := integration.MCPAppSlug
			if slug == "" {
				slug = integration.ID
			}
			if slug == app || integration.ID == app {
				return integration.MCPProvider + ":" + slug
			}
		case integrations.ProviderDirectMCP:
			slug := integration.ID
			if slug == app || integration.ID == app {
				return integration.ID + ":" + slug
			}
		}
	}
	return app
}

func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	var dataStr string
	if data != nil {
		bytes, _ := json.Marshal(data)
		dataStr = string(bytes)
	} else {
		dataStr = "{}"
	}

	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, dataStr)
	flusher.Flush()
}
