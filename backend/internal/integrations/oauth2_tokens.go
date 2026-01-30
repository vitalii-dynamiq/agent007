package integrations

import (
	"context"
	"fmt"
	"time"
)

const oauthTokenExpiryBuffer = 2 * time.Minute

// GetOAuth2AccessToken returns a valid access token, refreshing if needed.
func (r *Registry) GetOAuth2AccessToken(ctx context.Context, userID, integrationID string) (string, error) {
	r.mu.RLock()
	ui := r.userIntegrations[userID][integrationID]
	r.mu.RUnlock()

	if ui == nil || ui.OAuth2Token == nil {
		return "", fmt.Errorf("oauth2 integration not connected: %s", integrationID)
	}

	token := ui.OAuth2Token
	if token.ExpiresAt.IsZero() || token.ExpiresAt.After(time.Now().Add(oauthTokenExpiryBuffer)) {
		return token.AccessToken, nil
	}

	handler, ok := r.GetOAuth2Handler(integrationID)
	if !ok {
		return "", fmt.Errorf("oauth2 handler not registered: %s", integrationID)
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("oauth2 refresh token missing: %s", integrationID)
	}

	refreshed, err := handler.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	if r.userIntegrations[userID] != nil {
		if current := r.userIntegrations[userID][integrationID]; current != nil {
			current.OAuth2Token = refreshed
		}
	}
	r.mu.Unlock()

	return refreshed.AccessToken, nil
}

// GetOAuth2Handler returns the handler if registered.
func (r *Registry) GetOAuth2Handler(integrationID string) (OAuth2Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.oauth2Handlers[integrationID]
	return handler, ok
}
