package cloud

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OracleCloudProvider handles Oracle Cloud Infrastructure (OCI) credential operations.
//
// Authentication Flow:
//  1. User provides OCI API key (tenancy OCID, user OCID, fingerprint, private key)
//  2. Backend stores credentials encrypted
//  3. Sandbox requests credentials via credential helper
//  4. Backend generates a session token using OCI session token service
//  5. Session token returned to sandbox (5-60 minute validity)
//
// Security:
//   - Private keys never enter sandbox
//   - Sandbox receives only short-lived session tokens
//   - Session tokens have configurable TTL (5-60 minutes)
//   - Each session gets an ephemeral key pair
//
// Token Details:
//   - Session tokens created via `oci session authenticate`
//   - TTL: Configurable 5-60 minutes (default 60)
//   - Requires ephemeral RSA key pair for request signing
//
// Documentation: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm
type OracleCloudProvider struct {
	httpClient *http.Client
}

// NewOracleCloudProvider creates a new Oracle Cloud credential provider.
func NewOracleCloudProvider() *OracleCloudProvider {
	return &OracleCloudProvider{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// GetSessionToken creates an OCI session token for the sandbox.
//
// Unlike AWS/Azure which use standard OAuth flows, OCI uses request signing
// with the user's private key. For sandbox security, we:
// 1. Generate an ephemeral RSA key pair
// 2. Use the user's private key to sign a session creation request
// 3. Return the session token + ephemeral private key to sandbox
// 4. Sandbox uses ephemeral key for subsequent requests
//
// Parameters:
//   - ctx: Context for cancellation
//   - config: User's OCI configuration
//   - sandboxID: For logging/audit purposes
//   - expirationMinutes: Session validity (5-60 minutes)
//
// Returns:
//   - Session token and ephemeral private key
//   - Error if authentication fails
func (p *OracleCloudProvider) GetSessionToken(ctx context.Context, config *OracleCloudCredentialConfig, sandboxID string, expirationMinutes int) (*OracleCloudSessionToken, error) {
	if config == nil {
		return nil, fmt.Errorf("oracle cloud config is nil")
	}
	if config.TenancyOCID == "" || config.UserOCID == "" || config.Fingerprint == "" {
		return nil, fmt.Errorf("tenancyOcid, userOcid, and fingerprint are required")
	}
	if config.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("privateKeyPem is required")
	}

	// Validate expiration
	if expirationMinutes < 5 || expirationMinutes > 60 {
		expirationMinutes = 60 // Default to max
	}

	// Parse the user's private key
	privateKey, err := parsePrivateKey(config.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	// Generate ephemeral key pair for the session
	ephemeralKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}

	// Get region endpoint
	region := config.Region
	if region == "" {
		region = "us-ashburn-1" // Default region
	}

	// Create signed session request
	// OCI uses request signing with the user's private key
	token, err := p.createSessionToken(ctx, config, privateKey, ephemeralKey, region, expirationMinutes)
	if err != nil {
		return nil, fmt.Errorf("create session token: %w", err)
	}

	// Encode ephemeral private key for sandbox
	ephemeralKeyPEM := encodePrivateKey(ephemeralKey)

	return &OracleCloudSessionToken{
		Token:      token,
		PrivateKey: ephemeralKeyPEM,
		Region:     region,
		ExpiresAt:  time.Now().Add(time.Duration(expirationMinutes) * time.Minute),
	}, nil
}

// createSessionToken creates an OCI session using the API.
// This is a simplified implementation - production should use OCI SDK.
func (p *OracleCloudProvider) createSessionToken(
	ctx context.Context,
	config *OracleCloudCredentialConfig,
	privateKey *rsa.PrivateKey,
	ephemeralKey *rsa.PrivateKey,
	region string,
	expirationMinutes int,
) (string, error) {
	// For a full implementation, use the OCI SDK.
	// This is a placeholder showing the request structure.

	// OCI session endpoint
	endpoint := fmt.Sprintf("https://auth.%s.oraclecloud.com/v1/authentication/generateScopedAccessToken", region)

	// Encode ephemeral public key
	ephemeralPubKeyDER, err := x509.MarshalPKIXPublicKey(&ephemeralKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshal ephemeral public key: %w", err)
	}
	ephemeralPubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: ephemeralPubKeyDER,
	})

	// Request body
	reqBody := map[string]interface{}{
		"publicKey":               string(ephemeralPubKeyPEM),
		"sessionExpirationInMins": expirationMinutes,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Sign the request using OCI request signing
	if err := p.signRequest(req, config, privateKey, bodyBytes); err != nil {
		return "", fmt.Errorf("sign request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("session request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("session creation failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return tokenResp.Token, nil
}

// signRequest signs an OCI API request using the RSA-SHA256 signature.
// OCI uses a custom HTTP signature scheme.
// Documentation: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/signingrequests.htm
func (p *OracleCloudProvider) signRequest(req *http.Request, config *OracleCloudCredentialConfig, privateKey *rsa.PrivateKey, body []byte) error {
	// Generate date header
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	// Calculate body hash for POST/PUT
	var bodyHash string
	if len(body) > 0 {
		hash := sha256.Sum256(body)
		bodyHash = base64.StdEncoding.EncodeToString(hash[:])
		req.Header.Set("x-content-sha256", bodyHash)
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}

	// Build signing string
	// Format: (request-target): post /v1/authentication/generateScopedAccessToken
	//         date: <date>
	//         host: <host>
	//         x-content-sha256: <hash>
	//         content-length: <len>
	//         content-type: application/json
	var signingString strings.Builder
	signingString.WriteString(fmt.Sprintf("(request-target): %s %s\n", strings.ToLower(req.Method), req.URL.Path))
	signingString.WriteString(fmt.Sprintf("date: %s\n", date))
	signingString.WriteString(fmt.Sprintf("host: %s", req.URL.Host))

	if len(body) > 0 {
		signingString.WriteString(fmt.Sprintf("\nx-content-sha256: %s", bodyHash))
		signingString.WriteString(fmt.Sprintf("\ncontent-length: %d", len(body)))
		signingString.WriteString("\ncontent-type: application/json")
	}

	// Sign the string
	hashed := sha256.Sum256([]byte(signingString.String()))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	// Build authorization header
	keyID := fmt.Sprintf("%s/%s/%s", config.TenancyOCID, config.UserOCID, config.Fingerprint)
	headers := "(request-target) date host"
	if len(body) > 0 {
		headers += " x-content-sha256 content-length content-type"
	}

	auth := fmt.Sprintf(
		`Signature version="1",keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		keyID,
		headers,
		base64.StdEncoding.EncodeToString(signature),
	)
	req.Header.Set("Authorization", auth)

	return nil
}

// parsePrivateKey parses a PEM-encoded RSA private key.
func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS8 first
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("key is not RSA")
	}

	// Fall back to PKCS1
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return rsaKey, nil
}

// encodePrivateKey encodes an RSA private key to PEM format.
func encodePrivateKey(key *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

// GenerateOCIConfig generates OCI configuration files for the sandbox.
//
// Creates:
//   - ~/.oci/config - OCI configuration file
//   - ~/.oci/sessions/<profile>/token - Session token
//   - ~/.oci/sessions/<profile>/oci_api_key.pem - Ephemeral private key
//
// The configuration uses security_token_file authentication method.
func GenerateOCIConfig(token *OracleCloudSessionToken, config *OracleCloudCredentialConfig, profileName string) string {
	if profileName == "" {
		profileName = "DEFAULT"
	}

	return fmt.Sprintf(`[%s]
tenancy=%s
region=%s
security_token_file=~/.oci/sessions/%s/token
key_file=~/.oci/sessions/%s/oci_api_key.pem
`, profileName, config.TenancyOCID, token.Region, profileName, profileName)
}

// GenerateOCICredentialHelper generates a bash script for the sandbox
// to set up OCI CLI authentication.
func GenerateOCICredentialHelper(backendURL, sessionToken, sandboxID, profileName string, expirationMinutes int) string {
	if profileName == "" {
		profileName = "DEFAULT"
	}
	if expirationMinutes < 5 || expirationMinutes > 60 {
		expirationMinutes = 60
	}

	return fmt.Sprintf(`#!/bin/bash
# OCI Credential Helper - Generated by Dynamiq
# This script fetches short-lived OCI session tokens from the backend
# Documentation: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm

set -e

PROFILE="%s"
SESSION_DIR="$HOME/.oci/sessions/$PROFILE"

# Fetch OCI session token from backend
response=$(curl -s -X POST "%s/api/cloud/oracle/credentials" \
  -H "Authorization: Bearer %s" \
  -H "Content-Type: application/json" \
  -d '{"sandboxId": "%s", "provider": "oracle", "expirationMinutes": %d}')

# Check for errors
error=$(echo "$response" | jq -r '.error // empty')
if [ -n "$error" ]; then
  echo "Error: $error" >&2
  exit 1
fi

# Extract token and key
session_token=$(echo "$response" | jq -r '.oracle.token')
private_key=$(echo "$response" | jq -r '.oracle.private_key')
region=$(echo "$response" | jq -r '.oracle.region')
expires_at=$(echo "$response" | jq -r '.oracle.expires_at')

if [ -z "$session_token" ] || [ "$session_token" = "null" ]; then
  echo "Error: Failed to get session token" >&2
  exit 1
fi

# Create session directory
mkdir -p "$SESSION_DIR"

# Write session files
echo "$session_token" > "$SESSION_DIR/token"
echo "$private_key" > "$SESSION_DIR/oci_api_key.pem"
chmod 600 "$SESSION_DIR/oci_api_key.pem"

# Get tenancy from backend response or use environment
tenancy=$(echo "$response" | jq -r '.oracle.tenancy // empty')

# Write OCI config
mkdir -p "$HOME/.oci"
cat > "$HOME/.oci/config" << EOF
[$PROFILE]
region=$region
security_token_file=$SESSION_DIR/token
key_file=$SESSION_DIR/oci_api_key.pem
EOF

chmod 600 "$HOME/.oci/config"

echo "OCI credentials configured (expires: $expires_at)"
echo "Using region: $region, profile: $PROFILE"
`, profileName, backendURL, sessionToken, sandboxID, expirationMinutes)
}

// ValidateCredentials tests if the OCI credentials are valid.
func (p *OracleCloudProvider) ValidateCredentials(ctx context.Context, config *OracleCloudCredentialConfig) error {
	// Try to parse the private key
	_, err := parsePrivateKey(config.PrivateKeyPEM)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	// For full validation, we'd make an API call to OCI
	// This is sufficient for basic validation
	return nil
}
