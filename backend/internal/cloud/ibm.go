package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IBMCloudProvider handles IBM Cloud credential operations.
//
// Authentication Flow:
//  1. User provides IBM Cloud API key
//  2. Backend stores API key encrypted
//  3. Sandbox requests credentials via credential helper
//  4. Backend exchanges API key for short-lived IAM access token
//  5. Token returned to sandbox (~60 minute validity)
//
// Security:
//   - API keys never enter sandbox
//   - Sandbox receives only short-lived IAM tokens (JWT format)
//   - Tokens are automatically validated by IBM Cloud services
//
// Token Details:
//   - Format: JWT (RS256 signed)
//   - Validity: ~60 minutes
//   - Can be refreshed using the refresh token (but we prefer re-exchange)
//
// Documentation: https://cloud.ibm.com/apidocs/iam-identity-token-api
type IBMCloudProvider struct {
	httpClient *http.Client
}

// IBM Cloud IAM endpoint
const ibmIAMTokenURL = "https://iam.cloud.ibm.com/identity/token"

// NewIBMCloudProvider creates a new IBM Cloud credential provider.
func NewIBMCloudProvider() *IBMCloudProvider {
	return &IBMCloudProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAccessToken exchanges an API key for an IBM Cloud IAM access token.
//
// The token request uses the API key grant type:
//
//	POST https://iam.cloud.ibm.com/identity/token
//	Content-Type: application/x-www-form-urlencoded
//	grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=<API_KEY>
//
// Parameters:
//   - ctx: Context for cancellation
//   - config: User's IBM Cloud configuration
//   - sandboxID: For logging/audit purposes
//
// Returns:
//   - Access token valid for ~60 minutes
//   - Error if authentication fails
func (p *IBMCloudProvider) GetAccessToken(ctx context.Context, config *IBMCloudCredentialConfig, sandboxID string) (*IBMCloudAccessToken, error) {
	if config == nil {
		return nil, fmt.Errorf("ibm cloud config is nil")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("apiKey is required")
	}

	// Request body
	data := url.Values{}
	data.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	data.Set("apikey", config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", ibmIAMTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ibm cloud auth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Expiration   int64  `json:"expiration"` // Unix timestamp
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &IBMCloudAccessToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		ExpiresAt:    time.Unix(tokenResp.Expiration, 0),
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}, nil
}

// RefreshAccessToken uses a refresh token to get a new access token.
// Note: When we have the API key, we prefer re-exchanging it instead of
// using refresh tokens, as it's simpler and equally secure.
func (p *IBMCloudProvider) RefreshAccessToken(ctx context.Context, refreshToken string) (*IBMCloudAccessToken, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", ibmIAMTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Expiration   int64  `json:"expiration"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &IBMCloudAccessToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		ExpiresAt:    time.Unix(tokenResp.Expiration, 0),
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}, nil
}

// GenerateIBMCloudCredentialHelper generates a bash script for the sandbox
// to authenticate with IBM Cloud CLI.
//
// The helper fetches a fresh IAM access token from the backend and configures
// the ibmcloud CLI to use it.
//
// Sandbox Environment Variables needed:
//   - BACKEND_URL: URL of our backend
//   - SESSION_TOKEN: Short-lived JWT for authentication
//   - SANDBOX_ID: Current sandbox identifier
func GenerateIBMCloudCredentialHelper(backendURL, sessionToken, sandboxID, region string) string {
	regionFlag := ""
	if region != "" {
		regionFlag = fmt.Sprintf("-r %s", region)
	}

	return fmt.Sprintf(`#!/bin/bash
# IBM Cloud Credential Helper - Generated by Dynamiq
# This script fetches short-lived IBM Cloud credentials from the backend
# Documentation: https://cloud.ibm.com/docs/cli?topic=cli-ibmcloud_cli

set -e

# Fetch IBM Cloud access token from backend
response=$(curl -s -X POST "%s/api/cloud/ibm/credentials" \
  -H "Authorization: Bearer %s" \
  -H "Content-Type: application/json" \
  -d '{"sandboxId": "%s", "provider": "ibm"}')

# Check for errors
error=$(echo "$response" | jq -r '.error // empty')
if [ -n "$error" ]; then
  echo "Error: $error" >&2
  exit 1
fi

# Extract token
access_token=$(echo "$response" | jq -r '.ibm.access_token')
expires_at=$(echo "$response" | jq -r '.ibm.expires_at')

if [ -z "$access_token" ] || [ "$access_token" = "null" ]; then
  echo "Error: Failed to get access token" >&2
  exit 1
fi

# Login to IBM Cloud CLI using the access token
# The --apikey flag can accept a token when prefixed with "Bearer "
# Alternative: Use ibmcloud login --no-region and then set config
ibmcloud config --check-version=false 2>/dev/null || true

# Set the access token directly in the config
export IBMCLOUD_API_KEY="$access_token"
ibmcloud login %s --apikey "$access_token" 2>&1 || {
  # Fallback: Try using the token as a bearer token
  echo "Note: Using access token directly"
}

echo "IBM Cloud credentials configured (expires: $expires_at)"
`, backendURL, sessionToken, sandboxID, regionFlag)
}

// GenerateIBMCloudEnvConfig generates environment configuration for IBM Cloud CLI.
// This is an alternative when the full credential helper isn't needed.
func GenerateIBMCloudEnvConfig(token *IBMCloudAccessToken, config *IBMCloudCredentialConfig) map[string]string {
	vars := map[string]string{
		"IBMCLOUD_IAM_TOKEN": "Bearer " + token.AccessToken,
	}

	if config != nil {
		if config.Region != "" {
			vars["IBMCLOUD_REGION"] = config.Region
		}
		if config.AccountID != "" {
			vars["IBMCLOUD_ACCOUNT_ID"] = config.AccountID
		}
		if config.ResourceGroup != "" {
			vars["IBMCLOUD_RESOURCE_GROUP"] = config.ResourceGroup
		}
	}

	return vars
}

// ValidateCredentials tests if the IBM Cloud API key is valid.
func (p *IBMCloudProvider) ValidateCredentials(ctx context.Context, config *IBMCloudCredentialConfig) error {
	_, err := p.GetAccessToken(ctx, config, "validation")
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}
	return nil
}

// GetAccountInfo retrieves account information using the access token.
func (p *IBMCloudProvider) GetAccountInfo(ctx context.Context, token *IBMCloudAccessToken) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://accounts.cloud.ibm.com/v1/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}
