package github

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	apiBaseURL       = "https://api.github.com"
	apiVersionHeader = "2022-11-28"
)

type AppClient struct {
	appID      string
	appSlug    string
	privateKey *rsa.PrivateKey
	httpClient *http.Client
}

type Installation struct {
	ID      int64
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
	RepositorySelection string `json:"repository_selection"`
}

type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewAppClient(appID, appSlug, privateKeyPEM string) (*AppClient, error) {
	if appID == "" || appSlug == "" || privateKeyPEM == "" {
		return nil, fmt.Errorf("github app configuration is missing")
	}

	key, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	return &AppClient{
		appID:      appID,
		appSlug:    appSlug,
		privateKey: key,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *AppClient) InstallURL(state string) string {
	if state != "" {
		return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s", c.appSlug, state)
	}
	return fmt.Sprintf("https://github.com/apps/%s/installations/new", c.appSlug)
}

func (c *AppClient) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
	jwtToken, err := c.createJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/app/installations/%d", apiBaseURL, installationID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, jwtToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github installation lookup failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var installation Installation
	if err := json.Unmarshal(body, &installation); err != nil {
		return nil, err
	}
	installation.ID = installationID
	return &installation, nil
}

func (c *AppClient) CreateInstallationToken(ctx context.Context, installationID int64) (*InstallationToken, error) {
	jwtToken, err := c.createJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/app/installations/%d/access_tokens", apiBaseURL, installationID),
		bytes.NewReader([]byte("{}")),
	)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github token creation failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var token InstallationToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	if token.Token == "" {
		return nil, fmt.Errorf("github token missing from response")
	}
	return &token, nil
}

func (c *AppClient) createJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.privateKey)
}

func (c *AppClient) applyHeaders(req *http.Request, jwtToken string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-GitHub-Api-Version", apiVersionHeader)
}

func parsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	payload := strings.TrimSpace(privateKeyPEM)
	if !strings.HasPrefix(payload, "-----BEGIN") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode github private key: %w", err)
		}
		payload = string(decoded)
	}

	block, _ := pem.Decode([]byte(payload))
	if block == nil {
		return nil, fmt.Errorf("failed to parse github private key PEM")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse github private key: %w", err)
	}

	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github private key is not RSA")
	}
	return rsaKey, nil
}
