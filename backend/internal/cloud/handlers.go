package cloud

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Handlers contains HTTP handlers for cloud credential operations
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new cloud handlers
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// HandleGetAWSCredentials handles requests for AWS credentials from sandboxes
func (h *Handlers) HandleGetAWSCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get session token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		req.SessionToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if req.SessionToken == "" {
		http.Error(w, "Missing session token", http.StatusUnauthorized)
		return
	}

	req.Provider = ProviderAWS

	resp, err := h.manager.GetCredentials(r.Context(), &req)
	if err != nil {
		log.Printf("Failed to get AWS credentials: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleGetGCPCredentials handles requests for GCP credentials from sandboxes
func (h *Handlers) HandleGetGCPCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get session token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		req.SessionToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if req.SessionToken == "" {
		http.Error(w, "Missing session token", http.StatusUnauthorized)
		return
	}

	req.Provider = ProviderGCP

	resp, err := h.manager.GetCredentials(r.Context(), &req)
	if err != nil {
		log.Printf("Failed to get GCP credentials: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleStoreAWSCredentials handles storing AWS credentials
func (h *Handlers) HandleStoreAWSCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID (in production, this would come from authentication)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	var req struct {
		Name            string `json:"name"`
		AccountID       string `json:"accountId,omitempty"`
		RoleARN         string `json:"roleArn"`
		ExternalID      string `json:"externalId,omitempty"`
		Region          string `json:"region,omitempty"`
		AccessKeyID     string `json:"accessKeyId,omitempty"`
		SecretAccessKey string `json:"secretAccessKey,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	roleProvided := req.RoleARN != ""
	accessKeysProvided := req.AccessKeyID != "" && req.SecretAccessKey != ""
	if !roleProvided && !accessKeysProvided {
		http.Error(w, "roleArn or accessKeyId/secretAccessKey is required", http.StatusBadRequest)
		return
	}
	if accessKeysProvided && req.AccountID == "" && !roleProvided {
		http.Error(w, "accountId is required when using access keys", http.StatusBadRequest)
		return
	}

	config := &AWSCredentialConfig{
		AccountID:       req.AccountID,
		RoleARN:         req.RoleARN,
		ExternalID:      req.ExternalID,
		Region:          req.Region,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
	}

	// Validate credentials by requesting a short-lived session
	if _, err := h.manager.awsProvider.GetCredentialsForSandbox(r.Context(), config, "validate", userID); err != nil {
		log.Printf("AWS credential validation failed: %v", err)
		http.Error(w, "Failed to validate AWS credentials: "+err.Error(), http.StatusBadRequest)
		return
	}

	name := req.Name
	if name == "" {
		if req.AccountID != "" {
			name = "AWS " + req.AccountID
		} else {
			name = "AWS Credentials"
		}
	}

	if err := h.manager.StoreAWSCredentials(userID, name, config); err != nil {
		log.Printf("Failed to store AWS credentials: %v", err)
		http.Error(w, "Failed to store credentials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "AWS credentials stored successfully",
	})
}

// HandleStoreGCPCredentials handles storing GCP credentials
func (h *Handlers) HandleStoreGCPCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID (in production, this would come from authentication)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	var req struct {
		Name                      string   `json:"name"`
		ServiceAccountJSON        string   `json:"serviceAccountJson"`
		ProjectID                 string   `json:"projectId,omitempty"`
		ImpersonateServiceAccount string   `json:"impersonateServiceAccount,omitempty"`
		Scopes                    []string `json:"scopes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ServiceAccountJSON == "" {
		http.Error(w, "serviceAccountJson is required", http.StatusBadRequest)
		return
	}

	config := &GCPCredentialConfig{
		ServiceAccountJSON:        req.ServiceAccountJSON,
		ProjectID:                 req.ProjectID,
		ImpersonateServiceAccount: req.ImpersonateServiceAccount,
		Scopes:                    req.Scopes,
	}

	name := req.Name
	if name == "" {
		name = "GCP Credentials"
	}

	if err := h.manager.StoreGCPCredentials(userID, name, config); err != nil {
		log.Printf("Failed to store GCP credentials: %v", err)
		http.Error(w, "Failed to store credentials: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract project ID from service account if not provided
	email, projectID, _ := h.manager.gcpProvider.GetServiceAccountInfo(req.ServiceAccountJSON)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"message":             "GCP credentials stored successfully",
		"serviceAccountEmail": email,
		"projectId":           projectID,
	})
}

// HandleStorePostgresCredentials handles storing PostgreSQL credentials
func (h *Handlers) HandleStorePostgresCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	var req struct {
		Name     string `json:"name"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
		SSLMode  string `json:"sslMode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Host == "" || req.Database == "" || req.Username == "" {
		http.Error(w, "host, database, and username are required", http.StatusBadRequest)
		return
	}

	config := &PostgresCredentialConfig{
		Host:           req.Host,
		Port:           req.Port,
		Database:       req.Database,
		Username:       req.Username,
		Password:       req.Password,
		SSLMode:        req.SSLMode,
		ConnectionName: req.Name,
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("PostgreSQL %s@%s", req.Database, req.Host)
	}

	if err := h.manager.StorePostgresCredentials(userID, name, config); err != nil {
		log.Printf("Failed to store PostgreSQL credentials: %v", err)
		http.Error(w, "Failed to store credentials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "PostgreSQL credentials stored successfully",
	})
}

// HandleListCredentials lists all credentials for a user
func (h *Handlers) HandleListCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	creds := h.manager.ListCredentials(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"credentials": creds,
	})
}

// HandleDeleteCredentials deletes credentials for a user
func (h *Handlers) HandleDeleteCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default-user"
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "provider query parameter is required", http.StatusBadRequest)
		return
	}

	var providerType ProviderType
	switch provider {
	case "aws":
		providerType = ProviderAWS
	case "gcp":
		providerType = ProviderGCP
	case "postgres":
		providerType = ProviderPostgres
	default:
		http.Error(w, "invalid provider", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeleteCredentials(userID, providerType); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Credentials deleted successfully",
	})
}

// HandleGetSandboxConfig returns the credential configuration for a sandbox
func (h *Handlers) HandleGetSandboxConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID         string `json:"userId"`
		SandboxID      string `json:"sandboxId"`
		ConversationID string `json:"conversationId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SandboxID == "" {
		http.Error(w, "sandboxId is required", http.StatusBadRequest)
		return
	}

	userID := req.UserID
	if userID == "" {
		userID = "default-user"
	}

	config, err := h.manager.GenerateSandboxCredentialConfig(userID, req.SandboxID, req.ConversationID)
	if err != nil {
		log.Printf("Failed to generate sandbox config: %v", err)
		http.Error(w, "Failed to generate sandbox configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
