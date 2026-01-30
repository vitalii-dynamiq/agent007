package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dynamiq/manus-like/internal/integrations"
)

func (h *Handlers) HandleGitHubInstall(w http.ResponseWriter, r *http.Request) {
	if h.githubApp == nil {
		http.Error(w, "GitHub App not configured", http.StatusServiceUnavailable)
		return
	}

	userID := getUserID(r)
	state := buildGitHubState(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"authUrl": h.githubApp.InstallURL(state),
		"state":   state,
	})
}

func (h *Handlers) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if h.githubApp == nil {
		http.Error(w, "GitHub App not configured", http.StatusServiceUnavailable)
		return
	}

	installationIDStr := r.URL.Query().Get("installation_id")
	state := r.URL.Query().Get("state")

	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	if err != nil || installationID == 0 {
		http.Error(w, "Invalid installation_id", http.StatusBadRequest)
		return
	}

	userID, integrationID, ok := parseGitHubState(state)
	if !ok || integrationID != "github" {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	installation, err := h.githubApp.GetInstallation(r.Context(), installationID)
	if err != nil {
		http.Error(w, "Failed to fetch installation: "+err.Error(), http.StatusBadRequest)
		return
	}

	ui := &integrations.UserIntegration{
		AccountName:         installation.Account.Login,
		AccountID:           installation.Account.Login,
		Organization:        installation.Account.Login,
		GitHubInstallationID: installationID,
	}

	if err := h.integrationRegistry.ConnectIntegration(userID, "github", ui); err != nil {
		http.Error(w, "Failed to save GitHub installation", http.StatusInternalServerError)
		return
	}

	redirect := h.config.FrontendURL + "?oauth=success&app=github"
	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

func (h *Handlers) HandleGitHubToken(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing session token", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.tokenManager.ValidateSessionToken(token)
	if err != nil {
		http.Error(w, "Invalid session token", http.StatusUnauthorized)
		return
	}

	ui, connected := h.integrationRegistry.GetUserIntegration(claims.UserID, "github")
	if !connected || ui == nil || ui.GitHubInstallationID == 0 {
		// Allow OAuth-based connections without installation IDs
		if ui == nil || !connected {
			http.Error(w, "GitHub not connected", http.StatusNotFound)
			return
		}
	}

	if ui.OAuth2Token != nil && ui.OAuth2Token.AccessToken != "" {
		accessToken, err := h.integrationRegistry.GetOAuth2AccessToken(r.Context(), claims.UserID, "github")
		if err != nil {
			http.Error(w, "Failed to refresh GitHub token: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"token":     accessToken,
			"account":   ui.AccountName,
			"accountId": ui.AccountID,
			"source":    "oauth",
		}
		if ui.OAuth2Token != nil && !ui.OAuth2Token.ExpiresAt.IsZero() {
			resp["expiresAt"] = ui.OAuth2Token.ExpiresAt.Format(time.RFC3339)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if h.githubApp == nil {
		http.Error(w, "GitHub App not configured", http.StatusServiceUnavailable)
		return
	}
	if ui.GitHubInstallationID == 0 {
		http.Error(w, "GitHub installation not found", http.StatusNotFound)
		return
	}

	accessToken, err := h.githubApp.CreateInstallationToken(r.Context(), ui.GitHubInstallationID)
	if err != nil {
		http.Error(w, "Failed to generate GitHub token: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      accessToken.Token,
		"expiresAt":  accessToken.ExpiresAt.Format(time.RFC3339),
		"account":    ui.AccountName,
		"accountId":  ui.AccountID,
		"installId":  ui.GitHubInstallationID,
		"source":     "installation",
	})
}

func buildGitHubState(userID string) string {
	return userID + ":github"
}

func parseGitHubState(state string) (userID, integrationID string, ok bool) {
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
