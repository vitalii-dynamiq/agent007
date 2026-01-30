package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Scope represents allowed operations for a token
type Scope string

const (
	ScopeListTools Scope = "mcp:list_tools"
	ScopeCallTools Scope = "mcp:call_tools"
	ScopeListApps  Scope = "mcp:list_apps"
	ScopeAll       Scope = "mcp:*"
)

// TokenClaims represents the claims in a session token
type TokenClaims struct {
	UserID         string  `json:"user_id"`
	ConversationID string  `json:"conversation_id"`
	SandboxID      string  `json:"sandbox_id"`
	Scopes         []Scope `json:"scopes"` // Allowed operations
	Nonce          string  `json:"nonce"`  // Unique per-token to prevent replay
	jwt.RegisteredClaims
}

// HasScope checks if the token has a specific scope
func (tc *TokenClaims) HasScope(scope Scope) bool {
	for _, s := range tc.Scopes {
		if s == ScopeAll || s == scope {
			return true
		}
	}
	return false
}

// TokenManager handles JWT token operations
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenManager creates a new token manager
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// GenerateSessionToken generates a short-lived session token for sandbox use
// The token includes scopes that limit what operations can be performed
func (tm *TokenManager) GenerateSessionToken(userID, conversationID, sandboxID string) (string, error) {
	return tm.GenerateSessionTokenWithScopes(userID, conversationID, sandboxID, []Scope{ScopeAll})
}

// GenerateSessionTokenWithScopes generates a token with specific scopes
func (tm *TokenManager) GenerateSessionTokenWithScopes(userID, conversationID, sandboxID string, scopes []Scope) (string, error) {
	now := time.Now()

	// Generate a unique nonce for this token
	nonce, err := generateNonce()
	if err != nil {
		return "", err
	}

	claims := TokenClaims{
		UserID:         userID,
		ConversationID: conversationID,
		SandboxID:      sandboxID,
		Scopes:         scopes,
		Nonce:          nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "dynamiq",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

// ValidateSessionToken validates a session token and returns claims
func (tm *TokenManager) ValidateSessionToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateSessionTokenWithScope validates token and checks for required scope
func (tm *TokenManager) ValidateSessionTokenWithScope(tokenString string, requiredScope Scope) (*TokenClaims, error) {
	claims, err := tm.ValidateSessionToken(tokenString)
	if err != nil {
		return nil, err
	}

	if !claims.HasScope(requiredScope) {
		return nil, errors.New("insufficient permissions")
	}

	return claims, nil
}

// generateNonce creates a cryptographically secure random nonce
func generateNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
