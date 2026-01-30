# Credential Security by Integration Type

> Last updated: January 2026

This document explains the security mechanism used for each integration type, what credentials are exposed to the E2B sandbox, and how we ensure security.

## Table of Contents

1. [Overview](#overview)
2. [Cloud CLI Integrations](#cloud-cli-integrations)
3. [Developer CLI Integrations](#developer-cli-integrations)
4. [MCP Tool Integrations](#mcp-tool-integrations)
5. [Direct API Integrations](#direct-api-integrations)
6. [Security Comparison Matrix](#security-comparison-matrix)

---

## Overview

### Core Principle

**User credentials (API keys, OAuth tokens, service account keys) NEVER enter the E2B sandbox.**

Instead, we use one of these patterns:
1. **Credential Helper**: Sandbox runs a script that calls our backend for short-lived credentials
2. **Token Exchange**: Backend exchanges long-lived credentials for short-lived tokens
3. **Proxy**: All requests go through our backend, which adds credentials

### Credential Flow Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         CREDENTIAL SECURITY FLOW                              │
│                                                                               │
│  ┌─────────────────┐                              ┌─────────────────┐        │
│  │   User Config   │                              │   E2B Sandbox   │        │
│  │  (Long-lived)   │                              │   (Ephemeral)   │        │
│  │                 │                              │                 │        │
│  │ • API Keys      │                              │ • Session Token │        │
│  │ • OAuth Tokens  │      ┌─────────────┐        │   (5 min TTL)   │        │
│  │ • Service Acct  │ ───▶ │   Backend   │ ───▶   │                 │        │
│  │ • Role ARNs     │      │  (Secure)   │        │ • Credential    │        │
│  └─────────────────┘      │             │        │   Helpers       │        │
│         │                 │ • Encrypted │        │                 │        │
│         │                 │   Storage   │        │ • Env Vars      │        │
│         │                 │             │        │   (tokens only) │        │
│         │                 │ • Token     │        │                 │        │
│         │                 │   Exchange  │        │ • NO long-lived │        │
│         │                 └──────┬──────┘        │   credentials   │        │
│         │                        │               └────────┬────────┘        │
│         │                        │                        │                  │
│         │                        ▼                        ▼                  │
│         │                 ┌─────────────┐        ┌─────────────────┐        │
│         │                 │   Cloud     │        │   Tool Call     │        │
│         └───────────────▶ │   Provider  │ ◀───── │   via Helper    │        │
│            (Backend       │   API       │        │                 │        │
│             requests)     └─────────────┘        └─────────────────┘        │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Cloud CLI Integrations

### AWS

| Aspect | Detail |
|--------|--------|
| **User Provides** | IAM Role ARN (e.g., `arn:aws:iam::123456789012:role/AgentRole`) |
| **Stored in Backend** | Role ARN, External ID (plaintext - not secrets) |
| **Mechanism** | AWS `credential_process` in `~/.aws/config` |
| **Sandbox Receives** | Credential helper script + session token (5 min TTL) |
| **Credential Lifetime** | Temporary credentials via STS (1 hour) |

**Security Flow:**
```
1. Sandbox AWS CLI reads ~/.aws/config
2. credential_process points to our helper script
3. Helper calls backend: POST /api/cloud/aws/credentials
4. Backend validates session token (5 min TTL)
5. Backend calls AWS STS AssumeRole
6. Backend returns temp credentials (1 hour TTL)
7. AWS CLI uses temp credentials
```

**Config File (in sandbox):**
```ini
[default]
credential_process = /usr/local/bin/aws-credential-helper
region = us-east-1
```

**Documentation:** https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html

---

### Google Cloud (GCP)

| Aspect | Detail |
|--------|--------|
| **User Provides** | Service Account JSON |
| **Stored in Backend** | SA JSON (AES-256-GCM encrypted) |
| **Mechanism** | Application Default Credentials with access token |
| **Sandbox Receives** | Credential helper script + access token (1 hour) |
| **Credential Lifetime** | Access token (1 hour, refreshable) |

**Security Flow:**
```
1. Sandbox gcloud reads Application Default Credentials
2. ADC configured with external_account pointing to helper
3. Helper calls backend for fresh access token
4. Backend uses SA JSON to generate access token
5. Token returned to sandbox (never the SA JSON)
```

**Documentation:** https://cloud.google.com/iam/docs/workload-identity-federation

---

### Microsoft Azure

| Aspect | Detail |
|--------|--------|
| **User Provides** | Service Principal (Tenant ID, Client ID, Client Secret) |
| **Stored in Backend** | Client Secret (AES-256-GCM encrypted) |
| **Mechanism** | Azure AD token exchange |
| **Sandbox Receives** | Access token via environment variable |
| **Credential Lifetime** | Access token (~1 hour) |

**Security Flow:**
```
1. Sandbox requests Azure credentials via helper
2. Backend authenticates to Azure AD (OAuth2 client credentials)
3. Backend returns access token (never client secret)
4. Sandbox uses token via AZURE_ACCESS_TOKEN env var
5. Azure CLI configured to use the token
```

**Environment Variables (in sandbox):**
```bash
AZURE_ACCESS_TOKEN=eyJ...    # Short-lived token
AZURE_SUBSCRIPTION_ID=...    # Non-secret
AZURE_TENANT_ID=...          # Non-secret
```

**Documentation:** https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow

---

### IBM Cloud

| Aspect | Detail |
|--------|--------|
| **User Provides** | IBM Cloud API Key |
| **Stored in Backend** | API Key (AES-256-GCM encrypted) |
| **Mechanism** | API Key → IAM access token exchange |
| **Sandbox Receives** | IAM access token (~60 min) |
| **Credential Lifetime** | Access token (~60 minutes) |

**Security Flow:**
```
1. Sandbox requests IBM Cloud credentials
2. Backend exchanges API key for IAM token:
   POST https://iam.cloud.ibm.com/identity/token
   grant_type=urn:ibm:params:oauth:grant-type:apikey
3. IAM returns JWT access token (~60 min validity)
4. Token returned to sandbox (never API key)
5. ibmcloud CLI uses token
```

**Token Format:** JWT (RS256 signed, ~60 minute validity)

**Documentation:** https://cloud.ibm.com/apidocs/iam-identity-token-api

---

### Oracle Cloud (OCI)

| Aspect | Detail |
|--------|--------|
| **User Provides** | API signing key (Tenancy OCID, User OCID, Fingerprint, Private Key) |
| **Stored in Backend** | Private Key PEM (AES-256-GCM encrypted) |
| **Mechanism** | Session token-based authentication |
| **Sandbox Receives** | Session token + ephemeral private key |
| **Credential Lifetime** | Session token (5-60 minutes, configurable) |

**Security Flow:**
```
1. Sandbox requests OCI credentials
2. Backend generates ephemeral RSA key pair
3. Backend signs session request with user's private key
4. OCI returns session token bound to ephemeral key
5. Sandbox receives: session token + ephemeral private key
6. User's original private key never leaves backend
```

**Config File (in sandbox):**
```ini
[DEFAULT]
tenancy=ocid1.tenancy.oc1..xxx
region=us-ashburn-1
security_token_file=~/.oci/sessions/DEFAULT/token
key_file=~/.oci/sessions/DEFAULT/oci_api_key.pem  # Ephemeral key
```

**Documentation:** https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm

---

### Kubernetes

| Aspect | Detail |
|--------|--------|
| **User Provides** | Cluster config (API server, CA cert, auth method) |
| **Stored in Backend** | Service account token or cloud provider config |
| **Mechanism** | kubeconfig with exec credential plugin |
| **Sandbox Receives** | kubeconfig with exec plugin or short-lived token |
| **Credential Lifetime** | Depends on auth method (1 hour typical) |

**Auth Methods Supported:**

| Method | Flow |
|--------|------|
| `token` | Service account token (should be short-lived) |
| `aws-eks` | AWS EKS: Uses `aws eks get-token` (leverages AWS credential helper) |
| `gcp-gke` | GCP GKE: Uses `gke-gcloud-auth-plugin` (leverages GCP credential helper) |
| `azure-aks` | Azure AKS: Uses `kubelogin` with service principal |
| `exec` | Custom exec plugin calling our backend |

**Kubeconfig (exec plugin example):**
```yaml
users:
- name: default
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: /usr/local/bin/k8s-credential-helper
      args: ["--sandbox-id", "sandbox-123"]
```

**Documentation:** https://kubernetes.io/docs/reference/access-authn-authz/authentication/

---

## Developer CLI Integrations

### GitHub (gh)

| Aspect | Detail |
|--------|--------|
| **User Provides** | OAuth2 authorization (via browser) |
| **Stored in Backend** | OAuth access token (AES-256-GCM encrypted) |
| **Mechanism** | Token injected via environment variable |
| **Sandbox Receives** | `GH_TOKEN` environment variable |
| **Credential Lifetime** | OAuth token (until revoked, but can be scoped) |

**Security Consideration:** GitHub OAuth tokens are long-lived. We recommend:
- Using fine-grained personal access tokens with minimal scopes
- Setting token expiration when creating OAuth apps

**Environment (in sandbox):**
```bash
GH_TOKEN=gho_xxxx    # OAuth token
```

**Documentation:** https://cli.github.com/manual/gh_auth_login

---

### Stripe CLI

| Aspect | Detail |
|--------|--------|
| **User Provides** | Stripe API key (secret key) |
| **Stored in Backend** | API key (AES-256-GCM encrypted) |
| **Mechanism** | Token injected via environment variable |
| **Sandbox Receives** | `STRIPE_API_KEY` environment variable |
| **Credential Lifetime** | API key (until rotated) |

**Important:** Stripe test mode keys (`sk_test_`) vs live keys (`sk_live_`)
- Recommend using restricted keys with minimal permissions

**Documentation:** https://stripe.com/docs/cli

---

### Vercel CLI

| Aspect | Detail |
|--------|--------|
| **User Provides** | Vercel token |
| **Stored in Backend** | Token (AES-256-GCM encrypted) |
| **Mechanism** | Token injected via environment variable |
| **Sandbox Receives** | `VERCEL_TOKEN` environment variable |

**Documentation:** https://vercel.com/docs/cli

---

## MCP Tool Integrations

MCP (Model Context Protocol) integrations use a **proxy architecture** - the sandbox NEVER receives credentials.

### Gmail, Slack, Notion, etc.

| Aspect | Detail |
|--------|--------|
| **User Provides** | OAuth2 authorization via MCP provider |
| **Stored in Backend** | Provider handles token storage |
| **Mechanism** | All requests proxied through backend |
| **Sandbox Receives** | Session token for MCP proxy only |
| **Credential Lifetime** | N/A - credentials never in sandbox |

**Security Flow:**
```
1. User authorizes app via OAuth (handled by MCP provider)
2. MCP provider stores OAuth tokens securely
3. Agent calls list_app_tools / call_app_tool
4. Backend receives tool call request
5. Backend calls MCP provider API with stored credentials
6. Result returned to agent
7. OAuth tokens NEVER enter sandbox
```

**Why This is Secure:**
- OAuth tokens stored by trusted third party (MCP provider)
- Sandbox only has session token for our backend
- All external API calls happen server-side
- No credential exposure even if sandbox is compromised

---

## Direct API Integrations

### Datadog, New Relic, PagerDuty

| Aspect | Detail |
|--------|--------|
| **User Provides** | API key |
| **Stored in Backend** | API key (AES-256-GCM encrypted) |
| **Mechanism** | API key injected via environment variable |
| **Sandbox Receives** | API key in environment variable |
| **Credential Lifetime** | API key (until rotated) |

**Security Consideration:** These inject the actual API key. Mitigations:
- Use read-only API keys when possible
- Use scoped/restricted API keys
- Monitor API key usage in provider dashboards
- Rotate keys regularly

**Future Improvement:** Implement a proxy layer for API tools to avoid key injection.

---

## Security Comparison Matrix

| Integration Type | User Credential | Sandbox Receives | Risk Level | Notes |
|-----------------|----------------|------------------|------------|-------|
| **AWS** | IAM Role ARN | Temp credentials (1hr) | ✅ Low | credential_process pattern |
| **GCP** | Service Account JSON | Access token (1hr) | ✅ Low | ADC with token refresh |
| **Azure** | Service Principal | Access token (1hr) | ✅ Low | OAuth2 token exchange |
| **IBM Cloud** | API Key | IAM token (60min) | ✅ Low | Token exchange |
| **Oracle Cloud** | API Private Key | Session + ephemeral key | ✅ Low | Ephemeral key pair |
| **Kubernetes** | Various | Exec plugin/token | ✅ Low | Cloud-specific or exec |
| **GitHub CLI** | OAuth Token | OAuth token | ⚠️ Medium | Long-lived token exposed |
| **Stripe CLI** | API Key | API key | ⚠️ Medium | API key exposed |
| **MCP Tools** | OAuth via provider | Session token only | ✅ Low | Proxy architecture |
| **API Tools** | API Key | API key | ⚠️ Medium | Key exposed |

### Risk Mitigation for Medium-Risk Integrations

1. **Use scoped credentials** - Minimal permissions
2. **Short expiration** - Where supported
3. **Monitor usage** - Track API calls
4. **Rotate regularly** - After sandbox sessions
5. **Future: Proxy all API calls** - Remove direct key exposure

---

## Best Practices Summary

### For Cloud Providers (AWS, GCP, Azure, IBM, OCI)
1. Always use credential helpers or token exchange
2. Never inject long-lived credentials
3. Set shortest practical TTL
4. Use IAM roles/service accounts with minimal permissions

### For CLI Tools (GitHub, Stripe, etc.)
1. Use fine-grained tokens with minimal scopes
2. Set token expiration when possible
3. Consider proxying through backend (future improvement)

### For MCP Tools
1. Leverage the proxy architecture - most secure
2. All credentials handled by MCP provider
3. No credential exposure to sandbox

### For API Tools
1. Use read-only keys when possible
2. Use scoped/restricted keys
3. Plan migration to proxy architecture

---

## Documentation Links

| Provider | Documentation |
|----------|---------------|
| AWS credential_process | https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html |
| GCP Workload Identity | https://cloud.google.com/iam/docs/workload-identity-federation |
| Azure Service Principal | https://learn.microsoft.com/en-us/cli/azure/authenticate-azure-cli-service-principal |
| IBM Cloud IAM | https://cloud.ibm.com/apidocs/iam-identity-token-api |
| Oracle OCI Session | https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/clitoken.htm |
| Kubernetes Auth | https://kubernetes.io/docs/reference/access-authn-authz/authentication/ |
| GitHub CLI Auth | https://cli.github.com/manual/gh_auth_login |
| MCP Specification | https://spec.modelcontextprotocol.io |
