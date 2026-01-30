# Security Documentation

> Last updated: January 2026

## Overview

This document outlines the security architecture and considerations for Dynamiq. Security is a top priority - user credentials must never be exposed to sandboxes or logged.

## Table of Contents

1. [Threat Model](#threat-model)
2. [Credential Security](#credential-security)
3. [Token Security](#token-security)
4. [Sandbox Security](#sandbox-security)
5. [API Security](#api-security)
6. [Security Checklist](#security-checklist)
7. [Incident Response](#incident-response)

---

## Threat Model

### Assets to Protect

1. **User Credentials**
   - OAuth tokens (GitHub, Google, etc.)
   - API keys (Datadog, Stripe, etc.)
   - Cloud credentials (AWS roles, GCP service accounts)

2. **User Data**
   - Conversation history
   - Tool execution results
   - Connected app information

3. **System Secrets**
   - JWT signing key
   - MCP provider credentials (Pipedream, Composio)
   - E2B API key

### Threat Vectors

| Threat | Mitigation |
|--------|------------|
| Credential theft from sandbox | Credentials never enter sandbox; only short-lived tokens |
| Token replay attacks | Tokens include nonce and short TTL (5 min) |
| Man-in-the-middle | All external APIs use HTTPS |
| Injection attacks | Input validation, parameterized queries |
| Unauthorized access | JWT authentication, scope-based authorization |

---

## Credential Security

### Storage Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    CREDENTIAL FLOW                               │
│                                                                  │
│  User Input                Backend Storage              Sandbox  │
│  ──────────               ───────────────              ───────  │
│                                                                  │
│  OAuth Token  ──────▶  AES-256-GCM    ──────▶  Never exposed    │
│                        Encrypted                                 │
│                                                                  │
│  API Key      ──────▶  AES-256-GCM    ──────▶  Env var (masked) │
│                        Encrypted                                 │
│                                                                  │
│  AWS Role ARN ──────▶  Plaintext      ──────▶  STS temp creds   │
│               (no secret)              (via credential_process) │
│                                                                  │
│  GCP SA JSON  ──────▶  AES-256-GCM    ──────▶  Access token     │
│                        Encrypted       (via workload identity)  │
└─────────────────────────────────────────────────────────────────┘
```

### Encryption Details

```go
// AES-256-GCM encryption with random nonce
// Key derived from JWT_SECRET (32 bytes)

func (s *CredentialStore) encrypt(plaintext string) (string, error) {
    block, _ := aes.NewCipher(s.encryptionKey)  // 256-bit key
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())      // Random 12-byte nonce
    io.ReadFull(rand.Reader, nonce)
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

### What Gets Encrypted

| Data Type | Storage | Encrypted |
|-----------|---------|-----------|
| OAuth access_token | Memory | ✅ Yes |
| OAuth refresh_token | Memory | ✅ Yes |
| API keys | Memory | ✅ Yes |
| GCP Service Account JSON | Memory | ✅ Yes |
| AWS Role ARN | Memory | ❌ No (not a secret) |
| AWS External ID | Memory | ❌ No (not a secret) |

---

## Token Security

### Session Tokens (JWT)

Used for sandbox-to-backend authentication:

```go
type TokenClaims struct {
    UserID         string   `json:"user_id"`
    ConversationID string   `json:"conversation_id"`
    SandboxID      string   `json:"sandbox_id"`
    Scopes         []Scope  `json:"scopes"`
    Nonce          string   `json:"nonce"`  // Prevents replay
    jwt.RegisteredClaims
}
```

**Token Properties:**
- TTL: 5 minutes
- Algorithm: HS256
- Signed with: JWT_SECRET

**Scopes:**
```go
const (
    ScopeListTools  Scope = "mcp:list_tools"
    ScopeCallTools  Scope = "mcp:call_tools"
)
```

### Token Validation

```go
func (m *TokenManager) ValidateSessionTokenWithScope(tokenString string, requiredScope Scope) (*TokenClaims, error) {
    // 1. Parse and validate signature
    // 2. Check expiration
    // 3. Verify required scope
    // 4. Return claims
}
```

---

## Sandbox Security

### Isolation Principles

1. **Network Isolation**: Sandboxes run in isolated E2B environments
2. **Credential Isolation**: No long-lived credentials in sandbox
3. **Time Limits**: Sandbox timeout (default: 5 minutes)
4. **Resource Limits**: E2B enforces CPU/memory limits

### Credential Flow (AWS Example)

```bash
# Inside sandbox: ~/.aws/config
[default]
credential_process = /usr/local/bin/aws-credential-helper

# Credential helper script:
#!/bin/bash
# 1. Has short-lived session token (5 min)
# 2. Calls backend: POST /api/cloud/aws/credentials
# 3. Backend validates token
# 4. Backend calls AWS STS AssumeRole
# 5. Returns temp credentials (1 hour)
```

### What Sandbox Can Access

| Resource | Access Level |
|----------|-------------|
| User's long-lived tokens | ❌ Never |
| Backend's API keys | ❌ Never |
| Short-lived session token | ✅ Yes (5 min TTL) |
| Temporary cloud credentials | ✅ Yes (via helper) |
| MCP tool calls | ✅ Yes (via backend proxy) |

---

## API Security

### Authentication

All API endpoints (except `/health`) require authentication:

```go
// From request header
userID := r.Header.Get("X-User-ID")

// Or from session token
claims, err := tokenManager.ValidateSessionToken(token)
```

### CORS Configuration

```go
cors.Handler(cors.Options{
    AllowedOrigins:   []string{"http://localhost:*"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge:           300,
})
```

### Input Validation

All API inputs are validated:
- JSON parsing with strict types
- Required field checks
- Length limits where applicable

---

## Security Checklist

### Implemented ✅

- [x] Credential encryption (AES-256-GCM)
- [x] Short-lived session tokens (5 min TTL)
- [x] Token scoping (least privilege)
- [x] Nonce in tokens (replay prevention)
- [x] Credentials never enter sandbox
- [x] HTTPS for all external APIs
- [x] CORS restrictions

### TODO 🔲

- [ ] Rate limiting on API endpoints
- [ ] Request size limits
- [ ] Audit logging for credential access
- [ ] Token revocation mechanism
- [ ] Database encryption at rest (currently in-memory)
- [ ] Secret rotation procedures
- [ ] Penetration testing
- [ ] Security headers (CSP, HSTS, etc.)

---

## Incident Response

### If Credentials Are Compromised

1. **Immediate Actions:**
   - Rotate JWT_SECRET
   - Revoke OAuth tokens via provider dashboards
   - Rotate API keys

2. **Investigation:**
   - Review logs for unauthorized access
   - Identify scope of compromise
   - Document timeline

3. **Notification:**
   - Notify affected users
   - Report to relevant parties

### If Session Token Is Leaked

1. Token expires in 5 minutes - limited window
2. Token only valid for specific sandbox ID
3. Scope limits what can be done

### Contact

For security issues, contact: security@yourcompany.com

---

## References

- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [AWS Security Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [GCP Security Best Practices](https://cloud.google.com/security/best-practices)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
