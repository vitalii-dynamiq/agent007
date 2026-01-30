# Dynamiq: AI Agent Platform with Tool Integrations

> A production-ready AI agent platform that enables LLMs to interact with external services (GitHub, Slack, AWS, etc.) through a secure sandbox environment.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-19+-61DAFB?style=flat&logo=react)](https://reactjs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Integrations](#integrations)
- [Security Model](#security-model)
- [Development](#development)
- [API Reference](#api-reference)
- [Contributing](#contributing)

## Overview

Dynamiq is an AI agent platform that allows Large Language Models to securely interact with external tools and services. The agent runs code in isolated [E2B](https://e2b.dev) sandboxes and communicates with services through:

- **MCP (Model Context Protocol)** via [Pipedream](https://pipedream.com) and [Composio](https://composio.dev)
- **Official CLIs** (GitHub CLI, Stripe CLI, AWS CLI, etc.)
- **Direct APIs** with secure credential management

### Key Principles

1. **Security First**: User credentials never enter sandboxes; only short-lived tokens
2. **Modularity**: Easy to add new integrations without changing core code
3. **Extensibility**: Support for multiple provider types (MCP, CLI, API, Cloud)
4. **Developer Experience**: Clean Go code following best practices, modern React UI

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                 FRONTEND                                     │
│                     React 19 + Radix UI + TailwindCSS                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │  Chat View  │  │ Integrations│  │  Settings   │  │   OAuth     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │ REST API + SSE
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                 BACKEND                                      │
│                          Go 1.24 + Chi Router                               │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                          API Layer (/api)                             │  │
│  │  • Conversations  • Integrations  • Cloud Credentials  • MCP Proxy   │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │   Agent    │  │    LLM     │  │    MCP     │  │   Cloud    │           │
│  │ Orchestrator│  │  Adapter   │  │  Registry  │  │  Manager   │           │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘           │
│        │               │               │               │                    │
│        ▼               ▼               ▼               ▼                    │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │    E2B     │  │   OpenAI   │  │  Pipedream │  │  AWS STS   │           │
│  │  Sandbox   │  │  GPT-5.2   │  │  Composio  │  │  GCP IAM   │           │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘           │
└─────────────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            E2B SANDBOX                                       │
│                    Isolated Linux Environment                                │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Pre-installed CLIs: gh, aws, gcloud, stripe, vercel, etc.          │   │
│  │  Credential Helpers: aws-credential-helper, gcp-credential-helper   │   │
│  │  MCP CLI: For Pipedream/Composio tool calls                         │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  Credentials injected via:                                                  │
│  • Environment variables (short-lived tokens)                               │
│  • credential_process (AWS)                                                 │
│  • Application Default Credentials (GCP)                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Features

### 39 Pre-configured Integrations

| Category | Integrations |
|----------|--------------|
| **Developer Tools** | GitHub, Stripe, Vercel, Supabase, Neon, Cloudflare, Sentry, Linear, Jira, Confluence, ClickUp |
| **Cloud** | AWS, Google Cloud, Microsoft Azure, IBM Cloud, Oracle Cloud, Kubernetes |
| **Data & Analytics** | Snowflake, Databricks |
| **Productivity** | Gmail, Google Calendar, Google Drive, OneDrive, SharePoint, Notion, Asana, Monday.com, Trello, Airtable, HubSpot |
| **Communication** | Slack, Discord, Microsoft Teams, Outlook |
| **Monitoring** | Datadog, New Relic, PagerDuty |

### Provider Types

| Type | Description | Examples |
|------|-------------|----------|
| `cli` | Official CLI tools | GitHub (gh), Stripe, Vercel |
| `cloud_cli` | Cloud provider CLIs with credential injection | AWS, GCP |
| `mcp` | MCP via Pipedream/Composio | Gmail, Slack, Notion |
| `direct_mcp` | Official MCP servers | Sentry |
| `api` | Direct REST API access | Datadog, PagerDuty |

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+
- Docker (for E2B custom template building)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/dynamiq-agent-platform.git
cd dynamiq-agent-platform

# Backend setup
cd backend
cp .env.example .env
# Edit .env with your API keys
go mod download
go build -o bin/server ./cmd/server

# Frontend setup
cd ../frontend
npm install
npm run build
```

### Build E2B Custom Template (Recommended)

For instant sandbox startup, build the custom template with pre-installed CLIs:

```bash
# Install E2B CLI
npm install -g @e2b/cli

# Login to E2B
e2b auth login

# Build custom template (takes ~5-10 minutes first time)
cd e2b-template
make build

# Note the template ID from the output, then add to .env:
# E2B_TEMPLATE_ID=dynamiq-agent-sandbox
```

See [e2b-template/README.md](e2b-template/README.md) for details.

### Running

```bash
# Terminal 1: Backend
cd backend
./bin/server

# Terminal 2: Frontend (development)
cd frontend
npm run dev
```

Visit http://localhost:5173

## Configuration

### Required Environment Variables

```bash
# .env
# LLM Configuration
LLM_API_KEY=sk-...              # OpenAI API key
LLM_MODEL=gpt-5.2               # Model to use

# E2B Sandbox
E2B_API_KEY=e2b_...             # E2B API key (https://e2b.dev)

# MCP Providers
PIPEDREAM_CLIENT_ID=...         # Pipedream Connect (https://pipedream.com/docs/connect)
PIPEDREAM_CLIENT_SECRET=...
PIPEDREAM_PROJECT_ID=proj_...

COMPOSIO_API_KEY=...            # Composio (https://composio.dev)
COMPOSIO_PROJECT_ID=...

# Security
JWT_SECRET=your-32-byte-secret  # For session tokens
```

### OAuth Credentials (Optional)

For CLI-based integrations that use OAuth:

```bash
# See .env.integrations.example for full list
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
```

## Integrations

### Adding a New Integration

1. **Add to catalog** (`backend/internal/integrations/catalog.go`):

```go
"myservice": {
    ID:           "myservice",
    Name:         "My Service",
    Description:  "Description here",
    Category:     CategoryDeveloperTools,
    ProviderType: ProviderMCP,        // or ProviderCLI, ProviderAPI, etc.
    AuthType:     AuthOAuth2,         // or AuthAPIKey, AuthToken
    MCPProvider:  "pipedream",        // if using MCP
    MCPAppSlug:   "myservice",
    AgentInstructions: `Instructions for the agent...`,
    Capabilities: []string{"feature1", "feature2"},
    Enabled:      true,
},
```

2. **Configure OAuth** (if needed) in `backend/internal/config/config.go`

3. **Test the integration**:
```bash
curl http://localhost:8080/api/integrations/myservice
```

### Integration Documentation Links

| Integration | Documentation |
|-------------|---------------|
| E2B | https://e2b.dev/docs |
| Pipedream MCP | https://pipedream.com/docs/connect/mcp |
| Composio | https://docs.composio.dev |
| GitHub CLI | https://cli.github.com/manual |
| AWS CLI | https://docs.aws.amazon.com/cli |
| GCP CLI | https://cloud.google.com/sdk/docs |
| Azure CLI | https://learn.microsoft.com/en-us/cli/azure |
| IBM Cloud CLI | https://cloud.ibm.com/docs/cli |
| Oracle OCI CLI | https://docs.oracle.com/en-us/iaas/tools/oci-cli |
| kubectl | https://kubernetes.io/docs/reference/kubectl |
| Sentry MCP | https://docs.sentry.io/product/sentry-mcp |

## Security Model

### Credential Flow

```
User Credentials (stored encrypted in backend)
         │
         ▼
Backend generates short-lived token (5 min TTL)
         │
         ▼
Token injected into E2B sandbox as env var
         │
         ▼
Sandbox CLI calls backend with token
         │
         ▼
Backend validates token, fetches real credentials
         │
         ▼
Backend calls external service (AWS STS, GCP IAM, etc.)
         │
         ▼
Returns short-lived credentials to sandbox
```

### Security Measures

1. **Credential Isolation**: User secrets never enter sandboxes
2. **Encrypted Storage**: AES-256-GCM for all stored credentials
3. **Short-lived Tokens**: 5-minute TTL for sandbox session tokens
4. **Scoped Permissions**: Tokens limited to specific operations
5. **Audit Logging**: All credential access logged

### AWS Credential Flow

```bash
# In sandbox ~/.aws/config:
[default]
credential_process = /usr/local/bin/aws-credential-helper

# Credential helper calls backend, which calls STS AssumeRole
```

### GCP Credential Flow

Uses Workload Identity Federation pattern - sandbox presents JWT, exchanges for GCP access token.

## Development

### Project Structure

```
dynamiq/
├── backend/
│   ├── cmd/server/          # Application entry point
│   ├── internal/
│   │   ├── agent/           # LLM agent orchestration
│   │   ├── api/             # HTTP handlers and routing
│   │   ├── auth/            # JWT token management
│   │   ├── cloud/           # AWS/GCP credential management
│   │   ├── config/          # Configuration loading
│   │   ├── e2b/             # E2B sandbox client
│   │   ├── integrations/    # Integration registry and catalog
│   │   ├── llm/             # LLM client abstraction
│   │   ├── mcp/             # MCP provider implementations
│   │   └── store/           # In-memory data store
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/      # React components
│   │   │   ├── chat/        # Chat UI components
│   │   │   ├── connect/     # OAuth connection UI
│   │   │   ├── sidebar/     # Navigation sidebar
│   │   │   └── ui/          # Shared UI primitives (Radix)
│   │   ├── lib/             # Utilities and API client
│   │   └── App.tsx          # Main application
│   └── package.json
├── mcp-cli/                  # MCP CLI binary for sandboxes
└── README.md
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

### Code Style

**Go**: Follow [Effective Go](https://golang.org/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

**TypeScript/React**: ESLint with recommended rules, Prettier for formatting

### Adding Tests

```go
// Example: backend/internal/mcp/registry_test.go
func TestRegistryListTools(t *testing.T) {
    registry := NewRegistry()
    registry.AddProvider("mock", NewMockProvider("mock"))
    registry.SetDefaultProvider("mock")

    tools, err := registry.ListTools(context.Background(), "user1", "app")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // assertions...
}
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
| `/api/integrations/agent-context` | GET | Get agent prompt context |

### Cloud Credentials

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/cloud/credentials` | GET | List user's cloud credentials |
| `/api/cloud/credentials/aws` | POST | Store AWS role config |
| `/api/cloud/credentials/gcp` | POST | Store GCP service account |
| `/api/cloud/aws/credentials` | POST | Get AWS temp credentials (sandbox) |
| `/api/cloud/gcp/credentials` | POST | Get GCP access token (sandbox) |

### MCP

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/mcp/providers` | GET | List MCP providers |
| `/api/mcp/proxy` | POST | Proxy MCP requests |

## Architecture Decisions

### Why Multiple Provider Types?

Different services have different optimal access patterns:

1. **CLI (GitHub, Stripe)**: Best developer experience, comprehensive features
2. **MCP (Gmail, Slack)**: Unified interface via Pipedream/Composio
3. **Cloud CLI (AWS, GCP)**: Native credential management with STS/IAM
4. **API (Datadog)**: When no good CLI/MCP exists

### Why E2B Sandboxes?

- **Security**: Isolated execution environment
- **Reproducibility**: Clean state for each session
- **Flexibility**: Install any tools needed

### Why Credential Helpers?

Following AWS/GCP best practices:
- Credentials fetched on-demand
- Short-lived (auto-refresh)
- Never stored on disk in plaintext

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes following the code style guide
4. Add tests for new functionality
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Resources

- [E2B Documentation](https://e2b.dev/docs)
- [Pipedream Connect](https://pipedream.com/docs/connect)
- [Composio Documentation](https://docs.composio.dev)
- [Model Context Protocol](https://modelcontextprotocol.io)
- [OpenAI API](https://platform.openai.com/docs)
- [AWS CLI Credential Process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
