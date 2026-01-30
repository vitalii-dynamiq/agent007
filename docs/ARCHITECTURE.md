# Architecture Documentation

> Last updated: January 2026

## System Overview

Dynamiq is a multi-layered AI agent platform designed for secure tool integration. This document provides detailed technical architecture for developers.

## Table of Contents

1. [High-Level Architecture](#high-level-architecture)
2. [Backend Architecture](#backend-architecture)
3. [Frontend Architecture](#frontend-architecture)
4. [Security Architecture](#security-architecture)
5. [Integration System](#integration-system)
6. [Data Flow](#data-flow)
7. [Extension Points](#extension-points)

---

## High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              USER INTERFACE                                   │
│                                                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         React Application                                │ │
│  │  • Chat Interface (SSE streaming)                                       │ │
│  │  • Integration Management                                                │ │
│  │  • OAuth Connection Flows                                                │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │          HTTP/SSE             │
                    └───────────────┬───────────────┘
                                    │
┌───────────────────────────────────┴──────────────────────────────────────────┐
│                              API GATEWAY                                      │
│                                                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         Chi Router                                       │ │
│  │  • CORS Middleware                                                       │ │
│  │  • Request Logging                                                       │ │
│  │  • Error Recovery                                                        │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
┌───────────────────────────────────┴──────────────────────────────────────────┐
│                            BUSINESS LOGIC                                     │
│                                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │    Agent     │  │     MCP      │  │    Cloud     │  │ Integrations │     │
│  │ Orchestrator │  │   Registry   │  │   Manager    │  │   Registry   │     │
│  │              │  │              │  │              │  │              │     │
│  │ • Tool loop  │  │ • Pipedream  │  │ • AWS STS    │  │ • Catalog    │     │
│  │ • LLM calls  │  │ • Composio   │  │ • GCP IAM    │  │ • User state │     │
│  │ • Streaming  │  │ • Direct MCP │  │ • Encryption │  │ • Config gen │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
└───────────────────────────────────┬──────────────────────────────────────────┘
                                    │
┌───────────────────────────────────┴──────────────────────────────────────────┐
│                           EXTERNAL SERVICES                                   │
│                                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │    OpenAI    │  │     E2B      │  │   Pipedream  │  │   Composio   │     │
│  │   (LLM)      │  │  (Sandbox)   │  │    (MCP)     │  │    (MCP)     │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │
│  │   AWS STS    │  │   GCP IAM    │  │  Sentry MCP  │                       │
│  └──────────────┘  └──────────────┘  └──────────────┘                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Backend Architecture

### Package Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, dependency injection
│
└── internal/                     # Private packages (Go convention)
    │
    ├── agent/                    # Agent Orchestration
    │   └── agent.go             # LLM interaction loop, tool execution
    │
    ├── api/                      # HTTP Layer
    │   ├── handlers.go          # HTTP handlers
    │   └── router.go            # Route definitions, middleware
    │
    ├── auth/                     # Authentication
    │   └── tokens.go            # JWT generation/validation
    │
    ├── cloud/                    # Cloud Credential Management
    │   ├── types.go             # Shared types
    │   ├── store.go             # Encrypted credential storage
    │   ├── aws.go               # AWS STS integration
    │   ├── gcp.go               # GCP token generation
    │   ├── manager.go           # Orchestration layer
    │   └── handlers.go          # HTTP handlers
    │
    ├── config/                   # Configuration
    │   └── config.go            # Environment variable loading
    │
    ├── e2b/                      # E2B Sandbox
    │   ├── client.go            # Sandbox lifecycle
    │   └── executor.go          # Command execution, file ops
    │
    ├── integrations/             # Integration System
    │   ├── types.go             # Integration, Provider types
    │   ├── catalog.go           # Pre-defined integrations
    │   ├── registry.go          # User integration state
    │   └── handlers.go          # HTTP handlers
    │
    ├── llm/                      # LLM Abstraction
    │   ├── client.go            # Interface definition
    │   └── openai.go            # OpenAI implementation
    │
    ├── mcp/                      # MCP Provider System
    │   ├── types.go             # Tool, Provider types
    │   ├── registry.go          # Multi-provider registry
    │   ├── pipedream.go         # Pipedream implementation
    │   ├── composio.go          # Composio implementation
    │   └── direct.go            # Direct MCP server support
    │
    └── store/                    # Data Storage
        └── memory.go            # In-memory conversation store
```

### Key Design Patterns

#### 1. Provider Pattern (MCP)

```go
// Provider interface allows pluggable MCP backends
type Provider interface {
    Info() ProviderInfo
    Name() string
    ListTools(ctx context.Context, userID, app string) ([]Tool, error)
    CallTool(ctx context.Context, userID, app, tool string, input map[string]interface{}) (*ToolResult, error)
    GetConnectToken(ctx context.Context, userID string) (string, error)
    ListConnectedApps(ctx context.Context, userID string) ([]ConnectedApp, error)
}

// Registry routes to appropriate provider
type Registry struct {
    providers map[string]Provider
    defaultProvider string
}

// Supports "provider:app" syntax for explicit routing
// e.g., "pipedream:gmail" or "composio:jira"
```

#### 2. Factory Pattern (Provider Registration)

```go
// Providers registered via factory functions
r.RegisterFactory(ProviderTypePipedream, func(cfg ProviderConfig) (Provider, error) {
    return NewPipedreamProvider(cfg.Extra["clientId"], cfg.Extra["clientSecret"], cfg.ProjectID, env), nil
})

// Create providers from configuration
registry.CreateProvider(ProviderConfig{
    Type:      ProviderTypePipedream,
    Name:      "pipedream",
    ProjectID: "proj_xxx",
    Extra:     map[string]string{"clientId": "...", "clientSecret": "..."},
})
```

#### 3. Credential Helper Pattern (Cloud)

```go
// Sandbox never receives long-lived credentials
// Instead, it runs a helper that calls back to our backend

func (m *Manager) generateAWSCredentialHelper(sessionToken, sandboxID string) string {
    return fmt.Sprintf(`#!/bin/bash
# Calls backend with session token
response=$(curl -s -X POST "${BACKEND_URL}/api/cloud/aws/credentials" \
  -H "Authorization: Bearer ${SESSION_TOKEN}" \
  -d '{"sandboxId": "%s"}')
# Outputs in AWS credential_process format
echo "$response" | jq '{Version: 1, AccessKeyId: ..., ...}'
`, sandboxID)
}
```

### Request Flow

```
1. HTTP Request arrives at Chi router
2. Middleware: CORS, Logging, Recovery
3. Handler extracts user ID (from header or session)
4. Handler calls appropriate service layer
5. Service interacts with external APIs/databases
6. Response formatted and sent

For SSE (chat):
1. POST /api/conversations/{id}/messages
2. Handler sets up SSE writer
3. Agent.Run() called with event channel
4. Events streamed to client (status, tool_call, message, etc.)
5. Connection closed on "done" event
```

---

## Frontend Architecture

### Component Structure

```
frontend/src/
├── components/
│   ├── chat/                     # Chat Feature
│   │   ├── chat-view.tsx        # Main chat container, SSE handling
│   │   ├── message.tsx          # Message display (user/assistant/tool)
│   │   └── message-input.tsx    # Input with submit handling
│   │
│   ├── connect/                  # OAuth Connections
│   │   └── connect-apps.tsx     # App connection UI
│   │
│   ├── sidebar/                  # Navigation
│   │   └── conversation-list.tsx # Conversation history
│   │
│   └── ui/                       # Shared Primitives (Radix-based)
│       ├── button.tsx           # Button component
│       ├── input.tsx            # Input component
│       └── scroll-area.tsx      # Scrollable container
│
├── lib/
│   ├── api.ts                   # Backend API client
│   └── utils.ts                 # Utility functions (cn, etc.)
│
├── App.tsx                      # Root component, layout
└── main.tsx                     # Entry point
```

### UI Framework

- **Radix UI**: Accessible primitives (https://radix-ui.com)
- **Radix Icons**: Icon set (https://icons.radix-ui.com)
- **TailwindCSS**: Utility-first CSS
- **shadcn/ui patterns**: Component styling approach

### State Management

- Local state with `useState`/`useReducer`
- SSE streaming state for real-time updates
- No global state library (kept simple)

---

## Security Architecture

### Credential Security

```
┌─────────────────────────────────────────────────────────────────┐
│                    CREDENTIAL STORAGE                            │
│                                                                  │
│  User provides:                                                  │
│  • AWS Role ARN                                                  │
│  • GCP Service Account JSON                                      │
│  • OAuth tokens (via flow)                                       │
│  • API keys                                                      │
│                         │                                        │
│                         ▼                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              AES-256-GCM Encryption                      │   │
│  │              (key from JWT_SECRET)                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                         │                                        │
│                         ▼                                        │
│  Stored in memory (production: use database with encryption)    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    SANDBOX CREDENTIALS                           │
│                                                                  │
│  1. Backend generates session token (5 min TTL)                 │
│     - Contains: userID, sandboxID, scopes, nonce                │
│     - Signed with JWT_SECRET                                    │
│                                                                  │
│  2. Token injected into sandbox as env var                      │
│     - MCP_SESSION_TOKEN                                         │
│                                                                  │
│  3. Credential helper uses token to request real creds          │
│     - Backend validates token                                   │
│     - Backend calls AWS STS / GCP IAM                          │
│     - Returns short-lived credentials (1hr max)                 │
│                                                                  │
│  4. Sandbox CLI uses short-lived credentials                    │
│     - Auto-refresh via credential_process                       │
└─────────────────────────────────────────────────────────────────┘
```

### Token Scopes

```go
type Scope string

const (
    ScopeListTools  Scope = "mcp:list_tools"
    ScopeCallTools  Scope = "mcp:call_tools"
    ScopeCloudAWS   Scope = "cloud:aws"
    ScopeCloudGCP   Scope = "cloud:gcp"
)

// Tokens are scoped to specific operations
// Sandbox tokens only get the scopes they need
```

### Security Checklist

- [x] Credentials encrypted at rest (AES-256-GCM)
- [x] Short-lived session tokens (5 min TTL)
- [x] Sandbox never receives long-lived credentials
- [x] Token scoping for least privilege
- [x] Nonce in tokens to prevent replay
- [ ] Rate limiting (TODO)
- [ ] Audit logging (TODO)
- [ ] Database encryption (TODO - currently in-memory)

---

## Integration System

### Integration Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                    INTEGRATION CATALOG                           │
│                                                                  │
│  Defined in: backend/internal/integrations/catalog.go           │
│                                                                  │
│  struct Integration {                                            │
│    ID, Name, Description, Category                               │
│    ProviderType: cli | cloud_cli | mcp | direct_mcp | api       │
│    AuthType: oauth2 | api_key | token | iam_role | svc_account  │
│    MCPProvider: "pipedream" | "composio" | ""                   │
│    CLICommand, CLIInstallCmd, CLIAuthCmd                        │
│    AgentInstructions (for prompt)                                │
│    Capabilities []string                                         │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    USER CONNECTS INTEGRATION                     │
│                                                                  │
│  POST /api/integrations/{id}/connect                            │
│                                                                  │
│  For OAuth2:                                                     │
│    1. Frontend initiates OAuth flow                             │
│    2. User authorizes on provider site                          │
│    3. Callback exchanges code for token                         │
│    4. Token stored encrypted                                    │
│                                                                  │
│  For API Key:                                                    │
│    1. User enters API key in UI                                 │
│    2. Key stored encrypted                                      │
│                                                                  │
│  For IAM Role (AWS):                                            │
│    1. User enters Role ARN                                      │
│    2. Config stored (no secrets)                                │
│    3. Backend assumes role when needed                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    AGENT USES INTEGRATION                        │
│                                                                  │
│  1. Agent context generated from enabled integrations           │
│  2. System prompt includes integration instructions             │
│  3. Agent makes tool calls based on provider type:              │
│                                                                  │
│  MCP: list_app_tools → call_app_tool                           │
│  CLI: Execute command in sandbox                                │
│  API: curl with injected credentials                            │
└─────────────────────────────────────────────────────────────────┘
```

### Provider Type Decision Tree

```
Is there an official MCP server?
  └─ YES → Use direct_mcp (e.g., Sentry)
  └─ NO ↓

Does the service have a comprehensive CLI?
  └─ YES → Use cli (e.g., GitHub gh, Stripe)
  └─ NO ↓

Is it a cloud provider (AWS/GCP)?
  └─ YES → Use cloud_cli with credential injection
  └─ NO ↓

Does Pipedream have good support (many tools)?
  └─ YES → Use mcp with pipedream
  └─ NO ↓

Does Composio have support?
  └─ YES → Use mcp with composio
  └─ NO ↓

Fall back to api with direct REST calls
```

---

## Data Flow

### Chat Message Flow

```
1. User sends message
   │
   ▼
2. POST /api/conversations/{id}/messages
   │
   ▼
3. SSE stream opened
   │
   ▼
4. Agent.Run() starts
   │
   ├─→ Send "status" event (Initializing...)
   │
   ├─→ Build messages array with system prompt
   │   (includes integration instructions)
   │
   ├─→ Call LLM
   │   │
   │   ├─→ Send "thinking" event
   │   │
   │   └─→ LLM returns response
   │       │
   │       ├─→ If tool_calls present:
   │       │   │
   │       │   ├─→ Send "tool_call" event for each
   │       │   │
   │       │   ├─→ Execute tool (MCP/CLI/API)
   │       │   │
   │       │   ├─→ Send "tool_result" event
   │       │   │
   │       │   └─→ Loop back to Call LLM
   │       │
   │       └─→ If content (no more tools):
   │           │
   │           └─→ Send "message" event
   │
   └─→ Send "done" event
       │
       ▼
5. SSE stream closed
```

### Credential Flow (AWS Example)

```
1. User configures AWS integration
   │
   ├─→ Enters Role ARN: arn:aws:iam::123456789012:role/AgentRole
   │
   └─→ Stored in CredentialStore (not encrypted - no secrets)
   │
   ▼
2. Agent starts, sandbox created
   │
   ├─→ GenerateSandboxCredentialConfig() called
   │   │
   │   ├─→ Session token generated (5 min TTL)
   │   │
   │   ├─→ AWS credential helper script generated
   │   │
   │   └─→ ~/.aws/config generated with credential_process
   │
   └─→ Files/scripts injected into sandbox
   │
   ▼
3. Agent runs `aws s3 ls` in sandbox
   │
   ├─→ AWS CLI reads ~/.aws/config
   │
   ├─→ Executes credential_process (our helper)
   │   │
   │   ├─→ Helper calls POST /api/cloud/aws/credentials
   │   │   with session token
   │   │
   │   ├─→ Backend validates token
   │   │
   │   ├─→ Backend calls AWS STS AssumeRole
   │   │
   │   └─→ Returns temporary credentials
   │
   └─→ AWS CLI uses temporary credentials
```

---

## Extension Points

### Adding a New MCP Provider

1. Create `backend/internal/mcp/newprovider.go`:

```go
type NewProvider struct {
    // fields
}

func NewNewProvider(apiKey string) *NewProvider {
    return &NewProvider{...}
}

func (p *NewProvider) Info() ProviderInfo { ... }
func (p *NewProvider) Name() string { return "newprovider" }
func (p *NewProvider) ListTools(...) ([]Tool, error) { ... }
func (p *NewProvider) CallTool(...) (*ToolResult, error) { ... }
func (p *NewProvider) GetConnectToken(...) (string, error) { ... }
func (p *NewProvider) ListConnectedApps(...) ([]ConnectedApp, error) { ... }
```

2. Register factory in `registry.go`:

```go
r.RegisterFactory(ProviderTypeNew, func(cfg ProviderConfig) (Provider, error) {
    return NewNewProvider(cfg.APIKey), nil
})
```

### Adding a New LLM Provider

1. Implement `llm.Client` interface in new file:

```go
type AnthropicClient struct { ... }

func (c *AnthropicClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) { ... }
func (c *AnthropicClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) { ... }
```

2. Update `llm.NewClient()` factory

### Adding a New Cloud Provider

1. Create `backend/internal/cloud/azure.go` (for example)
2. Implement credential exchange logic
3. Add to `Manager` and `handlers.go`

---

## External Documentation Links

| Component | Documentation |
|-----------|---------------|
| E2B Sandbox | https://e2b.dev/docs |
| Pipedream Connect | https://pipedream.com/docs/connect |
| Pipedream MCP | https://pipedream.com/docs/connect/mcp |
| Composio | https://docs.composio.dev |
| Sentry MCP | https://docs.sentry.io/product/sentry-mcp |
| AWS STS | https://docs.aws.amazon.com/STS/latest/APIReference |
| AWS credential_process | https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html |
| GCP Workload Identity | https://cloud.google.com/iam/docs/workload-identity-federation |
| OpenAI API | https://platform.openai.com/docs/api-reference |
| Chi Router | https://go-chi.io |
| Radix UI | https://radix-ui.com |
| TailwindCSS | https://tailwindcss.com/docs |

---

*This document should be updated when significant architectural changes are made.*
