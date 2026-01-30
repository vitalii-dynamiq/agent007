package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dynamiq/manus-like/internal/auth"
)

// Manager orchestrates cloud credential operations
type Manager struct {
	store        *CredentialStore
	awsProvider  *AWSProvider
	gcpProvider  *GCPProvider
	tokenManager *auth.TokenManager
	backendURL   string
}

// NewManager creates a new cloud credential manager
func NewManager(encryptionKey string, tokenManager *auth.TokenManager, backendURL string) (*Manager, error) {
	store, err := NewCredentialStore(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	return &Manager{
		store:        store,
		awsProvider:  NewAWSProvider("", ""), // Will use default credentials or user-provided
		gcpProvider:  NewGCPProvider(),
		tokenManager: tokenManager,
		backendURL:   backendURL,
	}, nil
}

// SetAWSDefaultCredentials sets default AWS credentials for assuming roles
func (m *Manager) SetAWSDefaultCredentials(accessKeyID, secretAccessKey string) {
	m.awsProvider = NewAWSProvider(accessKeyID, secretAccessKey)
}

// StoreAWSCredentials stores AWS credentials for a user
func (m *Manager) StoreAWSCredentials(userID, name string, config *AWSCredentialConfig) error {
	return m.store.StoreAWSCredentials(userID, name, config)
}

// StoreGCPCredentials stores GCP credentials for a user
func (m *Manager) StoreGCPCredentials(userID, name string, config *GCPCredentialConfig) error {
	// Validate the credentials first
	if err := m.gcpProvider.ValidateServiceAccount(context.Background(), config); err != nil {
		return fmt.Errorf("invalid GCP credentials: %w", err)
	}
	return m.store.StoreGCPCredentials(userID, name, config)
}

// GetCredentials returns credentials for a sandbox based on session token
func (m *Manager) GetCredentials(ctx context.Context, req *CredentialRequest) (*CredentialResponse, error) {
	// Validate the session token
	claims, err := m.tokenManager.ValidateSessionToken(req.SessionToken)
	if err != nil {
		return nil, fmt.Errorf("invalid session token: %w", err)
	}

	// Verify sandbox ID matches
	if claims.SandboxID != req.SandboxID {
		return nil, fmt.Errorf("sandbox ID mismatch")
	}

	userID := claims.UserID

	switch req.Provider {
	case ProviderAWS:
		return m.getAWSCredentials(ctx, userID, req.SandboxID)
	case ProviderGCP:
		return m.getGCPCredentials(ctx, userID, req.SandboxID)
	default:
		return nil, fmt.Errorf("unknown provider: %s", req.Provider)
	}
}

// getAWSCredentials retrieves AWS credentials for a sandbox
func (m *Manager) getAWSCredentials(ctx context.Context, userID, sandboxID string) (*CredentialResponse, error) {
	// Get user's AWS config
	config, err := m.store.GetAWSCredentials(userID)
	if err != nil {
		return &CredentialResponse{
			Provider: ProviderAWS,
			Error:    err.Error(),
		}, nil
	}

	// Get temporary credentials
	creds, err := m.awsProvider.GetCredentialsForSandbox(ctx, config, sandboxID, userID)
	if err != nil {
		log.Printf("Failed to get AWS credentials for user %s: %v", userID, err)
		return &CredentialResponse{
			Provider: ProviderAWS,
			Error:    err.Error(),
		}, nil
	}

	return &CredentialResponse{
		Provider: ProviderAWS,
		AWS:      creds,
	}, nil
}

// getGCPCredentials retrieves GCP credentials for a sandbox
func (m *Manager) getGCPCredentials(ctx context.Context, userID, sandboxID string) (*CredentialResponse, error) {
	// Get user's GCP config
	config, err := m.store.GetGCPCredentials(userID)
	if err != nil {
		return &CredentialResponse{
			Provider: ProviderGCP,
			Error:    err.Error(),
		}, nil
	}

	// Get access token
	token, err := m.gcpProvider.GetAccessTokenForSandbox(ctx, config, sandboxID, userID)
	if err != nil {
		log.Printf("Failed to get GCP credentials for user %s: %v", userID, err)
		return &CredentialResponse{
			Provider: ProviderGCP,
			Error:    err.Error(),
		}, nil
	}

	return &CredentialResponse{
		Provider: ProviderGCP,
		GCP:      token,
	}, nil
}

// ListCredentials lists all credentials for a user (without sensitive data)
func (m *Manager) ListCredentials(userID string) []UserCloudCredentials {
	return m.store.ListCredentials(userID)
}

// DeleteCredentials deletes credentials for a user and provider
func (m *Manager) DeleteCredentials(userID string, provider ProviderType) error {
	return m.store.DeleteCredentials(userID, provider)
}

// HasCredentials checks if a user has credentials for a provider
func (m *Manager) HasCredentials(userID string, provider ProviderType) bool {
	return m.store.HasCredentials(userID, provider)
}

// GenerateSandboxCredentialConfig generates the credential helper configuration
// to be injected into a sandbox
func (m *Manager) GenerateSandboxCredentialConfig(userID, sandboxID, conversationID string) (*SandboxCredentialConfig, error) {
	// Generate a session token for the sandbox
	scopes := []auth.Scope{
		auth.ScopeListTools,
		auth.ScopeCallTools,
	}
	sessionToken, err := m.tokenManager.GenerateSessionTokenWithScopes(userID, conversationID, sandboxID, scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	config := &SandboxCredentialConfig{
		BackendURL:   m.backendURL,
		SessionToken: sessionToken,
		SandboxID:    sandboxID,
	}

	// Check which providers the user has configured
	if m.store.HasCredentials(userID, ProviderAWS) {
		awsRegion := "us-east-1"
		if awsConfig, err := m.store.GetAWSCredentials(userID); err == nil {
			if awsConfig.Region != "" {
				awsRegion = awsConfig.Region
			}
		}
		config.AWSEnabled = true
		config.AWSCredentialHelper = m.generateAWSCredentialHelper(sessionToken, sandboxID)
		config.AWSConfig = m.generateAWSConfig(awsRegion)
	}

	if m.store.HasCredentials(userID, ProviderGCP) {
		config.GCPEnabled = true
		config.GCPCredentialHelper = m.generateGCPCredentialHelper(sessionToken, sandboxID)
		config.GCPConfig = m.generateGCPConfig(sessionToken, sandboxID)
	}

	return config, nil
}

// SandboxCredentialConfig contains all the configuration needed for a sandbox
// to use cloud credentials
type SandboxCredentialConfig struct {
	BackendURL   string `json:"backendUrl"`
	SessionToken string `json:"sessionToken"`
	SandboxID    string `json:"sandboxId"`

	// AWS
	AWSEnabled          bool   `json:"awsEnabled"`
	AWSCredentialHelper string `json:"awsCredentialHelper,omitempty"` // Shell script
	AWSConfig           string `json:"awsConfig,omitempty"`           // ~/.aws/config content

	// GCP
	GCPEnabled          bool   `json:"gcpEnabled"`
	GCPCredentialHelper string `json:"gcpCredentialHelper,omitempty"` // Shell script
	GCPConfig           string `json:"gcpConfig,omitempty"`           // Application default credentials JSON
}

// generateAWSCredentialHelper generates the credential_process script for AWS
func (m *Manager) generateAWSCredentialHelper(sessionToken, sandboxID string) string {
	return fmt.Sprintf(`#!/usr/bin/env python3
# AWS Credential Helper - fetches short-lived credentials from backend
# This script is called by AWS CLI/SDK via credential_process

import json
import sys
import urllib.request
import urllib.error

BACKEND_URL = "%s"
SESSION_TOKEN = "%s"
SANDBOX_ID = "%s"

payload = json.dumps({"sandboxId": SANDBOX_ID, "provider": "aws"}).encode()
req = urllib.request.Request(
    f"{BACKEND_URL}/api/cloud/aws/credentials",
    data=payload,
    headers={
        "Content-Type": "application/json",
        "Authorization": "Bearer " + SESSION_TOKEN,
    },
)

try:
    with urllib.request.urlopen(req, timeout=20) as resp:
        body = resp.read()
except urllib.error.HTTPError as e:
    error_body = e.read()
    try:
        parsed = json.loads(error_body)
        error = parsed.get("error") or parsed.get("message") or str(e)
    except Exception:
        error = error_body.decode("utf-8", "ignore") or str(e)
    print(f"Error: {error}", file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)

try:
    resp = json.loads(body)
except Exception:
    text = body.decode("utf-8", "ignore")
    print(f"Error: {text or 'Invalid response from backend'}", file=sys.stderr)
    sys.exit(1)
error = resp.get("error")
if error:
    print(f"Error: {error}", file=sys.stderr)
    sys.exit(1)

aws = resp.get("aws") or {}
output = {
    "Version": 1,
    "AccessKeyId": aws.get("AccessKeyId"),
    "SecretAccessKey": aws.get("SecretAccessKey"),
    "SessionToken": aws.get("SessionToken"),
    "Expiration": aws.get("Expiration"),
}
print(json.dumps(output))
`, m.backendURL, sessionToken, sandboxID)
}

// generateAWSConfig generates the ~/.aws/config content
func (m *Manager) generateAWSConfig(region string) string {
	if region == "" {
		region = "us-east-1"
	}
	return fmt.Sprintf(`[default]
credential_process = /usr/local/bin/aws-credential-helper
region = %s

[profile sandbox]
credential_process = /usr/local/bin/aws-credential-helper
region = %s
`, region, region)
}

// generateGCPCredentialHelper generates the credential helper script for GCP
func (m *Manager) generateGCPCredentialHelper(sessionToken, sandboxID string) string {
	return fmt.Sprintf(`#!/bin/bash
# GCP Credential Helper - fetches short-lived access tokens from backend
# This script is called by gcloud/SDK via external account credentials

set -e

BACKEND_URL="%s"
SESSION_TOKEN="%s"
SANDBOX_ID="%s"

# Request token from backend
response=$(curl -s -X POST "${BACKEND_URL}/api/cloud/gcp/credentials" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${SESSION_TOKEN}" \
  -d "{\"sandboxId\": \"${SANDBOX_ID}\", \"provider\": \"gcp\"}")

# Check for errors
error=$(echo "$response" | jq -r '.error // empty')
if [ -n "$error" ]; then
  echo "Error: $error" >&2
  exit 1
fi

# Output token
echo "$response" | jq -r '.gcp.access_token'
`, m.backendURL, sessionToken, sandboxID)
}

// generateGCPConfig generates the application default credentials JSON
func (m *Manager) generateGCPConfig(sessionToken, sandboxID string) string {
	config := map[string]interface{}{
		"type":               "external_account",
		"audience":           "//iam.googleapis.com/locations/global/workloadIdentityPools/dynamiq-pool/providers/dynamiq-provider",
		"subject_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"token_url":          "https://sts.googleapis.com/v1/token",
		"credential_source": map[string]interface{}{
			"executable": map[string]interface{}{
				"command":        "/usr/local/bin/gcp-credential-helper",
				"timeout_millis": 5000,
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	return string(data)
}

// GetSubjectTokenForSandbox generates an OIDC-style subject token for a sandbox
// This token can be exchanged with GCP STS for a real access token
func (m *Manager) GetSubjectTokenForSandbox(userID, sandboxID, conversationID string) (string, error) {
	// Generate a short-lived token that identifies this sandbox session
	scopes := []auth.Scope{"cloud:gcp:token"}
	return m.tokenManager.GenerateSessionTokenWithScopes(userID, conversationID, sandboxID, scopes)
}
