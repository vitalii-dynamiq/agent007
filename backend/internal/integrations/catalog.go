package integrations

// Catalog contains all available integrations
// Add new integrations here - this is the single source of truth
//
// Provider Selection Logic:
// 1. Direct MCP - if service has official MCP server (Sentry)
// 2. CLI - if service has comprehensive CLI (GitHub, Stripe, Vercel, etc.)
// 3. Cloud CLI - for cloud providers (AWS, GCP)
// 4. MCP/Pipedream - for services with good Pipedream support (3000+ apps)
// 5. MCP/Composio - for services with better Composio support
// 6. API - last resort for services without good MCP/CLI support
var Catalog = map[string]*Integration{

	// ============================================================================
	// DEVELOPER TOOLS - CLI Based (Best developer experience)
	// ============================================================================

	"github": {
		ID:           "github",
		Name:         "GitHub",
		Description:  "Code hosting, issues, pull requests, actions, gists",
		Category:     CategoryDeveloperTools,
		Icon:         "🐙",
		ProviderType: ProviderCLI,
		AuthType:     AuthOAuth2,
		CLICommand:   "gh",
		CLIInstallCmd: `python3 - <<'PY'
import json
import os
import tarfile
import urllib.request

api = "https://api.github.com/repos/cli/cli/releases/latest"
data = json.loads(urllib.request.urlopen(api).read())
asset = next(a for a in data["assets"] if a["name"].endswith("linux_amd64.tar.gz"))
url = asset["browser_download_url"]
tar_data = urllib.request.urlopen(url).read()
target = "/home/user/.local/gh"
os.makedirs(target, exist_ok=True)
tarfile.open(fileobj=__import__("io").BytesIO(tar_data), mode="r:gz").extractall(target)
folder = next(d for d in os.listdir(target) if d.startswith("gh_") and d.endswith("_linux_amd64"))
bin_path = os.path.join(target, folder, "bin", "gh")
os.makedirs("/home/user/.local/bin", exist_ok=True)
os.rename(bin_path, "/home/user/.local/bin/gh-real")
os.chmod("/home/user/.local/bin/gh-real", 0o755)
PY`,
		CLIAuthCmd: "GH_TOKEN=<token> gh <command>",
		OAuth2Config: &OAuth2Config{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
			Scopes:   []string{"repo", "read:org", "workflow", "gist", "read:user", "user:email"},
			// ClientID and ClientSecret set via config
		},
		AgentInstructions: `Use the GitHub CLI (gh) for all GitHub operations. The CLI is pre-authenticated with a short-lived token.

Common commands:
- Repos: gh repo list, gh repo clone <repo>, gh repo view
- Issues: gh issue list, gh issue create --title "..." --body "...", gh issue view <num>
- PRs: gh pr list, gh pr create --title "..." --body "...", gh pr view <num>, gh pr merge <num>
- Actions: gh run list, gh run view <id>, gh workflow list
- Releases: gh release list, gh release create <tag>

Run 'gh help' or 'gh <command> --help' for detailed usage.`,
		Capabilities: []string{"repos", "issues", "pull_requests", "actions", "gists", "releases", "workflows", "packages"},
		Enabled:      true,
	},

	"stripe": {
		ID:           "stripe",
		Name:         "Stripe",
		Description:  "Payment processing, subscriptions, invoices",
		Category:     CategoryDeveloperTools,
		Icon:         "💳",
		ProviderType: ProviderCLI,
		AuthType:     AuthAPIKey,
		CLICommand:   "stripe",
		CLIInstallCmd: `curl -s https://packages.stripe.dev/api/security/keypair/stripe-cli-gpg/public | gpg --dearmor | tee /usr/share/keyrings/stripe.gpg
echo "deb [signed-by=/usr/share/keyrings/stripe.gpg] https://packages.stripe.dev/stripe-cli-debian-local stable main" | tee -a /etc/apt/sources.list.d/stripe.list
apt update && apt install stripe -y`,
		CLIAuthCmd: "stripe login --api-key $STRIPE_API_KEY",
		AgentInstructions: `Use the Stripe CLI for payment operations. The CLI is pre-authenticated.

Common commands:
- Customers: stripe customers list, stripe customers create --email "..."
- Payments: stripe payment_intents list, stripe payment_intents create --amount 1000 --currency usd
- Subscriptions: stripe subscriptions list, stripe subscriptions create --customer <id> --price <price_id>
- Products: stripe products list, stripe prices list
- Webhooks: stripe listen --forward-to localhost:3000/webhook
- Logs: stripe logs tail

Run 'stripe help' or 'stripe <resource> --help' for details.`,
		Capabilities: []string{"customers", "payments", "subscriptions", "invoices", "products", "webhooks"},
		Enabled:      true,
	},

	"vercel": {
		ID:            "vercel",
		Name:          "Vercel",
		Description:   "Frontend deployment platform, serverless functions",
		Category:      CategoryDeveloperTools,
		Icon:          "▲",
		ProviderType:  ProviderCLI,
		AuthType:      AuthToken,
		CLICommand:    "vercel",
		CLIInstallCmd: "npm install -g vercel",
		CLIAuthCmd:    "", // Token passed via --token flag or VERCEL_TOKEN env
		AgentInstructions: `Use the Vercel CLI for deployments. The CLI is pre-authenticated via VERCEL_TOKEN.

Common commands:
- Deploy: vercel --prod (or just 'vercel' for preview)
- Projects: vercel project ls, vercel project add
- Domains: vercel domains ls, vercel domains add <domain>
- Env vars: vercel env ls, vercel env add <name>
- Logs: vercel logs <deployment-url>
- Secrets: vercel secrets ls

Run 'vercel help' for all commands.`,
		Capabilities: []string{"deployments", "projects", "domains", "env_vars", "logs", "functions"},
		Enabled:      true,
	},

	"supabase": {
		ID:            "supabase",
		Name:          "Supabase",
		Description:   "Backend-as-a-service: Postgres, Auth, Storage, Functions",
		Category:      CategoryDeveloperTools,
		Icon:          "⚡",
		ProviderType:  ProviderCLI,
		AuthType:      AuthToken,
		CLICommand:    "supabase",
		CLIInstallCmd: "npm install -g supabase",
		CLIAuthCmd:    "supabase login --token $SUPABASE_ACCESS_TOKEN",
		AgentInstructions: `Use the Supabase CLI for database and backend operations. Pre-authenticated.

Common commands:
- Projects: supabase projects list
- Database: supabase db diff, supabase db push, supabase db reset
- Migrations: supabase migration new <name>, supabase migration list
- Functions: supabase functions deploy <name>, supabase functions serve
- Secrets: supabase secrets set <name>=<value>, supabase secrets list
- Types: supabase gen types typescript --project-id <id>

Run 'supabase help' for all commands.`,
		Capabilities: []string{"database", "migrations", "functions", "auth", "storage", "realtime"},
		Enabled:      true,
	},

	"neon": {
		ID:            "neon",
		Name:          "Neon",
		Description:   "Serverless Postgres with branching",
		Category:      CategoryDeveloperTools,
		Icon:          "🐘",
		ProviderType:  ProviderCLI,
		AuthType:      AuthAPIKey,
		CLICommand:    "neonctl",
		CLIInstallCmd: "npm install -g neonctl",
		CLIAuthCmd:    "", // Uses NEON_API_KEY env var
		AgentInstructions: `Use neonctl for Neon Postgres operations. Pre-authenticated via NEON_API_KEY.

Common commands:
- Projects: neonctl projects list, neonctl projects create --name <name>
- Branches: neonctl branches list, neonctl branches create --name <name>
- Databases: neonctl databases list, neonctl databases create --name <name>
- Roles: neonctl roles list
- Connection: neonctl connection-string

Run 'neonctl help' for all commands.`,
		Capabilities: []string{"projects", "branches", "databases", "roles", "endpoints"},
		Enabled:      true,
	},

	"cloudflare": {
		ID:            "cloudflare",
		Name:          "Cloudflare",
		Description:   "CDN, Workers, Pages, DNS, security",
		Category:      CategoryDeveloperTools,
		Icon:          "☁️",
		ProviderType:  ProviderCLI,
		AuthType:      AuthAPIKey,
		CLICommand:    "wrangler",
		CLIInstallCmd: "npm install -g wrangler",
		CLIAuthCmd:    "", // Uses CLOUDFLARE_API_TOKEN env var
		AgentInstructions: `Use Wrangler CLI for Cloudflare operations. Pre-authenticated via CLOUDFLARE_API_TOKEN.

Common commands:
- Workers: wrangler deploy, wrangler dev, wrangler tail
- Pages: wrangler pages deploy <dir>, wrangler pages project list
- KV: wrangler kv namespace list, wrangler kv key list --namespace-id <id>
- R2: wrangler r2 bucket list, wrangler r2 object get <bucket> <key>
- D1: wrangler d1 list, wrangler d1 execute <db> --command "SELECT..."
- Secrets: wrangler secret put <name>

Run 'wrangler --help' for all commands.`,
		Capabilities: []string{"workers", "pages", "kv", "r2", "d1", "dns", "secrets"},
		Enabled:      true,
	},

	// ============================================================================
	// DEVELOPER TOOLS - Direct MCP (Official MCP servers)
	// ============================================================================

	"sentry": {
		ID:           "sentry",
		Name:         "Sentry",
		Description:  "Error tracking, performance monitoring, release health",
		Category:     CategoryDeveloperTools,
		Icon:         "🐛",
		ProviderType: ProviderDirectMCP,
		AuthType:     AuthOAuth2,
		MCPServerURL: "https://mcp.sentry.dev/mcp",
		OAuth2Config: &OAuth2Config{
			AuthURL:  "https://mcp.sentry.dev/oauth/authorize",
			TokenURL: "https://mcp.sentry.dev/oauth/token",
			Scopes:   []string{"org:read", "project:write", "team:write", "event:write"},
		},
		AgentInstructions: `Sentry has an official MCP server with 16+ tools. Use MCP tools:
- list_app_tools(app="sentry") to discover available actions
- call_app_tool(app="sentry", tool="...", input={...}) to execute

Available capabilities: search issues, get error details, list projects, manage DSNs, view release health.`,
		Capabilities: []string{"issues", "events", "projects", "releases", "dsns", "performance"},
		Enabled:      true,
	},

	// ============================================================================
	// DEVELOPER TOOLS - MCP Based
	// ============================================================================

	"linear": {
		ID:           "linear",
		Name:         "Linear",
		Description:  "Issue tracking and project management for software teams",
		Category:     CategoryDeveloperTools,
		Icon:         "📐",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "linear_app",
		AgentInstructions: `Use MCP tools for Linear:
- list_app_tools(app="linear") to see available actions
- call_app_tool(app="linear", tool="...", input={...}) to execute

Common actions: create issues, update status, manage projects, search issues.`,
		Capabilities: []string{"issues", "projects", "teams", "cycles", "labels"},
		Enabled:      true,
	},

	"jira": {
		ID:           "jira",
		Name:         "Jira",
		Description:  "Issue tracking and agile project management",
		Category:     CategoryDeveloperTools,
		Icon:         "📋",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "jira",
		AgentInstructions: `Use MCP tools for Jira:
- list_app_tools(app="jira") to see available actions
- call_app_tool(app="jira", tool="...", input={...}) to execute

Common actions: create/update issues, search with JQL, manage sprints, transitions.`,
		Capabilities: []string{"issues", "projects", "boards", "sprints", "workflows"},
		Enabled:      true,
	},

	"confluence": {
		ID:           "confluence",
		Name:         "Confluence",
		Description:  "Team wiki and documentation",
		Category:     CategoryDeveloperTools,
		Icon:         "📚",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "confluence",
		AgentInstructions: `Use MCP tools for Confluence:
- list_app_tools(app="confluence") to see available actions
- call_app_tool(app="confluence", tool="...", input={...}) to execute

Common actions: create/update pages, search content, manage spaces.`,
		Capabilities: []string{"pages", "spaces", "search", "attachments"},
		Enabled:      true,
	},

	"clickup": {
		ID:           "clickup",
		Name:         "ClickUp",
		Description:  "Project management and productivity platform",
		Category:     CategoryDeveloperTools,
		Icon:         "✅",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "clickup",
		AgentInstructions: `Use MCP tools for ClickUp:
- list_app_tools(app="clickup") to see available actions
- call_app_tool(app="clickup", tool="...", input={...}) to execute`,
		Capabilities: []string{"tasks", "lists", "spaces", "goals", "docs"},
		Enabled:      true,
	},

	// ============================================================================
	// CLOUD PROVIDERS - Cloud CLI
	// ============================================================================

	"aws": {
		ID:           "aws",
		Name:         "AWS",
		Description:  "Amazon Web Services - comprehensive cloud platform",
		Category:     CategoryCloud,
		Icon:         "🔶",
		ProviderType: ProviderCloudCLI,
		AuthType:     AuthAWSAccessKey,
		CLICommand:   "aws",
		CLIInstallCmd: `curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "/tmp/awscliv2.zip"
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/aws /tmp/awscliv2.zip`,
		AgentInstructions: `Use the AWS CLI for all AWS operations. Pre-authenticated via credential_process.

Common commands:
- S3: aws s3 ls, aws s3 cp <src> <dst>, aws s3 sync
- EC2: aws ec2 describe-instances, aws ec2 start-instances --instance-ids <id>
- Lambda: aws lambda list-functions, aws lambda invoke --function-name <name>
- IAM: aws iam list-users, aws iam list-roles
- CloudFormation: aws cloudformation list-stacks
- ECS: aws ecs list-clusters, aws ecs list-services

Run 'aws help' or 'aws <service> help' for detailed commands.`,
		Capabilities: []string{"s3", "ec2", "lambda", "iam", "rds", "dynamodb", "cloudformation", "ecs", "eks"},
		Enabled:      true,
	},

	"gcp": {
		ID:           "gcp",
		Name:         "Google Cloud",
		Description:  "Google Cloud Platform - cloud computing services",
		Category:     CategoryCloud,
		Icon:         "🔵",
		ProviderType: ProviderCloudCLI,
		AuthType:     AuthServiceAccount,
		CLICommand:   "gcloud",
		CLIInstallCmd: `curl -s https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-latest-linux-x86_64.tar.gz | tar -xz -C /opt
/opt/google-cloud-sdk/install.sh --quiet --path-update=true
echo 'source /opt/google-cloud-sdk/path.bash.inc' >> ~/.bashrc`,
		AgentInstructions: `Use gcloud CLI for GCP operations. Pre-authenticated via application default credentials.

Common commands:
- Compute: gcloud compute instances list, gcloud compute ssh <instance>
- GKE: gcloud container clusters list, gcloud container clusters get-credentials <name>
- Functions: gcloud functions list, gcloud functions deploy <name>
- Storage: gsutil ls, gsutil cp <src> <dst>
- BigQuery: bq ls, bq query "SELECT..."
- Pub/Sub: gcloud pubsub topics list

Run 'gcloud help' for all commands.`,
		Capabilities: []string{"compute", "gke", "functions", "storage", "bigquery", "pubsub", "iam"},
		Enabled:      true,
	},

	"azure": {
		ID:            "azure",
		Name:          "Microsoft Azure",
		Description:   "Microsoft Azure cloud platform - VMs, AKS, Functions, storage",
		Category:      CategoryCloud,
		Icon:          "🔷",
		ProviderType:  ProviderCloudCLI,
		AuthType:      AuthServiceAccount, // Service Principal
		CLICommand:    "az",
		CLIInstallCmd: `curl -sL https://aka.ms/InstallAzureCLIDeb | bash`,
		AgentInstructions: `Use Azure CLI (az) for Azure operations. Pre-authenticated via service principal.

Common commands:
- VMs: az vm list, az vm start --name <vm> --resource-group <rg>
- AKS: az aks list, az aks get-credentials --name <cluster> --resource-group <rg>
- Storage: az storage account list, az storage blob list --container-name <name>
- Functions: az functionapp list, az functionapp deployment source config-zip
- Resources: az resource list, az group list
- ACR: az acr list, az acr login --name <registry>

Run 'az help' or 'az <service> --help' for all commands.`,
		Capabilities: []string{"vms", "aks", "functions", "storage", "sql", "cosmos", "acr", "iam"},
		Enabled:      true,
	},

	"ibm_cloud": {
		ID:            "ibm_cloud",
		Name:          "IBM Cloud",
		Description:   "IBM Cloud platform - Watson, Kubernetes, Cloud Functions, databases",
		Category:      CategoryCloud,
		Icon:          "🔵",
		ProviderType:  ProviderCloudCLI,
		AuthType:      AuthAPIKey,
		CLICommand:    "ibmcloud",
		CLIInstallCmd: `curl -fsSL https://clis.cloud.ibm.com/install/linux | sh`,
		AgentInstructions: `Use IBM Cloud CLI for IBM Cloud operations. Pre-authenticated via IAM token.

Common commands:
- Account: ibmcloud account show, ibmcloud resource groups
- Kubernetes: ibmcloud ks cluster ls, ibmcloud ks cluster config --cluster <name>
- Functions: ibmcloud fn list, ibmcloud fn action invoke <name>
- Databases: ibmcloud cdb deployments, ibmcloud cdb deployment <id>
- Object Storage: ibmcloud cos bucket-list, ibmcloud cos object-list --bucket <name>
- Watson: ibmcloud watson list

Run 'ibmcloud help' for all commands.`,
		Capabilities: []string{"kubernetes", "functions", "watson", "databases", "storage", "vpc", "iam"},
		Enabled:      true,
	},

	"oracle_cloud": {
		ID:            "oracle_cloud",
		Name:          "Oracle Cloud",
		Description:   "Oracle Cloud Infrastructure - compute, OKE, autonomous database",
		Category:      CategoryCloud,
		Icon:          "🔴",
		ProviderType:  ProviderCloudCLI,
		AuthType:      AuthAPIKey, // API signing key
		CLICommand:    "oci",
		CLIInstallCmd: `pip3 install oci-cli`,
		AgentInstructions: `Use OCI CLI for Oracle Cloud operations. Pre-authenticated via session token.

Common commands:
- Compute: oci compute instance list --compartment-id <ocid>
- OKE: oci ce cluster list --compartment-id <ocid>
- Object Storage: oci os bucket list --compartment-id <ocid>
- Autonomous DB: oci db autonomous-database list --compartment-id <ocid>
- Functions: oci fn function list --application-id <ocid>
- Networking: oci network vcn list --compartment-id <ocid>

Note: Most commands require --compartment-id. Run 'oci --help' for details.`,
		Capabilities: []string{"compute", "oke", "storage", "database", "functions", "networking", "iam"},
		Enabled:      true,
	},

	"kubernetes": {
		ID:           "kubernetes",
		Name:         "Kubernetes",
		Description:  "Kubernetes cluster management - pods, deployments, services",
		Category:     CategoryCloud,
		Icon:         "☸️",
		ProviderType: ProviderCloudCLI,
		AuthType:     AuthToken, // Service account token or exec plugin
		CLICommand:   "kubectl",
		CLIInstallCmd: `curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && mv kubectl /usr/local/bin/`,
		AgentInstructions: `Use kubectl for Kubernetes cluster operations. Pre-authenticated via kubeconfig.

Common commands:
- Pods: kubectl get pods, kubectl describe pod <name>, kubectl logs <pod>
- Deployments: kubectl get deployments, kubectl rollout status deployment/<name>
- Services: kubectl get svc, kubectl expose deployment <name> --port=<port>
- ConfigMaps/Secrets: kubectl get configmaps, kubectl get secrets
- Namespaces: kubectl get ns, kubectl config set-context --current --namespace=<ns>
- Apply: kubectl apply -f <file.yaml>, kubectl delete -f <file.yaml>
- Exec: kubectl exec -it <pod> -- /bin/bash

Run 'kubectl help' or 'kubectl <command> --help' for details.`,
		Capabilities: []string{"pods", "deployments", "services", "configmaps", "secrets", "namespaces", "helm"},
		Enabled:      true,
	},

	// ============================================================================
	// PRODUCTIVITY - MCP Based (Pipedream has excellent support)
	// ============================================================================

	"gmail": {
		ID:           "gmail",
		Name:         "Gmail",
		Description:  "Email service with powerful search and organization",
		Category:     CategoryProductivity,
		Icon:         "📧",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "gmail",
		AgentInstructions: `Use MCP tools for Gmail:
- list_app_tools(app="gmail") to see available actions
- call_app_tool(app="gmail", tool="send-email", input={to, subject, body})

Common actions: send emails, search messages, manage labels, read threads.`,
		Capabilities: []string{"send_email", "read_email", "search", "labels", "drafts", "threads"},
		Enabled:      true,
	},

	"google_calendar": {
		ID:           "google_calendar",
		Name:         "Google Calendar",
		Description:  "Calendar and scheduling",
		Category:     CategoryProductivity,
		Icon:         "📅",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "google_calendar",
		AgentInstructions: `Use MCP tools for Google Calendar:
- list_app_tools(app="google_calendar") to see available actions

Common actions: create events, list events, update events, check availability.`,
		Capabilities: []string{"events", "calendars", "reminders", "availability"},
		Enabled:      true,
	},

	"google_drive": {
		ID:           "google_drive",
		Name:         "Google Drive",
		Description:  "Cloud storage and file collaboration",
		Category:     CategoryProductivity,
		Icon:         "📁",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "google_drive",
		AgentInstructions: `Use MCP tools for Google Drive:
- list_app_tools(app="google_drive") to see available actions

Common actions: upload files, list files, search, share files, create folders.`,
		Capabilities: []string{"files", "folders", "sharing", "search", "permissions"},
		Enabled:      true,
	},

	"notion": {
		ID:           "notion",
		Name:         "Notion",
		Description:  "All-in-one workspace for notes, docs, and wikis",
		Category:     CategoryProductivity,
		Icon:         "📝",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "notion",
		AgentInstructions: `Use MCP tools for Notion:
- list_app_tools(app="notion") to see available actions

Common actions: create pages, update pages, query databases, search.`,
		Capabilities: []string{"pages", "databases", "blocks", "search", "comments"},
		Enabled:      true,
	},

	"asana": {
		ID:           "asana",
		Name:         "Asana",
		Description:  "Project and task management",
		Category:     CategoryProductivity,
		Icon:         "🎯",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "asana",
		AgentInstructions: `Use MCP tools for Asana:
- list_app_tools(app="asana") to see available actions

Common actions: create tasks, update tasks, manage projects, assign work.`,
		Capabilities: []string{"tasks", "projects", "teams", "workspaces", "sections"},
		Enabled:      true,
	},

	"monday": {
		ID:           "monday",
		Name:         "Monday.com",
		Description:  "Work operating system for teams",
		Category:     CategoryProductivity,
		Icon:         "📊",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "monday",
		AgentInstructions: `Use MCP tools for Monday.com:
- list_app_tools(app="monday") to see available actions

Common actions: create items, update columns, manage boards, automate workflows.`,
		Capabilities: []string{"boards", "items", "columns", "groups", "updates"},
		Enabled:      true,
	},

	"trello": {
		ID:           "trello",
		Name:         "Trello",
		Description:  "Kanban-style project management",
		Category:     CategoryProductivity,
		Icon:         "📌",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "trello",
		AgentInstructions: `Use MCP tools for Trello:
- list_app_tools(app="trello") to see available actions

Common actions: create cards, move cards, manage lists, add comments.`,
		Capabilities: []string{"boards", "lists", "cards", "members", "labels"},
		Enabled:      true,
	},

	"airtable": {
		ID:           "airtable",
		Name:         "Airtable",
		Description:  "Spreadsheet-database hybrid for organizing work",
		Category:     CategoryProductivity,
		Icon:         "📊",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "airtable",
		AgentInstructions: `Use MCP tools for Airtable:
- list_app_tools(app="airtable") to see available actions

Common actions: create records, query tables, update fields, manage views.`,
		Capabilities: []string{"bases", "tables", "records", "views", "fields"},
		Enabled:      true,
	},

	// ============================================================================
	// COMMUNICATION - MCP Based
	// ============================================================================

	"slack": {
		ID:           "slack",
		Name:         "Slack",
		Description:  "Team messaging and collaboration hub",
		Category:     CategoryCommunication,
		Icon:         "💬",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "slack",
		AgentInstructions: `Use MCP tools for Slack:
- list_app_tools(app="slack") to see available actions

Common tools:
- send-message: Send to channel or user
- list-channels: Get channel list
- find-message: Search messages
- upload-file: Share files`,
		Capabilities: []string{"messages", "channels", "users", "files", "reactions", "threads"},
		Enabled:      true,
	},

	"discord": {
		ID:           "discord",
		Name:         "Discord",
		Description:  "Community chat and collaboration",
		Category:     CategoryCommunication,
		Icon:         "🎮",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "pipedream",
		MCPAppSlug:   "discord",
		AgentInstructions: `Use MCP tools for Discord:
- list_app_tools(app="discord") to see available actions

Common actions: send messages, manage channels, handle reactions.`,
		Capabilities: []string{"messages", "channels", "guilds", "members", "roles"},
		Enabled:      true,
	},

	"microsoft_teams": {
		ID:           "microsoft_teams",
		Name:         "Microsoft Teams",
		Description:  "Microsoft collaboration and meetings platform",
		Category:     CategoryCommunication,
		Icon:         "👥",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "microsoft-teams",
		AgentInstructions: `Use MCP tools for Microsoft Teams:
- list_app_tools(app="microsoft_teams") to see available actions

Common actions: send messages, create channels, manage teams.`,
		Capabilities: []string{"messages", "channels", "teams", "meetings", "calls"},
		Enabled:      true,
	},

	"outlook": {
		ID:           "outlook",
		Name:         "Outlook",
		Description:  "Microsoft email and calendar",
		Category:     CategoryCommunication,
		Icon:         "📬",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "outlook",
		AgentInstructions: `Use MCP tools for Outlook:
- list_app_tools(app="outlook") to see available actions

Common actions: send emails, manage calendar, search messages.`,
		Capabilities: []string{"email", "calendar", "contacts", "tasks"},
		Enabled:      true,
	},

	// ============================================================================
	// CRM & SALES - MCP Based
	// ============================================================================

	"hubspot": {
		ID:           "hubspot",
		Name:         "HubSpot",
		Description:  "CRM, marketing, and sales platform",
		Category:     CategoryProductivity,
		Icon:         "🧡",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "hubspot",
		AgentInstructions: `Use MCP tools for HubSpot:
- list_app_tools(app="hubspot") to see available actions

Common actions: manage contacts, deals, companies, tickets, marketing campaigns.`,
		Capabilities: []string{"contacts", "deals", "companies", "tickets", "marketing", "sales"},
		Enabled:      true,
	},

	// ============================================================================
	// FILE STORAGE - MCP Based
	// ============================================================================

	"onedrive": {
		ID:           "onedrive",
		Name:         "Microsoft OneDrive",
		Description:  "Cloud file storage and sync from Microsoft 365",
		Category:     CategoryProductivity,
		Icon:         "☁️",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "onedrive",
		AgentInstructions: `Use MCP tools for OneDrive:
- list_app_tools(app="onedrive") to see available actions

Common actions: upload files, download files, list files, search, share files, manage folders.
OneDrive integrates with Microsoft 365 ecosystem for seamless file access.`,
		Capabilities: []string{"files", "folders", "sharing", "search", "sync", "versions"},
		Enabled:      true,
	},

	"sharepoint": {
		ID:           "sharepoint",
		Name:         "Microsoft SharePoint",
		Description:  "Enterprise content management and collaboration platform",
		Category:     CategoryProductivity,
		Icon:         "📑",
		ProviderType: ProviderMCP,
		AuthType:     AuthOAuth2,
		MCPProvider:  "composio",
		MCPAppSlug:   "sharepoint",
		AgentInstructions: `Use MCP tools for SharePoint:
- list_app_tools(app="sharepoint") to see available actions

Common actions: manage sites, document libraries, lists, pages, and workflows.
SharePoint is Microsoft's enterprise collaboration platform.`,
		Capabilities: []string{"sites", "lists", "documents", "pages", "libraries", "workflows"},
		Enabled:      true,
	},

	// ============================================================================
	// DATABASES - Direct Connection
	// ============================================================================

	"postgres": {
		ID:           "postgres",
		Name:         "PostgreSQL",
		Description:  "Open-source relational database with SQL queries and analytics",
		Category:     CategoryData,
		Icon:         "🐘",
		ProviderType: ProviderCLI,
		AuthType:     AuthDatabase,
		CLICommand:   "psql",
		CLIInstallCmd: `apt-get update && apt-get install -y postgresql-client`,
		AgentInstructions: `Use the PostgreSQL CLI (psql) for database operations.
Pre-authenticated via environment variables: PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD.

Common commands:
- Connect: psql (uses env vars automatically)
- List databases: psql -c "\l"
- List tables: psql -c "\dt"
- Describe table: psql -c "\d tablename"
- Run query: psql -c "SELECT * FROM table LIMIT 10"
- Run SQL file: psql -f query.sql

For interactive SQL:
psql -c "your query here"

Or use heredoc for multi-line:
psql <<EOF
SELECT column1, column2
FROM table
WHERE condition
ORDER BY column1;
EOF

Run 'psql --help' for all options.`,
		Capabilities: []string{"sql", "tables", "schemas", "queries", "analytics"},
		Enabled:      true,
	},

	"mysql": {
		ID:           "mysql",
		Name:         "MySQL",
		Description:  "Popular open-source relational database",
		Category:     CategoryData,
		Icon:         "🐬",
		ProviderType: ProviderCLI,
		AuthType:     AuthDatabase,
		CLICommand:   "mysql",
		CLIInstallCmd: `apt-get update && apt-get install -y mysql-client`,
		AgentInstructions: `Use the MySQL CLI for database operations.
Pre-authenticated via environment variables: MYSQL_HOST, MYSQL_TCP_PORT, MYSQL_DATABASE, MYSQL_USER, MYSQL_PWD.

Common commands:
- Connect: mysql
- List databases: mysql -e "SHOW DATABASES"
- List tables: mysql -e "SHOW TABLES"
- Describe table: mysql -e "DESCRIBE tablename"
- Run query: mysql -e "SELECT * FROM table LIMIT 10"

Run 'mysql --help' for all options.`,
		Capabilities: []string{"sql", "tables", "schemas", "queries"},
		Enabled:      true,
	},

	"bigquery": {
		ID:           "bigquery",
		Name:         "BigQuery",
		Description:  "Google Cloud serverless data warehouse",
		Category:     CategoryData,
		Icon:         "📊",
		ProviderType: ProviderCLI,
		AuthType:     AuthServiceAccount,
		CLICommand:   "bq",
		CLIInstallCmd: `# BigQuery CLI is part of gcloud SDK`,
		AgentInstructions: `Use the BigQuery CLI (bq) for data warehouse operations.
Pre-authenticated via GCP service account.

Common commands:
- List datasets: bq ls
- List tables: bq ls dataset_name
- Show table schema: bq show dataset.table
- Run query: bq query "SELECT * FROM dataset.table LIMIT 10"
- Run query with standard SQL: bq query --use_legacy_sql=false "SELECT..."

Run 'bq --help' for all commands.`,
		Capabilities: []string{"sql", "datasets", "tables", "queries", "analytics"},
		Enabled:      true,
	},

	"sqlserver": {
		ID:           "sqlserver",
		Name:         "Microsoft SQL Server",
		Description:  "Enterprise relational database from Microsoft",
		Category:     CategoryData,
		Icon:         "🔷",
		ProviderType: ProviderCLI,
		AuthType:     AuthDatabase,
		CLICommand:   "sqlcmd",
		CLIInstallCmd: `curl https://packages.microsoft.com/keys/microsoft.asc | apt-key add -
curl https://packages.microsoft.com/config/ubuntu/22.04/prod.list | tee /etc/apt/sources.list.d/msprod.list
apt-get update && ACCEPT_EULA=Y apt-get install -y mssql-tools18 unixodbc-dev`,
		AgentInstructions: `Use sqlcmd for SQL Server operations.
Pre-authenticated via environment variables: SQLCMDSERVER, SQLCMDDBNAME, SQLCMDUSER, SQLCMDPASSWORD.

Common commands:
- Run query: sqlcmd -Q "SELECT * FROM table"
- List databases: sqlcmd -Q "SELECT name FROM sys.databases"
- List tables: sqlcmd -Q "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES"

Run 'sqlcmd -?' for all options.`,
		Capabilities: []string{"sql", "tables", "schemas", "queries"},
		Enabled:      true,
	},

	"vertica": {
		ID:           "vertica",
		Name:         "Vertica",
		Description:  "Columnar analytics database for big data",
		Category:     CategoryData,
		Icon:         "📈",
		ProviderType: ProviderCLI,
		AuthType:     AuthDatabase,
		CLICommand:   "vsql",
		CLIInstallCmd: `# Vertica client requires manual installation`,
		AgentInstructions: `Use vsql for Vertica analytics operations.
Pre-authenticated via environment variables: VSQL_HOST, VSQL_PORT, VSQL_DATABASE, VSQL_USER, VSQL_PASSWORD.

Common commands:
- Connect: vsql
- List schemas: vsql -c "\dn"
- List tables: vsql -c "\dt"
- Run query: vsql -c "SELECT * FROM table LIMIT 10"

Run 'vsql --help' for all options.`,
		Capabilities: []string{"sql", "analytics", "schemas", "projections"},
		Enabled:      true,
	},

	// ============================================================================
	// DATA WAREHOUSES - CLI/MCP Based
	// ============================================================================

	"snowflake": {
		ID:            "snowflake",
		Name:          "Snowflake",
		Description:   "Cloud data warehouse with SQL analytics and data sharing",
		Category:      CategoryData,
		Icon:          "❄️",
		ProviderType:  ProviderCLI,
		AuthType:      AuthToken, // Uses key pair or OAuth
		CLICommand:    "snow",
		CLIInstallCmd: `pip3 install snowflake-cli-labs`,
		CLIAuthCmd:    "", // Configured via connection.toml or env vars
		AgentInstructions: `Use the Snowflake CLI (snow) for data warehouse operations.
Pre-authenticated via SNOWFLAKE_ACCOUNT, SNOWFLAKE_USER, SNOWFLAKE_PASSWORD environment variables.

Common commands:
- Connections: snow connection test, snow connection list
- SQL: snow sql -q "SELECT * FROM table LIMIT 10"
- Databases: snow sql -q "SHOW DATABASES", snow sql -q "USE DATABASE mydb"
- Schemas: snow sql -q "SHOW SCHEMAS", snow sql -q "SHOW TABLES"
- Warehouses: snow sql -q "SHOW WAREHOUSES"
- Cortex: snow cortex search, snow cortex complete

For interactive SQL:
snow sql -q "your query here"

For multi-line queries, use a file:
snow sql -f query.sql

Run 'snow --help' for all commands.`,
		Capabilities: []string{"sql", "databases", "warehouses", "stages", "tasks", "cortex"},
		Enabled:      true,
	},

	"databricks": {
		ID:            "databricks",
		Name:          "Databricks",
		Description:   "Unified analytics platform for data engineering and ML",
		Category:      CategoryData,
		Icon:          "🧱",
		ProviderType:  ProviderCLI,
		AuthType:      AuthToken,
		CLICommand:    "databricks",
		CLIInstallCmd: `pip3 install databricks-cli`,
		CLIAuthCmd:    "", // Uses DATABRICKS_HOST and DATABRICKS_TOKEN env vars
		AgentInstructions: `Use the Databricks CLI for data platform operations.
Pre-authenticated via DATABRICKS_HOST and DATABRICKS_TOKEN environment variables.

Common commands:
- Clusters: databricks clusters list, databricks clusters get --cluster-id <id>
- Jobs: databricks jobs list, databricks jobs run-now --job-id <id>
- Notebooks: databricks workspace ls, databricks workspace export <path>
- DBFS: databricks fs ls dbfs:/, databricks fs cp <src> <dst>
- SQL: databricks sql execute --query "SELECT..."
- Unity Catalog: databricks unity-catalog catalogs list

Run 'databricks --help' for all commands.`,
		Capabilities: []string{"clusters", "jobs", "notebooks", "dbfs", "sql", "mlflow", "unity_catalog"},
		Enabled:      true,
	},

	// ============================================================================
	// MONITORING & OBSERVABILITY - API Based (No good MCP/CLI support)
	// ============================================================================

	"datadog": {
		ID:           "datadog",
		Name:         "Datadog",
		Description:  "Infrastructure monitoring and APM",
		Category:     CategoryMonitoring,
		Icon:         "🐕",
		ProviderType: ProviderAPI,
		AuthType:     AuthAPIKey,
		APIBaseURL:   "https://api.datadoghq.com",
		APIDocsURL:   "https://docs.datadoghq.com/api/latest/",
		AgentInstructions: `For Datadog, use curl with the pre-configured API keys.
Environment variables set: DATADOG_API_KEY, DATADOG_APP_KEY, DATADOG_SITE

Common API calls:
- List dashboards: curl -H "DD-API-KEY: $DATADOG_API_KEY" -H "DD-APPLICATION-KEY: $DATADOG_APP_KEY" "$DATADOG_SITE/api/v1/dashboard"
- Get monitors: curl -H "DD-API-KEY: $DATADOG_API_KEY" -H "DD-APPLICATION-KEY: $DATADOG_APP_KEY" "$DATADOG_SITE/api/v1/monitor"
- Query metrics: curl -H "DD-API-KEY: $DATADOG_API_KEY" "$DATADOG_SITE/api/v1/query?query=..."
- List hosts: curl -H "DD-API-KEY: $DATADOG_API_KEY" -H "DD-APPLICATION-KEY: $DATADOG_APP_KEY" "$DATADOG_SITE/api/v1/hosts"

Refer to Datadog API docs for full endpoint list.`,
		Capabilities: []string{"dashboards", "monitors", "metrics", "events", "logs", "hosts", "apm"},
		Enabled:      true,
	},

	"newrelic": {
		ID:           "newrelic",
		Name:         "New Relic",
		Description:  "Full-stack observability platform",
		Category:     CategoryMonitoring,
		Icon:         "📈",
		ProviderType: ProviderAPI,
		AuthType:     AuthAPIKey,
		APIBaseURL:   "https://api.newrelic.com",
		APIDocsURL:   "https://docs.newrelic.com/docs/apis/",
		AgentInstructions: `For New Relic, use curl with the pre-configured API key.
Environment variable set: NEW_RELIC_API_KEY, NEW_RELIC_ACCOUNT_ID

Common API calls:
- List applications: curl -H "Api-Key: $NEW_RELIC_API_KEY" "https://api.newrelic.com/v2/applications.json"
- NRQL query: curl -H "Api-Key: $NEW_RELIC_API_KEY" "https://api.newrelic.com/graphql" -d '{"query":"..."}'
- List alerts: curl -H "Api-Key: $NEW_RELIC_API_KEY" "https://api.newrelic.com/v2/alerts_policies.json"

New Relic primarily uses GraphQL/NRQL for queries. Check docs for NerdGraph API.`,
		Capabilities: []string{"applications", "alerts", "dashboards", "nrql", "apm", "infrastructure"},
		Enabled:      true,
	},

	"pagerduty": {
		ID:           "pagerduty",
		Name:         "PagerDuty",
		Description:  "Incident management and on-call scheduling",
		Category:     CategoryMonitoring,
		Icon:         "🚨",
		ProviderType: ProviderAPI,
		AuthType:     AuthAPIKey,
		APIBaseURL:   "https://api.pagerduty.com",
		APIDocsURL:   "https://developer.pagerduty.com/api-reference/",
		AgentInstructions: `For PagerDuty, use curl with the pre-configured API key.
Environment variable set: PAGERDUTY_API_KEY

Common API calls:
- List incidents: curl -H "Authorization: Token token=$PAGERDUTY_API_KEY" "https://api.pagerduty.com/incidents"
- List services: curl -H "Authorization: Token token=$PAGERDUTY_API_KEY" "https://api.pagerduty.com/services"
- List on-calls: curl -H "Authorization: Token token=$PAGERDUTY_API_KEY" "https://api.pagerduty.com/oncalls"
- Create incident: curl -X POST -H "Authorization: Token token=$PAGERDUTY_API_KEY" -H "Content-Type: application/json" "https://api.pagerduty.com/incidents" -d '...'

Refer to PagerDuty API docs for full endpoint list.`,
		Capabilities: []string{"incidents", "services", "schedules", "users", "escalation_policies"},
		Enabled:      true,
	},

	// ============================================================================
	// OTHER TOOLS
	// ============================================================================

	"fireflies": {
		ID:           "fireflies",
		Name:         "Fireflies",
		Description:  "AI meeting transcription and notes",
		Category:     CategoryProductivity,
		Icon:         "🔥",
		ProviderType: ProviderAPI,
		AuthType:     AuthAPIKey,
		APIBaseURL:   "https://api.fireflies.ai/graphql",
		APIDocsURL:   "https://docs.fireflies.ai/",
		AgentInstructions: `For Fireflies, use their GraphQL API.
Environment variable set: FIREFLIES_API_KEY

The API is GraphQL-based. Example:
curl -X POST "https://api.fireflies.ai/graphql" \
  -H "Authorization: Bearer $FIREFLIES_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ transcripts { id title } }"}'

Refer to Fireflies API docs for available queries.`,
		Capabilities: []string{"transcripts", "meetings", "search", "summaries"},
		Enabled:      true,
	},

	"canva": {
		ID:           "canva",
		Name:         "Canva",
		Description:  "Design and visual content creation",
		Category:     CategoryProductivity,
		Icon:         "🎨",
		ProviderType: ProviderAPI,
		AuthType:     AuthOAuth2,
		APIBaseURL:   "https://api.canva.com/v1",
		APIDocsURL:   "https://www.canva.dev/docs/connect/",
		OAuth2Config: &OAuth2Config{
			AuthURL:  "https://www.canva.com/api/oauth/authorize",
			TokenURL: "https://api.canva.com/rest/v1/oauth/token",
			Scopes:   []string{"design:content:read", "design:content:write"},
		},
		AgentInstructions: `For Canva, use their REST API.
The access token is available in the environment.

Common API calls:
- List designs: curl -H "Authorization: Bearer $CANVA_ACCESS_TOKEN" "https://api.canva.com/rest/v1/designs"
- Get design: curl -H "Authorization: Bearer $CANVA_ACCESS_TOKEN" "https://api.canva.com/rest/v1/designs/{design_id}"

Note: Canva API has limited functionality. Check docs for available endpoints.`,
		Capabilities: []string{"designs", "folders", "exports"},
		Enabled:      true,
		Beta:         true,
	},
}

// GetIntegration returns an integration by ID
func GetIntegration(id string) (*Integration, bool) {
	i, ok := Catalog[id]
	return i, ok
}

// GetEnabledIntegrations returns all enabled integrations
func GetEnabledIntegrations() []*Integration {
	var result []*Integration
	for _, i := range Catalog {
		if i.Enabled {
			result = append(result, i)
		}
	}
	return result
}

// GetIntegrationsByCategory returns integrations for a category
func GetIntegrationsByCategory(cat Category) []*Integration {
	var result []*Integration
	for _, i := range Catalog {
		if i.Category == cat && i.Enabled {
			result = append(result, i)
		}
	}
	return result
}

// GetIntegrationsByProviderType returns integrations by provider type
func GetIntegrationsByProviderType(pt ProviderType) []*Integration {
	var result []*Integration
	for _, i := range Catalog {
		if i.ProviderType == pt && i.Enabled {
			result = append(result, i)
		}
	}
	return result
}

// GetMCPIntegrations returns all MCP-based integrations grouped by provider
func GetMCPIntegrations() map[string][]*Integration {
	result := map[string][]*Integration{
		"pipedream": {},
		"composio":  {},
		"direct":    {},
	}

	for _, i := range Catalog {
		if !i.Enabled {
			continue
		}

		switch i.ProviderType {
		case ProviderMCP:
			if i.MCPProvider == "pipedream" {
				result["pipedream"] = append(result["pipedream"], i)
			} else if i.MCPProvider == "composio" {
				result["composio"] = append(result["composio"], i)
			}
		case ProviderDirectMCP:
			result["direct"] = append(result["direct"], i)
		}
	}

	return result
}

// GetCLIIntegrations returns all CLI-based integrations
func GetCLIIntegrations() []*Integration {
	var result []*Integration
	for _, i := range Catalog {
		if i.Enabled && (i.ProviderType == ProviderCLI || i.ProviderType == ProviderCloudCLI) {
			result = append(result, i)
		}
	}
	return result
}

// GetAPIIntegrations returns all API-based integrations
func GetAPIIntegrations() []*Integration {
	var result []*Integration
	for _, i := range Catalog {
		if i.Enabled && i.ProviderType == ProviderAPI {
			result = append(result, i)
		}
	}
	return result
}
