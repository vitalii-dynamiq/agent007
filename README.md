# Agent007: AI Agent Platform with Tool Integrations

> A production-ready AI agent platform that enables LLMs to interact with external services (GitHub, Slack, AWS, HubSpot, etc.) through secure sandbox environments and managed OAuth.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-19+-61DAFB?style=flat&logo=react)](https://reactjs.org)
[![Python](https://img.shields.io/badge/Python-3.10+-3776AB?style=flat&logo=python)](https://python.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Integrations](#integrations)
- [MCP Providers](#mcp-providers)
- [Cloud Providers](#cloud-providers)
- [Security Model](#security-model)
- [Development](#development)
- [API Reference](#api-reference)

## Overview

Agent007 is an AI agent platform that allows Large Language Models to securely interact with external tools and services. The platform consists of three main components:

1. **Go Backend** - API server, OAuth handling, credential management, MCP proxy
2. **React Frontend** - Chat UI, integration management, OAuth flows
3. **Python Agent** - LLM orchestration, E2B sandbox execution

### Integration Methods

The platform supports multiple methods for connecting to external services:

| Method | Description | Examples |
|--------|-------------|----------|
| **MCP (Pipedream)** | 2000+ apps via Pipedream Connect | Gmail, Slack, Notion, Jira, Linear |
| **MCP (Composio)** | 300+ apps with managed OAuth credentials | HubSpot, ClickUp, Confluence |
| **Direct MCP** | Official MCP servers | Sentry |
| **CLI Tools** | Official command-line tools | GitHub CLI, Stripe CLI, Vercel |
| **Cloud CLIs** | Cloud provider CLIs with credential injection | AWS, GCP, Azure, IBM, Oracle |
| **GitHub App** | GitHub App installation for repo access | GitHub (enhanced) |

### Key Principles

1. **Security First**: User credentials never enter sandboxes; only short-lived tokens
2. **Managed OAuth**: Use Composio/Pipedream managed credentials - no need to create your own OAuth apps
3. **Modularity**: Easy to add new integrations without changing core code
4. **Extensibility**: Support for multiple provider types

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                 FRONTEND                                     │
│                     React 19 + Radix UI + TailwindCSS                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │  Chat View  │  │ Integrations│  │  OAuth Flow │  │   Settings  │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │ REST API + SSE
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GO BACKEND                                      │
│                          Go 1.24 + Chi Router                               │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                          API Layer (/api)                             │  │
│  │  • Conversations  • Integrations  • Cloud Credentials  • MCP Proxy   │  │
│  │  • GitHub App     • OAuth Callbacks                                   │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │    MCP     │  │   Cloud    │  │  GitHub    │  │Integration │           │
│  │  Registry  │  │  Manager   │  │    App     │  │  Registry  │           │
│  │            │  │            │  │            │  │            │           │
│  │ • Pipedream│  │ • AWS STS  │  │ • JWT Auth │  │ • Catalog  │           │
│  │ • Composio │  │ • GCP IAM  │  │ • Install  │  │ • 39 Apps  │           │
│  │ • Direct   │  │ • Azure    │  │   Tokens   │  │ • State    │           │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘           │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            PYTHON AGENT                                      │
│                         OpenAI + E2B Sandbox                                │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │  • LLM Tool Loop (OpenAI function calling)                           │  │
│  │  • E2B Sandbox Execution (isolated Linux environment)                │  │
│  │  • Credential Helpers (AWS, GCP, GitHub token injection)             │  │
│  │  • MCP CLI (calls backend proxy for tool execution)                  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        ▼                         ▼                         ▼
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│   Pipedream  │         │   Composio   │         │ Cloud APIs   │
│              │         │              │         │              │
│ • Gmail      │         │ • HubSpot    │         │ • AWS STS    │
│ • Slack      │         │ • ClickUp    │         │ • GCP IAM    │
│ • Notion     │         │ • Confluence │         │ • Azure AD   │
│ • Jira       │         │              │         │              │
│ • Linear     │         │              │         │              │
│ • Calendar   │         │              │         │              │
└──────────────┘         └──────────────┘         └──────────────┘
```

## Features

### 44+ Pre-configured Integrations

| Category | Integrations |
|----------|--------------|
| **Developer Tools** | GitHub, Stripe, Vercel, Supabase, Neon, Cloudflare, Sentry, Linear, Jira, Confluence, ClickUp |
| **Cloud** | AWS, Google Cloud, Microsoft Azure, IBM Cloud, Oracle Cloud, Kubernetes |
| **Databases** | PostgreSQL, MySQL, BigQuery, Snowflake, Databricks, SQL Server, Vertica |
| **Productivity** | Gmail, Google Calendar, Google Drive, OneDrive, SharePoint, Notion, Asana, Monday.com, Trello, Airtable, HubSpot |
| **Communication** | Slack, Discord, Microsoft Teams, Outlook |
| **Monitoring** | Datadog, New Relic, PagerDuty |

### Provider Types

| Type | Description | Auth Method | Examples |
|------|-------------|-------------|----------|
| `cli` | Official CLI tools | OAuth2 / API Key | GitHub (gh), Stripe, Vercel |
| `cloud_cli` | Cloud provider CLIs | IAM Role / Service Account | AWS, GCP, Azure |
| `mcp` (Pipedream) | Pipedream Connect (2000+ apps) | Managed OAuth | Gmail, Slack, Notion, Jira |
| `mcp` (Composio) | Composio (300+ apps) | Managed OAuth | HubSpot, ClickUp, Confluence |
| `direct_mcp` | Official MCP servers | OAuth2 | Sentry |
| `github_app` | GitHub App installation | App JWT + Installation Token | GitHub (enhanced) |

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+
- Python 3.10+
- Docker & Docker Compose (for PostgreSQL analytics database)
- E2B account (https://e2b.dev)

### Installation

```bash
# Clone the repository
git clone https://github.com/vitalii-dynamiq/agent007.git
cd agent007

# Backend setup
cd backend
cp ../.env.example .env
# Edit .env with your API keys (see Configuration section)
go mod download

# Frontend setup
cd ../frontend
npm install

# Agent setup
cd ../agent
pip install -r requirements.txt
```

### Running Locally with ngrok

**Why ngrok?** The platform needs a public URL for:
1. **E2B Sandbox Callbacks** - The sandbox runs in the cloud and needs to call back to your backend for MCP tools, credential helpers, etc.
2. **OAuth Redirects** - Composio/Pipedream redirect back to your backend after OAuth authorization completes

#### Step 1: Install ngrok

```bash
# macOS
brew install ngrok

# Linux
curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
sudo apt update && sudo apt install ngrok

# Or download from https://ngrok.com/download
```

#### Step 2: Start ngrok

```bash
# Expose your backend (port 8080) to the internet
ngrok http 8080
```

You'll see output like:
```
Forwarding    https://abc123.ngrok.io -> http://localhost:8080
```

#### Step 3: Configure BACKEND_URL

Copy the ngrok HTTPS URL and set it in your `.env`:

```bash
# .env
BACKEND_URL=https://abc123.ngrok.io    # Your ngrok URL (changes each restart unless you have a paid plan)
FRONTEND_URL=http://localhost:5173
```

**Tip**: With a paid ngrok plan, you can use a stable subdomain:
```bash
ngrok http 8080 --subdomain=myagent
# Then use: BACKEND_URL=https://myagent.ngrok.io
```

#### Step 4: Start the services

```bash
# Terminal 1: ngrok (keep running)
ngrok http 8080

# Terminal 2: Start database (Docker)
make db

# Terminal 3: Backend
make backend

# Terminal 4: Agent
make agent

# Terminal 5: Frontend
make frontend
```

Or use the quick start:
```bash
# In separate terminals:
ngrok http 8080           # Terminal 1 - ngrok tunnel
make dev                  # Terminal 2 - shows instructions after starting DB
make backend              # Terminal 3 - Go backend on :8080
make agent                # Terminal 4 - Python agent on :8082
make frontend             # Terminal 5 - React frontend on :5173
```

Visit http://localhost:5173

#### Database (PostgreSQL Analytics)

The platform includes a PostgreSQL sidecar for analytics capabilities:

```bash
# Start the database
make db

# Connect via psql
make db-psql

# View logs
make db-logs

# Reset database (destroys data)
make db-reset
```

The database is pre-populated with sample analytics data (sales, marketing, support schemas) for AI agent queries.

#### Troubleshooting ngrok

| Issue | Solution |
|-------|----------|
| OAuth callback fails | Make sure `BACKEND_URL` matches your current ngrok URL |
| "Waiting for authorization" spinner | Check ngrok is running and URL is correct in `.env` |
| E2B sandbox can't reach backend | Verify ngrok tunnel is active, check backend logs |
| ngrok URL changed | Update `BACKEND_URL` in `.env` and restart backend |

### Running without ngrok (Limited)

You can run without ngrok for basic testing, but:
- OAuth flows (Composio, Pipedream) won't work - callbacks can't reach localhost
- E2B sandbox MCP tools won't work - sandbox can't call localhost
- GitHub App webhooks won't work

```bash
# Backend only (for API testing)
cd backend && go run cmd/server/main.go

# Frontend
cd frontend && npm run dev
```

## Configuration

### Required Environment Variables

```bash
# .env (in project root or backend/)

# LLM Configuration
LLM_API_KEY=sk-...              # OpenAI API key
LLM_MODEL=gpt-4-turbo           # Model to use

# E2B Sandbox
E2B_API_KEY=e2b_...             # E2B API key (https://e2b.dev)

# Security
JWT_SECRET=your-32-byte-secret  # For session tokens (generate with: openssl rand -base64 32)
```

### MCP Provider Configuration

You need at least one MCP provider configured:

```bash
# Pipedream Connect (2000+ apps)
# Setup: https://pipedream.com/docs/connect
PIPEDREAM_CLIENT_ID=oa2-...
PIPEDREAM_CLIENT_SECRET=...
PIPEDREAM_PROJECT_ID=proj_...
PIPEDREAM_ENVIRONMENT=development

# Composio (300+ apps with managed OAuth)
# Setup: https://app.composio.dev
COMPOSIO_API_KEY=ak_...
COMPOSIO_PROJECT_ID=...         # Optional
```

### Cloud Provider Configuration (Optional)

```bash
# AWS - for assuming roles on behalf of users
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1

# GCP - handled via service account JSON uploaded by users
# Azure - handled via service principal credentials uploaded by users
```

### GitHub App Configuration (Optional)

For enhanced GitHub integration with repository access:

```bash
GITHUB_APP_ID=123456
GITHUB_APP_SLUG=your-app-name
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."
```

## Integrations

### Integration by MCP Provider

**Pipedream** (default for most integrations):
- Gmail, Google Calendar, Google Drive
- Slack, Discord
- Notion, Asana, Monday.com, Trello
- Linear, Jira
- Airtable

**Composio** (managed OAuth credentials):
- HubSpot
- ClickUp  
- Confluence

**Direct MCP** (official servers):
- Sentry

### Adding a New Integration

1. **Add to catalog** (`backend/internal/integrations/catalog.go`):

```go
"myservice": {
    ID:           "myservice",
    Name:         "My Service",
    Description:  "Description here",
    Category:     CategoryDeveloperTools,
    ProviderType: ProviderMCP,
    AuthType:     AuthOAuth2,
    MCPProvider:  "pipedream",  // or "composio"
    MCPAppSlug:   "myservice",  // app slug in the MCP provider
    AgentInstructions: `Instructions for the agent...`,
    Capabilities: []string{"feature1", "feature2"},
    Enabled:      true,
},
```

2. **Test the integration**:
```bash
# Get connect link
curl -X POST "http://localhost:8080/api/auth/connect-token?provider=pipedream&app=myservice" \
  -H "X-User-ID: test_user"
```

## MCP Providers

### Pipedream

Pipedream Connect provides access to 2000+ apps with managed OAuth.

| Feature | Details |
|---------|---------|
| Apps | 2000+ |
| Auth | Managed OAuth (Pipedream handles client credentials) |
| Documentation | https://pipedream.com/docs/connect |
| MCP Docs | https://pipedream.com/docs/connect/mcp |

**Supported Apps**: Gmail, Google Calendar, Slack, Notion, Linear, Jira, Discord, Twitter, and many more.

### Composio

Composio provides 300+ integrations with managed OAuth credentials - no need to create your own OAuth apps.

| Feature | Details |
|---------|---------|
| Apps | 300+ |
| Auth | Managed OAuth (Composio provides OAuth credentials) |
| Documentation | https://docs.composio.dev |

**Supported Apps**: HubSpot, ClickUp, Confluence, and many more.

**Note**: Some apps (like Jira) may not have managed auth available in Composio and will use Pipedream instead.

### Direct MCP

Some services provide official MCP servers:

| Service | MCP Server | Documentation |
|---------|------------|---------------|
| Sentry | https://mcp.sentry.dev/mcp | https://docs.sentry.io/product/sentry-mcp |

## Cloud Providers

### AWS

Uses STS AssumeRole to provide temporary credentials to sandboxes.

**User Setup**:
1. Create an IAM Role in their AWS account
2. Trust the Agent007 AWS account to assume the role
3. Provide the Role ARN to Agent007

**Agent Flow**:
1. Sandbox requests credentials via credential helper
2. Backend validates session token
3. Backend calls STS AssumeRole with user's Role ARN
4. Returns temporary credentials (1-hour TTL)

```bash
# In sandbox ~/.aws/config:
[default]
credential_process = /usr/local/bin/aws-credential-helper
```

### Google Cloud (GCP)

Uses Service Account credentials with Workload Identity Federation pattern.

**User Setup**:
1. Create a Service Account in their GCP project
2. Grant necessary permissions
3. Upload Service Account JSON to Agent007

**Agent Flow**:
1. Sandbox requests access token
2. Backend validates session, retrieves stored credentials
3. Backend generates access token from Service Account
4. Returns short-lived access token

### Azure

Uses Service Principal credentials for Azure resource access.

**User Setup**:
1. Create a Service Principal in Azure AD
2. Grant necessary RBAC permissions
3. Provide Tenant ID, Client ID, Client Secret

### IBM Cloud / Oracle Cloud

Basic support for CLI authentication using API keys.

### Kubernetes

Supports kubeconfig-based authentication for cluster management.

## Security Model

### Credential Flow

```
User Credentials (stored encrypted in backend)
         │
         ▼
Backend generates session token (5 min TTL)
         │
         ▼
Token injected into E2B sandbox as env var
         │
         ▼
Sandbox credential helper calls backend with token
         │
         ▼
Backend validates token, calls cloud provider
         │
         ▼
Returns short-lived credentials to sandbox
```

### Security Measures

1. **Credential Isolation**: User secrets never enter sandboxes
2. **Encrypted Storage**: AES-256-GCM for all stored credentials
3. **Short-lived Tokens**: 5-minute TTL for sandbox session tokens
4. **Managed OAuth**: Use Composio/Pipedream credentials instead of storing user OAuth tokens
5. **Scoped Permissions**: Tokens limited to specific operations

## Development

### Project Structure

```
agent007/
├── backend/
│   ├── cmd/server/          # Application entry point
│   └── internal/
│       ├── api/             # HTTP handlers and routing
│       ├── auth/            # JWT token management
│       ├── cloud/           # AWS/GCP/Azure credential management
│       ├── config/          # Configuration loading
│       ├── github/          # GitHub App integration
│       ├── integrations/    # Integration registry and catalog
│       ├── llm/             # LLM client abstraction
│       ├── mcp/             # MCP providers (Pipedream, Composio, Direct)
│       └── store/           # In-memory data store
├── frontend/
│   └── src/
│       ├── components/      # React components
│       │   ├── chat/        # Chat UI
│       │   ├── integrations/# OAuth connection UI
│       │   └── ui/          # Shared primitives (Radix)
│       └── lib/             # API client and utilities
├── agent/
│   ├── main.py              # Python agent with E2B sandbox
│   ├── server.py            # FastAPI server for agent
│   └── requirements.txt     # Python dependencies
├── docs/                    # Additional documentation
├── Makefile                 # Build and run commands
└── .env.example             # Environment template
```

### Running Tests

```bash
# Backend tests
cd backend
go test ./... -v

# Specific package
go test ./internal/mcp/... -v
go test ./internal/cloud/... -v
```

## API Reference

### Conversations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/conversations` | GET | List all conversations |
| `/api/conversations` | POST | Create new conversation |
| `/api/conversations/{id}` | GET | Get conversation details |
| `/api/conversations/{id}/messages` | POST | Send message (SSE stream) |

### Integrations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/integrations` | GET | List all integrations |
| `/api/integrations/{id}` | GET | Get integration details |
| `/api/integrations/{id}/connect` | POST | Connect integration |
| `/api/integrations/{id}/disconnect` | DELETE | Disconnect integration |
| `/api/apps` | GET | List connected apps |

### MCP

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/mcp/providers` | GET | List MCP providers |
| `/api/mcp/proxy` | POST | Proxy MCP tool calls |
| `/api/auth/connect-token` | POST | Get OAuth connect link |

### Cloud Credentials

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/cloud/credentials` | GET | List user's cloud credentials |
| `/api/cloud/credentials/aws` | POST | Store AWS role config |
| `/api/cloud/credentials/gcp` | POST | Store GCP service account |
| `/api/cloud/aws/credentials` | POST | Get AWS temp credentials (sandbox) |
| `/api/cloud/gcp/credentials` | POST | Get GCP access token (sandbox) |
| `/api/cloud/credentials/postgres` | POST | Store PostgreSQL credentials |

---

## Resources

- [E2B Documentation](https://e2b.dev/docs)
- [Pipedream Connect](https://pipedream.com/docs/connect)
- [Pipedream MCP](https://pipedream.com/docs/connect/mcp)
- [Composio Documentation](https://docs.composio.dev)
- [Sentry MCP](https://docs.sentry.io/product/sentry-mcp)
- [Model Context Protocol](https://modelcontextprotocol.io)
- [OpenAI API](https://platform.openai.com/docs)
- [AWS CLI Credential Process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)

## License

MIT License - see [LICENSE](LICENSE) for details.
