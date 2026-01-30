package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

const (
	gcpTokenEndpoint        = "https://oauth2.googleapis.com/token"
	gcpSTSEndpoint          = "https://sts.googleapis.com/v1/token"
	gcpIAMCredentialsAPI    = "https://iamcredentials.googleapis.com/v1"
	defaultGCPTokenDuration = time.Hour
	maxGCPTokenDuration     = 12 * time.Hour
)

// Default scopes for GCP access
var defaultGCPScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

// GCPProvider handles GCP credential operations
type GCPProvider struct {
	httpClient *http.Client
}

// NewGCPProvider creates a new GCP provider
func NewGCPProvider() *GCPProvider {
	return &GCPProvider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAccessTokenForSandbox returns a GCP access token for a sandbox session
// This is the main entry point called by the credential endpoint
func (p *GCPProvider) GetAccessTokenForSandbox(ctx context.Context, config *GCPCredentialConfig, sandboxID, userID string) (*GCPAccessToken, error) {
	// If impersonation is configured, use workload identity flow
	if config.ImpersonateServiceAccount != "" {
		return p.getTokenViaImpersonation(ctx, config, sandboxID)
	}

	// Otherwise, generate token directly from service account
	return p.getTokenFromServiceAccount(ctx, config)
}

// getTokenFromServiceAccount generates an access token directly from a service account key
func (p *GCPProvider) getTokenFromServiceAccount(ctx context.Context, config *GCPCredentialConfig) (*GCPAccessToken, error) {
	if config.ServiceAccountJSON == "" {
		return nil, fmt.Errorf("service account JSON is required")
	}

	// Parse the service account JSON
	creds, err := google.CredentialsFromJSON(ctx, []byte(config.ServiceAccountJSON), p.getScopes(config)...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account JSON: %w", err)
	}

	// Get a token
	token, err := creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	return &GCPAccessToken{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresIn:   int(time.Until(token.Expiry).Seconds()),
		ExpiresAt:   token.Expiry,
	}, nil
}

// getTokenViaImpersonation uses the service account to impersonate another service account
// This provides an additional layer of security by using short-lived tokens
func (p *GCPProvider) getTokenViaImpersonation(ctx context.Context, config *GCPCredentialConfig, sandboxID string) (*GCPAccessToken, error) {
	// First, get a token for the source service account
	sourceToken, err := p.getTokenFromServiceAccount(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to get source token: %w", err)
	}

	// Use the source token to generate a token for the target service account
	targetSA := config.ImpersonateServiceAccount
	if !strings.HasSuffix(targetSA, ".iam.gserviceaccount.com") {
		// Add the suffix if not present
		targetSA = targetSA + ".iam.gserviceaccount.com"
	}

	// Build the request to generate an access token
	url := fmt.Sprintf("%s/projects/-/serviceAccounts/%s:generateAccessToken",
		gcpIAMCredentialsAPI, targetSA)

	reqBody := map[string]interface{}{
		"scope":    p.getScopes(config),
		"lifetime": fmt.Sprintf("%ds", int(defaultGCPTokenDuration.Seconds())),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sourceToken.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate access token: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	expireTime, _ := time.Parse(time.RFC3339, result.ExpireTime)

	return &GCPAccessToken{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(time.Until(expireTime).Seconds()),
		ExpiresAt:   expireTime,
	}, nil
}

// ExchangeSubjectToken exchanges a subject token (OIDC JWT) for a GCP access token
// This is used for workload identity federation where the sandbox presents our JWT
// and exchanges it for GCP credentials
func (p *GCPProvider) ExchangeSubjectToken(ctx context.Context, subjectToken string, audience string, config *GCPCredentialConfig) (*GCPAccessToken, error) {
	// Build the STS token exchange request
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	data.Set("subject_token", subjectToken)
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")

	if audience != "" {
		data.Set("audience", audience)
	}

	scopes := p.getScopes(config)
	if len(scopes) > 0 {
		data.Set("scope", strings.Join(scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", gcpSTSEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("STS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("STS request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken     string `json:"access_token"`
		TokenType       string `json:"token_type"`
		ExpiresIn       int    `json:"expires_in"`
		IssuedTokenType string `json:"issued_token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &GCPAccessToken{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   result.ExpiresIn,
		ExpiresAt:   time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// GetServiceAccountInfo extracts info from a service account JSON
func (p *GCPProvider) GetServiceAccountInfo(saJSON string) (email string, projectID string, err error) {
	var sa struct {
		ClientEmail string `json:"client_email"`
		ProjectID   string `json:"project_id"`
	}

	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", "", fmt.Errorf("failed to parse service account JSON: %w", err)
	}

	return sa.ClientEmail, sa.ProjectID, nil
}

// ValidateServiceAccount validates that a service account JSON is valid
func (p *GCPProvider) ValidateServiceAccount(ctx context.Context, config *GCPCredentialConfig) error {
	if config.ServiceAccountJSON == "" {
		return fmt.Errorf("service account JSON is required")
	}

	// Try to parse the JSON
	creds, err := google.CredentialsFromJSON(ctx, []byte(config.ServiceAccountJSON), defaultGCPScopes...)
	if err != nil {
		return fmt.Errorf("invalid service account JSON: %w", err)
	}

	// Try to get a token to validate the credentials work
	_, err = creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}

	return nil
}

// getScopes returns the scopes to use for token generation
func (p *GCPProvider) getScopes(config *GCPCredentialConfig) []string {
	if len(config.Scopes) > 0 {
		return config.Scopes
	}
	return defaultGCPScopes
}

// FormatGCPCredentialConfig formats a GCP credential config for the SDK
// This creates a JSON that can be used with GOOGLE_APPLICATION_CREDENTIALS
// or passed to the gcloud CLI
func FormatGCPCredentialConfig(token *GCPAccessToken) (string, error) {
	// This format is for external account credentials
	// https://google.aip.dev/auth/4117
	config := map[string]interface{}{
		"type": "external_account",
		"audience": "//iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID",
		"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
		"token_url": gcpSTSEndpoint,
		"credential_source": map[string]interface{}{
			"executable": map[string]interface{}{
				"command": "/usr/local/bin/mcp-credential-helper gcp",
				"timeout_millis": 5000,
				"output_file": "/tmp/gcp_token.json",
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// FormatExecutableCredentialOutput formats the output for GCP executable credential source
// This is what the credential helper script should output
func FormatExecutableCredentialOutput(token *GCPAccessToken) (string, error) {
	output := map[string]interface{}{
		"success": true,
		"version": 1,
		"token_type": token.TokenType,
		"id_token": token.AccessToken, // For OIDC tokens
		"expiration_time": token.ExpiresAt.Unix(),
	}

	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// CreateWorkloadIdentityConfig creates a workload identity configuration file
// that can be used with GOOGLE_APPLICATION_CREDENTIALS
func CreateWorkloadIdentityConfig(backendURL, sandboxToken string, projectNumber, poolID, providerID string) (string, error) {
	config := map[string]interface{}{
		"type": "external_account",
		"audience": fmt.Sprintf("//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
			projectNumber, poolID, providerID),
		"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
		"token_url": gcpSTSEndpoint,
		"credential_source": map[string]interface{}{
			"url": fmt.Sprintf("%s/api/cloud/gcp/token", backendURL),
			"headers": map[string]string{
				"Authorization": "Bearer " + sandboxToken,
			},
			"format": map[string]interface{}{
				"type": "json",
				"subject_token_field_name": "token",
			},
		},
		"service_account_impersonation_url": "",
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
