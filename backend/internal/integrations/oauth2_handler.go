package integrations

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

type OAuth2HandlerConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

type OAuth2HandlerImpl struct {
	cfg        OAuth2HandlerConfig
	httpClient *http.Client
}

func NewOAuth2Handler(cfg OAuth2HandlerConfig) OAuth2Handler {
	return &OAuth2HandlerImpl{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (h *OAuth2HandlerImpl) GetAuthURL(state string) string {
	authURL, _ := url.Parse(h.cfg.AuthURL)
	params := authURL.Query()
	params.Set("response_type", "code")
	params.Set("client_id", h.cfg.ClientID)
	params.Set("redirect_uri", h.cfg.RedirectURL)
	if len(h.cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(h.cfg.Scopes, " "))
	}
	if state != "" {
		params.Set("state", state)
	}
	authURL.RawQuery = params.Encode()
	return authURL.String()
}

func (h *OAuth2HandlerImpl) ExchangeCode(ctx context.Context, code string) (*OAuth2Token, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", h.cfg.ClientID)
	data.Set("client_secret", h.cfg.ClientSecret)
	data.Set("redirect_uri", h.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", h.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth exchange failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	return parseOAuthTokenResponse(body)
}

func (h *OAuth2HandlerImpl) RefreshToken(ctx context.Context, refreshToken string) (*OAuth2Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", h.cfg.ClientID)
	data.Set("client_secret", h.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", h.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth refresh failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	return parseOAuthTokenResponse(body)
}

func parseOAuthTokenResponse(body []byte) (*OAuth2Token, error) {
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		ExpiresAt    string `json:"expires_at"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	token := &OAuth2Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
	}

	if tokenResp.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else if tokenResp.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, tokenResp.ExpiresAt); err == nil {
			token.ExpiresAt = parsed
		}
	}

	if tokenResp.Scope != "" {
		token.Scopes = strings.Fields(tokenResp.Scope)
	}

	if token.AccessToken == "" {
		return nil, fmt.Errorf("missing access_token in response")
	}

	return token, nil
}
