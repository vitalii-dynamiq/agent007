package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates a new router with all routes configured
func NewRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins for development
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return true // Allow all origins
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"*"}, // Allow all headers
		ExposedHeaders:   []string{"Link", "Content-Type"},
		AllowCredentials: false, // Must be false when AllowedOrigins is "*"
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", h.Health)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Conversations
		r.Route("/conversations", func(r chi.Router) {
			r.Get("/", h.ListConversations)
			r.Post("/", h.CreateConversation)
			r.Get("/{id}", h.GetConversation)
			r.Put("/{id}", h.UpdateConversation)
			r.Delete("/{id}", h.DeleteConversation)
			r.Post("/{id}/messages", h.SendMessage)
			r.Get("/{id}/tools", h.GetConversationTools)
			r.Put("/{id}/tools", h.SetConversationTools)
		})

		// MCP
		r.Route("/mcp", func(r chi.Router) {
			r.Post("/proxy", h.MCPProxy)
			r.Get("/providers", h.ListMCPProviders)
		})

		// Auth
		r.Route("/auth", func(r chi.Router) {
			r.Post("/connect-token", h.GetConnectToken)
			r.Get("/connect-token", h.GetConnectToken) // Also allow GET for convenience
			r.Get("/session-token", h.GetSessionToken)
			r.Get("/oauth/callback", h.HandleOAuthCallback) // OAuth callback for Pipedream/Composio
		})

		// GitHub App
		r.Route("/github", func(r chi.Router) {
			r.Get("/install", h.HandleGitHubInstall)
			r.Get("/callback", h.HandleGitHubCallback)
			r.Post("/token", h.HandleGitHubToken)
		})

		// Apps (legacy - use /integrations instead)
		r.Get("/apps", h.ListConnectedApps)

		// Integrations - unified service management
		r.Route("/integrations", func(r chi.Router) {
			r.Get("/", h.handleListIntegrations)           // List all integrations with status
			r.Get("/{id}", h.handleGetIntegration)         // Get specific integration
			r.Post("/{id}/connect", h.handleConnectIntegration)     // Connect integration
			r.Delete("/{id}/disconnect", h.handleDisconnectIntegration) // Disconnect
			r.Get("/oauth/callback", h.handleIntegrationOAuthCallback) // OAuth2 callback
			r.Get("/agent-context", h.handleGetAgentContext)   // Get agent context
			r.Get("/sandbox-config", h.handleGetSandboxConfig) // Get sandbox config
		})

		// Cloud Credentials
		r.Route("/cloud", func(r chi.Router) {
			// Endpoints for sandboxes to fetch credentials (called by credential helpers)
			r.Post("/aws/credentials", h.handleCloudAWSCredentials)
			r.Post("/gcp/credentials", h.handleCloudGCPCredentials)

			// Endpoints for frontend to manage credentials
			r.Get("/credentials", h.handleCloudListCredentials)
			r.Post("/credentials/aws", h.handleCloudStoreAWSCredentials)
			r.Post("/credentials/gcp", h.handleCloudStoreGCPCredentials)
			r.Delete("/credentials", h.handleCloudDeleteCredentials)

			// Endpoint for getting sandbox credential configuration
			r.Post("/sandbox-config", h.handleCloudSandboxConfig)
		})
	})

	return r
}

// Cloud credential handler wrappers
func (h *Handlers) handleCloudAWSCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleGetAWSCredentials(w, r)
}

func (h *Handlers) handleCloudGCPCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleGetGCPCredentials(w, r)
}

func (h *Handlers) handleCloudListCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleListCredentials(w, r)
}

func (h *Handlers) handleCloudStoreAWSCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleStoreAWSCredentials(w, r)
}

func (h *Handlers) handleCloudStoreGCPCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleStoreGCPCredentials(w, r)
}

func (h *Handlers) handleCloudDeleteCredentials(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleDeleteCredentials(w, r)
}

func (h *Handlers) handleCloudSandboxConfig(w http.ResponseWriter, r *http.Request) {
	if h.cloudHandlers == nil {
		http.Error(w, "Cloud credentials not configured", http.StatusServiceUnavailable)
		return
	}
	h.cloudHandlers.HandleGetSandboxConfig(w, r)
}

// Integration handler wrappers
func (h *Handlers) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleListIntegrations(w, r)
}

func (h *Handlers) handleGetIntegration(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleGetIntegration(w, r)
}

func (h *Handlers) handleConnectIntegration(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleConnectIntegration(w, r)
}

func (h *Handlers) handleDisconnectIntegration(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleDisconnectIntegration(w, r)
}

func (h *Handlers) handleIntegrationOAuthCallback(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleOAuthCallback(w, r)
}

func (h *Handlers) handleGetAgentContext(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleGetAgentContext(w, r)
}

func (h *Handlers) handleGetSandboxConfig(w http.ResponseWriter, r *http.Request) {
	h.integrationHandlers.HandleGetSandboxConfig(w, r)
}
