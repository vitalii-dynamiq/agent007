#!/usr/bin/env python3
"""
Dynamiq Agent - AI Agent with E2B Sandbox

Uses OpenAI for LLM and E2B for isolated code execution.
MCP tools are accessed via CLI commands in the sandbox calling our backend proxy.
"""
import os
import json
import asyncio
import base64
import urllib.request
from pathlib import Path
from typing import Any, Callable
from dotenv import load_dotenv
from openai import OpenAI
from e2b import Sandbox

# Load .env from project root (parent of agent/ directory)
env_path = Path(__file__).parent.parent / ".env"
load_dotenv(env_path)

# Configuration
OPENAI_API_KEY = os.getenv("LLM_API_KEY") or os.getenv("OPENAI_API_KEY")
E2B_API_KEY = os.getenv("E2B_API_KEY")
MODEL = os.getenv("LLM_MODEL", "gpt-5.2")

# MCP Proxy URL - must be accessible from E2B sandbox (use ngrok for dev)
BACKEND_URL = os.getenv("BACKEND_URL", "")
DEFAULT_MCP_PROXY_URL = os.getenv("MCP_PROXY_URL") or (
    f"{BACKEND_URL}/api/mcp/proxy" if BACKEND_URL else ""
)

AWS_CLI_INSTALL_CMD = """test -x /home/user/.local/bin/aws || (
  mkdir -p /home/user/.local/bin /home/user/.local/aws-cli /home/user/.local/tmp &&
  curl -s "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "/home/user/.local/tmp/awscliv2.zip" &&
  python3 - <<'PY'
import zipfile
zipfile.ZipFile("/home/user/.local/tmp/awscliv2.zip").extractall("/home/user/.local/tmp")
PY
  chmod +x /home/user/.local/tmp/aws/install &&
  /home/user/.local/tmp/aws/install -i /home/user/.local/aws-cli -b /home/user/.local/bin &&
  chmod +x /home/user/.local/aws-cli/v2/current/dist/aws &&
  rm -rf /home/user/.local/tmp/aws /home/user/.local/tmp/awscliv2.zip
)"""

GH_CLI_INSTALL_CMD = """test -x /home/user/.local/bin/gh-real || (
  mkdir -p /home/user/.local/bin /home/user/.local/tmp &&
  python3 - <<'PY'
import io
import json
import os
import tarfile
import urllib.request

api = "https://api.github.com/repos/cli/cli/releases/latest"
data = json.loads(urllib.request.urlopen(api).read())
asset = next(a for a in data["assets"] if a["name"].endswith("linux_amd64.tar.gz"))
url = asset["browser_download_url"]
tar_data = urllib.request.urlopen(url).read()
target = "/home/user/.local/tmp/gh"
os.makedirs(target, exist_ok=True)
tarfile.open(fileobj=io.BytesIO(tar_data), mode="r:gz").extractall(target)
folder = next(d for d in os.listdir(target) if d.startswith("gh_") and d.endswith("_linux_amd64"))
bin_path = os.path.join(target, folder, "bin", "gh")
os.makedirs("/home/user/.local/bin", exist_ok=True)
os.rename(bin_path, "/home/user/.local/bin/gh-real")
os.chmod("/home/user/.local/bin/gh-real", 0o755)
PY
  rm -rf /home/user/.local/tmp/gh
)"""

def build_github_token_helper(backend_url: str, session_token: str) -> str:
    return f"""#!/usr/bin/env python3
import json
import sys
import urllib.request

BACKEND_URL = "{backend_url}"
SESSION_TOKEN = "{session_token}"

payload = json.dumps({{"provider": "github"}}).encode()
req = urllib.request.Request(
    f"{{BACKEND_URL}}/api/github/token",
    data=payload,
    headers={{
        "Content-Type": "application/json",
        "Authorization": "Bearer " + SESSION_TOKEN,
    }},
)

try:
    with urllib.request.urlopen(req, timeout=20) as resp:
        body = json.loads(resp.read())
except Exception as e:
    print(f"Error: {{e}}", file=sys.stderr)
    sys.exit(1)

token = body.get("token", "")
if not token:
    print("Error: Missing GitHub token", file=sys.stderr)
    sys.exit(1)

print(token)
"""

GH_WRAPPER_SCRIPT = """#!/usr/bin/env bash
set -e
TOKEN=$(/home/user/.local/bin/gh-token)
if [ -z "$TOKEN" ]; then
  echo "Error: Missing GitHub token" >&2
  exit 1
fi
export GH_TOKEN="$TOKEN"
exec /home/user/.local/bin/gh-real "$@"
"""

# PostgreSQL CLI installation command
PSQL_CLI_INSTALL_CMD = """apt-get update && apt-get install -y postgresql-client 2>/dev/null || true"""


def derive_backend_url(mcp_proxy_url: str) -> str:
    if BACKEND_URL:
        return BACKEND_URL
    if mcp_proxy_url and mcp_proxy_url.endswith("/api/mcp/proxy"):
        return mcp_proxy_url[: -len("/api/mcp/proxy")]
    return ""


SYSTEM_PROMPT = """You are Dynamiq, a powerful AI assistant with access to an isolated Linux sandbox environment.

## Your Capabilities

You have access to a sandbox where you can:
- Execute any shell commands
- Write and read files
- Install packages (apt, pip, npm, etc.)
- Run scripts in any language (Python, Node.js, Bash, etc.)

## Data Visualization with Vega-Lite

When the user asks for data analysis, charts, or visualizations, you should generate **Vega-Lite** specifications. The chat interface will automatically render these as interactive charts.

### How to Create Charts

Output Vega-Lite JSON specifications inside a code block with the `vega-lite` or `vega` language tag:

```vega-lite
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": { "values": [...] },
  "mark": "bar",
  "encoding": {...}
}
```

### Common Chart Examples

**Bar Chart:**
```vega-lite
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {
    "values": [
      {"category": "A", "value": 28},
      {"category": "B", "value": 55},
      {"category": "C", "value": 43}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "category", "type": "nominal", "title": "Category"},
    "y": {"field": "value", "type": "quantitative", "title": "Value"},
    "color": {"field": "category", "type": "nominal", "legend": null}
  }
}
```

**Line Chart (Time Series):**
```vega-lite
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {
    "values": [
      {"date": "2024-01", "value": 100},
      {"date": "2024-02", "value": 150},
      {"date": "2024-03", "value": 120}
    ]
  },
  "mark": {"type": "line", "point": true},
  "encoding": {
    "x": {"field": "date", "type": "temporal", "title": "Date"},
    "y": {"field": "value", "type": "quantitative", "title": "Value"}
  }
}
```

**Pie/Donut Chart:**
```vega-lite
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {
    "values": [
      {"category": "Desktop", "value": 60},
      {"category": "Mobile", "value": 35},
      {"category": "Tablet", "value": 5}
    ]
  },
  "mark": {"type": "arc", "innerRadius": 50},
  "encoding": {
    "theta": {"field": "value", "type": "quantitative"},
    "color": {"field": "category", "type": "nominal", "title": "Device"}
  }
}
```

**Scatter Plot:**
```vega-lite
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {
    "values": [
      {"x": 1, "y": 2, "category": "A"},
      {"x": 3, "y": 4, "category": "B"}
    ]
  },
  "mark": "point",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "color": {"field": "category", "type": "nominal"}
  }
}
```

### Visualization Guidelines

1. **Always use Vega-Lite** for data visualizations - it will be rendered interactively
2. **Include the $schema** for proper rendering
3. **Embed data directly** in the spec using `data.values` array
4. **Use appropriate chart types**:
   - Bar charts for comparisons
   - Line charts for trends over time
   - Pie/donut for proportions
   - Scatter plots for correlations
   - Area charts for cumulative values
5. **Add meaningful titles** to axes and legends
6. **When analyzing data from APIs or files**, extract the relevant data and create a Vega-Lite visualization
7. Charts support interactivity: tooltips show on hover, users can zoom/pan

## GitHub CLI (gh)

GitHub is accessed via the **`gh` CLI** (NOT via MCP). The CLI is pre-authenticated with the user's OAuth token.

**Common commands:**
```bash
# Check auth status
gh auth status

# List repositories
gh repo list

# View a specific repo
gh repo view owner/repo

# Clone a repo
gh repo clone owner/repo

# List issues
gh issue list --repo owner/repo

# Create an issue
gh issue create --repo owner/repo --title "Title" --body "Description"

# List pull requests
gh pr list --repo owner/repo

# View PR details
gh pr view 123 --repo owner/repo

# List workflows/actions
gh run list --repo owner/repo

# View gists
gh gist list
```

If `gh` commands fail with auth errors, ask the user to connect GitHub in the Integrations panel.

Run `gh help` or `gh <command> --help` for more options.

## MCP Tools (Other External Integrations)

The sandbox has an `mcp` CLI for accessing connected services like Gmail, Slack, Notion, and 2000+ other apps (but NOT GitHub):

```bash
# List connected apps (always run this first!)
mcp list-apps

# List available tools for an app (IMPORTANT: check the schema before calling!)
mcp list-tools <app>

# Call a tool with JSON input matching the tool's schema
mcp call <app> <tool> '<json-input>'
```

### Important: Check Tool Schemas First

Each tool has its own parameter schema. Always run `mcp list-tools <app>` to see:
- Available tool names
- Required and optional parameters
- Parameter types and descriptions

### Examples

**Gmail:**
```bash
mcp list-tools gmail
# Check the output for exact parameter names, then call with proper params:
mcp call gmail gmail-find-email '{"q": "newer_than:1d", "maxResults": 10}'
mcp call gmail gmail-send-email '{"to": "user@example.com", "subject": "Hello", "body": "Hi there!"}'
```

**Slack:**
```bash
mcp list-tools slack
mcp call slack slack-send-message '{"channel": "#general", "text": "Hello from Dynamiq!"}'
```

**Google Sheets:**
```bash
mcp list-tools google_sheets
mcp call google_sheets google_sheets-add-single-row '{"sheetId": "...", "worksheetId": "...", "hasHeaders": true, "row": ["value1", "value2"]}'
```

## AWS / Cloud CLIs

AWS is **not** an MCP app. Do **not** use `mcp list-apps` to check AWS connectivity.
Use the AWS CLI directly (credentials are injected via `credential_process`):

```bash
aws --version
aws sts get-caller-identity
aws configure list
```

If AWS commands fail with credential errors, ask the user to connect AWS in the Integrations panel.

## PostgreSQL Database

PostgreSQL is accessed via the **`psql` CLI** (NOT via MCP). Credentials are pre-configured via environment variables.

**Common commands:**
```bash
# Check connection (uses PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD env vars)
psql -c "SELECT version();"

# List databases
psql -c "\\l"

# List tables in current database
psql -c "\\dt"

# List tables in a specific schema
psql -c "\\dt sales.*"

# Describe a table
psql -c "\\d tablename"

# Run a query
psql -c "SELECT * FROM table LIMIT 10"

# Run complex queries with heredoc
psql <<EOF
SELECT 
    column1, 
    column2,
    COUNT(*) as count
FROM schema.table
WHERE condition
GROUP BY column1, column2
ORDER BY count DESC
LIMIT 20;
EOF
```

**Discovering the database schema:**

When connected to a PostgreSQL database, always discover what's available first:
```bash
# List all schemas
psql -c "\\dn"

# List all tables in all schemas
psql -c "\\dt *.*"

# Describe a specific table
psql -c "\\d schema.tablename"

# Get column info
psql -c "SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = 'public' ORDER BY table_name, ordinal_position;"
```

**IMPORTANT:** Do NOT assume what tables exist. Always run discovery commands first to see the actual database schema before running queries.

If psql commands fail with "command not found", ask the user to connect PostgreSQL in the Integrations panel first.

## Workflow

1. For **GitHub**: Use `gh` CLI directly (e.g., `gh repo list`, `gh issue list`)
2. For **AWS/GCP/Azure**: Use cloud CLIs directly (`aws`, `gcloud`, `az`)
3. For **other services** (Gmail, Slack, Notion, etc.): Use `mcp list-apps` first, then `mcp list-tools <app>` and `mcp call`
4. If a service isn't connected, tell the user to connect it via the Integrations panel

## File Uploads

Users can upload files to the sandbox. Uploaded files are stored in `/home/user/uploads/`.

**CRITICAL:** When a user mentions an uploaded file, attachment, or asks about a file they uploaded:
1. ALWAYS run `ls -la /home/user/uploads/` FIRST to check what files are available
2. Do NOT rely on previous conversation history about uploads - always check the current state
3. For PDFs, use `pdftotext` or Python libraries to extract text
4. For CSV/JSON/Excel files, use Python pandas to read and analyze

Example workflow for uploaded files:
```bash
# Always check uploads first
ls -la /home/user/uploads/

# For PDF files - install pdftotext if needed
apt-get update && apt-get install -y poppler-utils
pdftotext /home/user/uploads/document.pdf -

# Or use Python
python3 -c "import subprocess; print(subprocess.run(['pdftotext', '/home/user/uploads/document.pdf', '-'], capture_output=True, text=True).stdout)"
```

## Returning Files to User

When you generate a file that the user should download or view, use the `return_file` tool:

```
Use return_file with:
- path: The file path in the sandbox (e.g., /home/user/report.csv)
- description: Brief description of what the file contains
```

**Images will be displayed inline** in the chat. Use this for:
- Charts generated with matplotlib, seaborn, or plotly
- Diagrams generated with graphviz
- Any PNG, JPG, SVG, or other image files

**Example: Generate and return a chart:**
```python
import matplotlib.pyplot as plt
import pandas as pd

# Create chart
plt.figure(figsize=(10, 6))
plt.bar(['A', 'B', 'C'], [10, 20, 15])
plt.title('Sample Chart')
plt.savefig('/home/user/chart.png', dpi=150, bbox_inches='tight')
plt.close()
```
Then call: `return_file(path="/home/user/chart.png", description="Sample bar chart")`

**PDFs can be opened in browser.** Use for reports:
```python
from reportlab.lib.pagesizes import letter
from reportlab.pdfgen import canvas

c = canvas.Canvas("/home/user/report.pdf", pagesize=letter)
c.drawString(100, 750, "Report Title")
c.save()
```

**Available visualization tools:**
- `matplotlib` / `seaborn` - Static charts and plots
- `plotly` + `kaleido` - Interactive charts (export to PNG/SVG)
- `graphviz` - Diagrams and flowcharts (use `dot` command)
- `pillow` - Image manipulation
- `reportlab` - PDF generation
- `imagemagick` - Image conversion (`convert` command)

## Mermaid Diagrams

You can create mermaid diagrams directly in your response using markdown code blocks. The UI will render them automatically.

**CRITICAL SYNTAX RULES - MUST FOLLOW:**
1. **NEVER use pipe characters (|) inside node labels** - they break parsing
2. **NEVER use angle brackets (<>) inside labels** - use ‹› or omit them
3. **Use · (middle dot) or - (dash) as separators** instead of |
4. **Use \\n for line breaks** within labels
5. **Keep labels simple** - avoid special characters

**CORRECT Examples:**

```mermaid
flowchart TB
    A[EKS Cluster · us-east-1]
    B[Node Group: system\\n2 instances]
    C[Service: API\\nPort 8080]
    A --> B
    B --> C
```

```mermaid
flowchart LR
    subgraph VPC[VPC - Production]
        S1[Subnet 1\\n10.0.1.0/24]
        S2[Subnet 2\\n10.0.2.0/24]
    end
    IGW[Internet Gateway] --> VPC
```

**WRONG - These will FAIL:**
```
A[EKS | K8s 1.29 | ACTIVE]    ❌ pipes break parsing
B[Status: <running>]           ❌ angle brackets break parsing  
C[Node|Group]                  ❌ any pipe in label fails
```

**For complex infrastructure diagrams**, prefer generating an image with Python/matplotlib/graphviz instead of mermaid, then use `return_file` to send it back.

**Supported diagram types:**
- `flowchart` / `graph` - Flowcharts (use TB, LR, RL, BT for direction)
- `sequenceDiagram` - Sequence diagrams
- `classDiagram` - Class diagrams
- `erDiagram` - Entity relationship diagrams
- `pie` - Pie charts

## Important Notes

- **GitHub uses `gh` CLI**, not MCP
- **Always check tool schemas** with `mcp list-tools <app>` before calling - each tool has specific parameters
- Always check connected apps with `mcp list-apps` before trying to use them
- **Always check `/home/user/uploads/`** when user mentions uploaded files - do not assume it's empty
- **Use `return_file`** to send generated files to the user for download
- The sandbox is isolated - changes don't affect the user's system
- Be efficient: combine related commands when possible
- Show clear, formatted results to the user
- If something fails, explain the error and suggest solutions
"""


# Tool definitions for OpenAI function calling
TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "execute_command",
            "description": "Execute a shell command in the sandbox. Use for: running mcp commands, installing packages, running scripts, file operations, etc.",
            "parameters": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "The shell command to execute"
                    },
                    "cwd": {
                        "type": "string",
                        "description": "Working directory (default: /home/user)"
                    }
                },
                "required": ["command"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "write_file",
            "description": "Write content to a file in the sandbox.",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "File path (e.g., /home/user/script.py)"
                    },
                    "content": {
                        "type": "string",
                        "description": "File content"
                    }
                },
                "required": ["path", "content"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "read_file",
            "description": "Read content from a file in the sandbox.",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "File path to read"
                    }
                },
                "required": ["path"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "return_file",
            "description": "Return a file to the user for download. Use this when the user needs to download a generated file (CSV, Excel, image, PDF, etc.).",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file in the sandbox to return to the user"
                    },
                    "description": {
                        "type": "string",
                        "description": "Brief description of what the file contains"
                    }
                },
                "required": ["path"]
            }
        }
    }
]


# MCP CLI script that gets uploaded to the sandbox
# Using raw string and avoiding f-strings to prevent any escaping issues
MCP_CLI_SCRIPT = r'''#!/usr/bin/env python3
import sys
import json
import os
import urllib.request
import urllib.error

PROXY_URL = os.environ.get("MCP_PROXY_URL", "")
TOKEN = os.environ.get("MCP_SESSION_TOKEN", "")
USER_ID = os.environ.get("MCP_USER_ID", "")

def request(method, **kwargs):
    if not PROXY_URL:
        print("Error: MCP_PROXY_URL not configured", file=sys.stderr)
        sys.exit(1)
    
    data = json.dumps({"method": method, **kwargs}).encode()
    headers = {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + TOKEN,
        "X-User-ID": USER_ID,
        "ngrok-skip-browser-warning": "true",
    }
    req = urllib.request.Request(PROXY_URL, data=data, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            result = json.loads(resp.read())
            if result.get("success"):
                return result.get("data")
            else:
                print("Error: " + str(result.get("error", "Unknown error")), file=sys.stderr)
                sys.exit(1)
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else str(e)
        print("HTTP Error " + str(e.code) + ": " + body, file=sys.stderr)
        sys.exit(1)
    except urllib.error.URLError as e:
        print("Connection Error: " + str(e.reason), file=sys.stderr)
        sys.exit(1)

def main():
    if len(sys.argv) < 2:
        print("MCP CLI - Access connected apps")
        print("Usage: mcp <command> [args]")
        print("Commands: list-apps, list-tools <app>, call <app> <tool> <json>")
        sys.exit(0)
    
    cmd = sys.argv[1]
    
    if cmd in ["list-apps", "apps"]:
        apps = request("list_apps")
        if not apps:
            print("No connected apps. Connect apps via the Integrations panel.")
        else:
            print("Connected apps:")
            for app in apps:
                # Show app slug (e.g., "gmail") as the primary identifier
                app_slug = app.get("app") or app.get("App") or "unknown"
                account_name = app.get("name") or app.get("Name") or ""
                if account_name:
                    print("  - " + str(app_slug) + " (" + str(account_name) + ")")
                else:
                    print("  - " + str(app_slug))
    
    elif cmd in ["list-tools", "tools"]:
        if len(sys.argv) < 3:
            print("Usage: mcp list-tools <app>", file=sys.stderr)
            sys.exit(1)
        tools = request("list_tools", app=sys.argv[2])
        if not tools:
            print("No tools found for " + sys.argv[2])
        else:
            print("Tools for " + sys.argv[2] + ":")
            for tool in tools:
                name = tool.get("name", "unknown")
                desc = tool.get("description", "")[:60]
                print("  - " + name + (": " + desc if desc else ""))
    
    elif cmd == "call":
        if len(sys.argv) < 5:
            print("Usage: mcp call <app> <tool> <json-input>", file=sys.stderr)
            sys.exit(1)
        app, tool, input_json = sys.argv[2], sys.argv[3], sys.argv[4]
        try:
            input_data = json.loads(input_json)
        except json.JSONDecodeError as e:
            print("Invalid JSON: " + str(e), file=sys.stderr)
            sys.exit(1)
        result = request("call_tool", app=app, tool=tool, input=input_data)
        if isinstance(result, (dict, list)):
            print(json.dumps(result, indent=2))
        else:
            print(result)
    
    else:
        print("Unknown command: " + cmd, file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
'''


class DynamiqAgent:
    """AI Agent with E2B sandbox and MCP tool access."""
    
    def __init__(
        self,
        user_id: str,
        session_token: str,
        mcp_proxy_url: str = "",
        conversation_id: str = "",
        sandbox_id: str | None = None,
        messages: list[dict] | None = None,
        files: list[dict] | None = None,
    ):
        self.user_id = user_id
        self.session_token = session_token
        self.mcp_proxy_url = mcp_proxy_url or DEFAULT_MCP_PROXY_URL
        self.conversation_id = conversation_id
        self.existing_sandbox_id = sandbox_id  # For reconnecting to existing sandbox
        self.messages_history = messages or []  # Conversation history
        self.pending_files = files or []  # Files to upload to sandbox
        self.backend_url = derive_backend_url(self.mcp_proxy_url)
        self.sandbox: Sandbox | None = None
        self.sandbox_reused = False  # Track if we reused an existing sandbox
        self.client = OpenAI(api_key=OPENAI_API_KEY)
        
        if not self.mcp_proxy_url:
            print("[Agent] WARNING: MCP_PROXY_URL not set - MCP commands will not work")
        if not self.backend_url:
            print("[Agent] WARNING: BACKEND_URL not set - cloud credentials won't be configured")
    
    async def setup(self) -> str:
        """Create or reconnect to E2B sandbox. Returns sandbox ID."""
        
        # Try to reconnect to existing sandbox if provided
        if self.existing_sandbox_id:
            print(f"[Agent] Attempting to reconnect to sandbox: {self.existing_sandbox_id}")
            try:
                self.sandbox = await asyncio.to_thread(
                    Sandbox.connect,
                    self.existing_sandbox_id,
                )
                # Update MCP environment variables - append to existing .env_session to preserve PG creds
                # Note: E2B sandbox commands run in their own shell, so we update bashrc
                mcp_env_setup = f'''
export MCP_SESSION_TOKEN="{self.session_token}"
export MCP_PROXY_URL="{self.mcp_proxy_url}"
export MCP_USER_ID="{self.user_id}"
'''
                # Read existing .env_session to preserve PostgreSQL credentials
                try:
                    existing_env = self.sandbox.files.read("/home/user/.env_session")
                    # Remove old MCP vars (they'll be replaced)
                    lines = [l for l in existing_env.split('\n') 
                             if not l.startswith('export MCP_')]
                    existing_env = '\n'.join(lines)
                except:
                    existing_env = ""
                
                # Write combined env file (preserving PG creds, updating MCP vars)
                self.sandbox.files.write("/home/user/.env_session", existing_env + mcp_env_setup)
                
                # Source in bashrc if not already there
                self.sandbox.commands.run(
                    'grep -q ".env_session" ~/.bashrc || echo \'[ -f ~/.env_session ] && source ~/.env_session\' >> ~/.bashrc'
                )
                
                # Ensure uploads directory exists (for file checks)
                self.sandbox.commands.run("mkdir -p /home/user/uploads")
                
                # Re-run cloud credentials setup to ensure everything is current
                # (This also re-installs psql if needed and refreshes credentials)
                await self._setup_cloud_credentials()
                
                # Upload any pending files
                await self._upload_files()
                
                self.sandbox_reused = True
                print(f"[Agent] Reconnected to existing sandbox: {self.sandbox.sandbox_id}")
                return self.sandbox.sandbox_id
            except Exception as e:
                print(f"[Agent] Failed to reconnect to sandbox: {e}, creating new one...")
                # Fall through to create new sandbox
        
        print("[Agent] Creating new E2B sandbox...")
        
        # Create sandbox with environment variables using Sandbox.create()
        # Timeout of 30 minutes (1800 seconds) to keep sandbox alive for demos
        self.sandbox = await asyncio.to_thread(
            Sandbox.create,
            timeout=1800,  # 30 minutes - keeps sandbox alive longer for demos
            envs={
                "MCP_PROXY_URL": self.mcp_proxy_url,
                "MCP_SESSION_TOKEN": self.session_token,
                "MCP_USER_ID": self.user_id,
            }
        )
        
        print(f"[Agent] Sandbox created: {self.sandbox.sandbox_id}")
        
        # Create uploads directory (always, so agent can check it)
        self.sandbox.commands.run("mkdir -p /home/user/uploads")
        
        # Install data science packages
        await self._install_data_packages()
        
        # Install MCP CLI
        await self._install_mcp_cli()

        # Install cloud credential helpers (AWS/GCP)
        await self._setup_cloud_credentials()

        # Install GitHub CLI wrapper (gh)
        await self._setup_github_cli()
        
        # Upload any pending files
        await self._upload_files()
        
        return self.sandbox.sandbox_id
    
    async def _install_data_packages(self):
        """Install common data science and visualization packages in the sandbox."""
        print("[Agent] Installing data science and visualization packages...")
        
        # Install Python packages for data analysis and visualization
        packages = [
            "pandas",           # Data manipulation
            "numpy",            # Numerical computing
            "scipy",            # Scientific computing
            "psycopg2-binary",  # PostgreSQL driver
            "openpyxl",         # Excel read/write
            "xlsxwriter",       # Excel writing with charts
            "matplotlib",       # Plotting
            "seaborn",          # Statistical visualization
            "pillow",           # Image processing
            "reportlab",        # PDF generation
            "pdfplumber",       # PDF text extraction
            "plotly",           # Interactive charts
            "kaleido",          # Plotly static image export
        ]
        
        result = self.sandbox.commands.run(
            f"pip install -q {' '.join(packages)}",
            timeout=180  # Allow more time for package installation
        )
        if result.exit_code == 0:
            print("[Agent] Python packages installed successfully")
        else:
            print(f"[Agent] Warning: Python package installation had issues: {result.stderr}")
        
        # Install system tools for image/diagram generation and database clients
        # Try simple approach first - just run apt-get directly
        print("[Agent] Installing system tools (including psql)...")
        try:
            # Simple apt-get install - the E2B sandbox should handle this
            apt_cmd = "sudo apt-get update -qq && sudo apt-get install -y postgresql-client graphviz imagemagick poppler-utils fonts-liberation"
            sys_result = self.sandbox.commands.run(apt_cmd, timeout=300)
            if sys_result.exit_code == 0:
                print("[Agent] System tools installed successfully")
                # Verify psql is available
                verify = self.sandbox.commands.run("which psql")
                if verify.stdout.strip():
                    print(f"[Agent] psql installed at: {verify.stdout.strip()}")
                else:
                    print("[Agent] Warning: psql not found after installation")
            else:
                error_msg = sys_result.stderr[:500] if sys_result.stderr else sys_result.stdout[:500] if sys_result.stdout else 'unknown'
                print(f"[Agent] Warning: System tools installation failed (exit {sys_result.exit_code}): {error_msg}")
                # Try alternative: just install psql
                print("[Agent] Trying to install just postgresql-client...")
                psql_result = self.sandbox.commands.run("sudo apt-get install -y postgresql-client", timeout=120)
                if psql_result.exit_code == 0:
                    print("[Agent] postgresql-client installed")
                else:
                    print(f"[Agent] postgresql-client also failed: {psql_result.stderr[:200] if psql_result.stderr else 'unknown'}")
        except Exception as e:
            print(f"[Agent] Exception during system tools installation: {e}")
    
    async def _install_mcp_cli(self):
        """Upload MCP CLI script to sandbox."""
        print("[Agent] Installing MCP CLI...")
        self.sandbox.files.write("/usr/local/bin/mcp", MCP_CLI_SCRIPT)
        result = self.sandbox.commands.run("chmod +x /usr/local/bin/mcp")
        print(f"[Agent] chmod result: exit={result.exit_code}")
        
        # Verify the script is valid Python
        verify = self.sandbox.commands.run("python3 -m py_compile /usr/local/bin/mcp")
        if verify.exit_code != 0:
            print(f"[Agent] ERROR: MCP CLI has syntax errors: {verify.stderr}")
            raise Exception(f"MCP CLI syntax error: {verify.stderr}")
        
        print("[Agent] MCP CLI installed and verified")

    async def _setup_cloud_credentials(self):
        """Configure cloud credential helpers inside the sandbox."""
        if not self.sandbox or not self.backend_url:
            return

        # Always ensure AWS CLI is available (credentials may be added later)
        print("[Agent] Ensuring AWS CLI is installed...")
        self.sandbox.commands.run(AWS_CLI_INSTALL_CMD)

        print(f"[Agent] Fetching sandbox cloud credential config...")
        print(f"[Agent] DEBUG: backend_url={self.backend_url}")
        print(f"[Agent] DEBUG: user_id={self.user_id}")
        print(f"[Agent] DEBUG: sandbox_id={self.sandbox.sandbox_id}")
        
        payload = json.dumps({
            "userId": self.user_id,
            "sandboxId": self.sandbox.sandbox_id,
            "conversationId": self.conversation_id,
        }).encode()

        try:
            url = f"{self.backend_url}/api/cloud/sandbox-config"
            print(f"[Agent] DEBUG: Requesting {url}")
            req = urllib.request.Request(
                url,
                data=payload,
                headers={"Content-Type": "application/json", "ngrok-skip-browser-warning": "true"},
            )
            with urllib.request.urlopen(req, timeout=15) as resp:
                config = json.loads(resp.read())
            print(f"[Agent] DEBUG: Got config: postgresEnabled={config.get('postgresEnabled')}")
        except Exception as e:
            print(f"[Agent] Cloud config fetch failed: {e}")
            import traceback
            traceback.print_exc()
            return

        if config.get("awsEnabled"):
            helper = config.get("awsCredentialHelper", "")
            aws_config = config.get("awsConfig", "")
            if helper and aws_config:
                print("[Agent] Installing AWS credential helper...")
                helper_path = "/home/user/.local/bin/aws-credential-helper"
                aws_config = aws_config.replace("/usr/local/bin/aws-credential-helper", helper_path)
                self.sandbox.commands.run("mkdir -p /home/user/.local/bin /home/user/.aws")
                self.sandbox.files.write(helper_path, helper)
                self.sandbox.commands.run(f"chmod +x {helper_path}")
                self.sandbox.files.write("/home/user/.aws/config", aws_config)
                self.sandbox.commands.run(AWS_CLI_INSTALL_CMD)
            else:
                print("[Agent] AWS credential helper config missing")

        # Setup PostgreSQL credentials if enabled
        if config.get("postgresEnabled"):
            pg_env_vars = config.get("postgresEnvVars", {})
            if pg_env_vars:
                print("[Agent] Setting up PostgreSQL credentials...")
                # Install psql client (needs sudo in E2B sandbox)
                print("[Agent] Installing PostgreSQL client (psql)...")
                try:
                    # Clean up apt locks and install psql
                    psql_install_cmd = """
                        # Wait for any running apt processes
                        while fuser /var/lib/apt/lists/lock >/dev/null 2>&1 || fuser /var/lib/dpkg/lock >/dev/null 2>&1 || fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
                            sleep 2
                        done
                        # Remove stale locks
                        sudo rm -f /var/lib/apt/lists/lock /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend 2>/dev/null || true
                        # Install postgresql-client
                        sudo apt-get update -qq && sudo apt-get install -y -qq postgresql-client
                    """
                    install_result = self.sandbox.commands.run(psql_install_cmd, timeout=90)
                    print(f"[Agent] psql install completed: exit_code={install_result.exit_code}")
                    if install_result.exit_code != 0:
                        print(f"[Agent] psql install stderr: {install_result.stderr[:200] if install_result.stderr else 'none'}")
                except Exception as e:
                    print(f"[Agent] psql install error: {e}")
                
                # Verify psql is installed
                try:
                    verify_result = self.sandbox.commands.run("which psql", timeout=5)
                    psql_path = verify_result.stdout.strip()
                    print(f"[Agent] psql location: {psql_path}")
                    if not psql_path or "not found" in psql_path:
                        print("[Agent] WARNING: psql not found after install")
                except Exception as e:
                    print(f"[Agent] WARNING: psql verification failed: {e}")
                
                # Write environment variables to bashrc and env_session
                pg_env_lines = "\n".join([f'export {k}="{v}"' for k, v in pg_env_vars.items()])
                
                # Append to .env_session for this session
                env_session_content = f"\n# PostgreSQL credentials\n{pg_env_lines}\n"
                try:
                    existing = self.sandbox.files.read("/home/user/.env_session")
                    env_session_content = existing + env_session_content
                except:
                    pass
                self.sandbox.files.write("/home/user/.env_session", env_session_content)
                
                # Also add to bashrc for persistence
                self.sandbox.commands.run(
                    f'grep -q "PGHOST" ~/.bashrc || echo \'{pg_env_lines}\' >> ~/.bashrc'
                )
                print(f"[Agent] PostgreSQL configured: {pg_env_vars.get('PGHOST')}:{pg_env_vars.get('PGPORT')}/{pg_env_vars.get('PGDATABASE')}")

    async def _setup_github_cli(self):
        """Configure GitHub CLI with a short-lived token helper."""
        if not self.sandbox or not self.backend_url:
            return

        helper_script = build_github_token_helper(self.backend_url, self.session_token)
        self.sandbox.commands.run("mkdir -p /home/user/.local/bin")
        self.sandbox.files.write("/home/user/.local/bin/gh-token", helper_script)
        self.sandbox.commands.run("chmod +x /home/user/.local/bin/gh-token")
        self.sandbox.files.write("/home/user/.local/bin/gh", GH_WRAPPER_SCRIPT)
        self.sandbox.commands.run("chmod +x /home/user/.local/bin/gh")
        self.sandbox.commands.run(GH_CLI_INSTALL_CMD)
    
    async def _upload_files(self):
        """Upload pending files to sandbox."""
        print(f"[Agent] _upload_files called, pending_files count: {len(self.pending_files) if self.pending_files else 0}")
        if not self.sandbox or not self.pending_files:
            return
        
        # Create uploads directory
        uploads_dir = "/home/user/uploads"
        self.sandbox.commands.run(f"mkdir -p {uploads_dir}")
        
        uploaded_files = []
        for file_info in self.pending_files:
            try:
                name = file_info.get("name", "file")
                data_b64 = file_info.get("data", "")
                
                # Decode base64 data
                try:
                    file_data = base64.b64decode(data_b64)
                except Exception as e:
                    print(f"[Agent] Failed to decode file {name}: {e}")
                    continue
                
                # Write file to sandbox (E2B accepts bytes directly)
                file_path = f"{uploads_dir}/{name}"
                self.sandbox.files.write(file_path, file_data)
                uploaded_files.append(file_path)
                print(f"[Agent] Uploaded file: {file_path} ({len(file_data)} bytes)")
                
            except Exception as e:
                print(f"[Agent] Failed to upload file {file_info.get('name', 'unknown')}: {e}")
        
        if uploaded_files:
            print(f"[Agent] Uploaded {len(uploaded_files)} file(s) to {uploads_dir}")
        
        # Clear pending files after upload
        self.pending_files = []
    
    async def cleanup(self, keep_sandbox: bool = False):
        """Clean up resources. If keep_sandbox=True, keep sandbox alive for conversation continuity."""
        if self.sandbox:
            sandbox_id = self.sandbox.sandbox_id
            if keep_sandbox:
                print(f"[Agent] Keeping sandbox alive for conversation continuity: {sandbox_id}")
                # Don't kill the sandbox, just disconnect from it
                # The sandbox will stay running (E2B sandboxes have a 1-hour default timeout)
                return
            
            print(f"[Agent] Cleaning up sandbox: {sandbox_id}")
            try:
                await asyncio.to_thread(self.sandbox.kill)
            except Exception as e:
                print(f"[Agent] Cleanup warning: {e}")
    
    # =========================================================================
    # Tool Implementations
    # =========================================================================
    
    def execute_command(self, command: str, cwd: str = "/home/user") -> str:
        """Execute shell command in sandbox."""
        if not self.sandbox:
            return "Error: Sandbox not initialized"
        
        print(f"[Tool] execute_command: {command}")
        
        try:
            # Source env_session for updated credentials, then run command
            result = self.sandbox.commands.run(
                f'[ -f ~/.env_session ] && source ~/.env_session; export PATH="/home/user/.local/bin:$PATH" && cd {cwd} && {command}',
                timeout=120
            )
            
            output = ""
            if result.stdout:
                output += result.stdout
            if result.stderr:
                if output:
                    output += "\n"
                output += result.stderr
            
            if not output.strip():
                output = "(command completed with no output)"
            
            # Log full output for debugging
            print(f"[Tool] execute_command result: exit={result.exit_code}, output={output[:200]}...")
            
            if result.exit_code != 0:
                return f"[Exit code {result.exit_code}]\n{output}"
            
            return output
            
        except Exception as e:
            error_msg = f"Error executing command: {e}"
            print(f"[Tool] {error_msg}")
            return error_msg
    
    def write_file(self, path: str, content: str) -> str:
        """Write file in sandbox."""
        if not self.sandbox:
            return "Error: Sandbox not initialized"
        
        print(f"[Tool] write_file: {path} ({len(content)} bytes)")
        
        try:
            self.sandbox.files.write(path, content)
            return f"Successfully wrote {len(content)} bytes to {path}"
        except Exception as e:
            return f"Error writing file: {e}"
    
    def read_file(self, path: str) -> str:
        """Read file from sandbox."""
        if not self.sandbox:
            return "Error: Sandbox not initialized"
        
        print(f"[Tool] read_file: {path}")
        
        try:
            content = self.sandbox.files.read(path)
            return content
        except Exception as e:
            return f"Error reading file: {e}"
    
    def return_file(self, path: str, description: str = "") -> dict:
        """Return a file to the user for download."""
        if not self.sandbox:
            return {"error": "Sandbox not initialized"}
        
        print(f"[Tool] return_file: {path}")
        
        try:
            # Read file as bytes
            content = self.sandbox.files.read(path)
            
            # If content is string, encode to bytes
            if isinstance(content, str):
                content_bytes = content.encode('utf-8')
            else:
                content_bytes = content
            
            # Base64 encode
            content_b64 = base64.b64encode(content_bytes).decode('ascii')
            
            # Get filename from path
            filename = path.split('/')[-1]
            
            # Determine MIME type
            ext = filename.split('.')[-1].lower() if '.' in filename else ''
            mime_types = {
                'csv': 'text/csv',
                'json': 'application/json',
                'txt': 'text/plain',
                'md': 'text/markdown',
                'html': 'text/html',
                'png': 'image/png',
                'jpg': 'image/jpeg',
                'jpeg': 'image/jpeg',
                'gif': 'image/gif',
                'webp': 'image/webp',
                'svg': 'image/svg+xml',
                'pdf': 'application/pdf',
                'xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
                'xls': 'application/vnd.ms-excel',
                'zip': 'application/zip',
            }
            mime_type = mime_types.get(ext, 'application/octet-stream')
            
            print(f"[Tool] return_file: {filename} ({len(content_bytes)} bytes, {mime_type})")
            
            return {
                "_file_artifact": True,
                "filename": filename,
                "mime_type": mime_type,
                "size": len(content_bytes),
                "data": content_b64,
                "description": description or f"File: {filename}",
            }
        except Exception as e:
            print(f"[Tool] return_file error: {e}")
            return {"error": f"Error reading file: {e}"}
    
    def _execute_tool(self, name: str, args: dict) -> str | dict:
        """Execute tool by name. Returns string for most tools, dict for file artifacts."""
        if name == "execute_command":
            return self.execute_command(
                args.get("command", ""),
                args.get("cwd", "/home/user")
            )
        elif name == "write_file":
            return self.write_file(
                args.get("path", ""),
                args.get("content", "")
            )
        elif name == "read_file":
            return self.read_file(args.get("path", ""))
        elif name == "return_file":
            return self.return_file(
                args.get("path", ""),
                args.get("description", "")
            )
        else:
            return f"Unknown tool: {name}"
    
    # =========================================================================
    # Agent Loop
    # =========================================================================
    
    async def run(
        self,
        user_message: str,
        on_event: Callable[[str, Any], None] | None = None
    ) -> str:
        """
        Run agent with user message.
        
        Args:
            user_message: User's input
            on_event: Callback for streaming events (event_type, data)
        
        Returns:
            Final assistant response
        """
        def emit(event_type: str, data: Any):
            if on_event:
                on_event(event_type, data)
            # Also log
            if isinstance(data, dict):
                print(f"[Event] {event_type}: {json.dumps(data)[:150]}")
            else:
                print(f"[Event] {event_type}: {str(data)[:150]}")
        
        # Build messages array with system prompt, history, and new message
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT},
        ]
        
        # Add conversation history (previous messages)
        if self.messages_history:
            print(f"[Agent] Including {len(self.messages_history)} messages from history")
            for msg in self.messages_history:
                # Convert tool calls to OpenAI format if present
                msg_dict = {"role": msg.get("role", "user"), "content": msg.get("content", "")}
                
                # Handle tool calls in assistant messages
                if msg.get("tool_calls") and msg["role"] == "assistant":
                    openai_tool_calls = []
                    for tc in msg["tool_calls"]:
                        openai_tool_calls.append({
                            "id": tc.get("id", ""),
                            "type": "function",
                            "function": {
                                "name": tc.get("name", ""),
                                "arguments": tc.get("arguments", "{}"),
                            }
                        })
                    if openai_tool_calls:
                        msg_dict["tool_calls"] = openai_tool_calls
                        # OpenAI requires no content when there are tool_calls
                        if not msg_dict["content"]:
                            msg_dict["content"] = None
                
                messages.append(msg_dict)
                
                # Add tool results as separate messages for each tool call
                # OpenAI requires a tool message for EVERY tool_call, even if result is empty
                if msg.get("tool_calls") and msg["role"] == "assistant":
                    for tc in msg["tool_calls"]:
                        messages.append({
                            "role": "tool",
                            "tool_call_id": tc.get("id", ""),
                            "content": tc.get("result", "") or "(no result)",
                        })
        
        # Add the new user message
        messages.append({"role": "user", "content": user_message})
        
        # Agent loop - max 15 iterations
        for iteration in range(15):
            emit("thinking", {"iteration": iteration + 1})
            
            try:
                response = await asyncio.to_thread(
                    self.client.chat.completions.create,
                    model=MODEL,
                    messages=messages,
                    tools=TOOLS,
                    tool_choice="auto",
                    timeout=120,
                )
            except Exception as e:
                emit("error", {"message": f"LLM error: {e}"})
                return f"Error calling LLM: {e}"
            
            msg = response.choices[0].message
            
            # No tool calls - return final response
            if not msg.tool_calls:
                final_response = msg.content or ""
                emit("message", {"content": final_response})
                return final_response
            
            # Add assistant message with tool calls
            messages.append(msg)
            
            # Execute each tool call
            for tc in msg.tool_calls:
                try:
                    args = json.loads(tc.function.arguments)
                except json.JSONDecodeError:
                    args = {}
                
                emit("tool_call", {
                    "id": tc.id,
                    "name": tc.function.name,
                    "arguments": json.dumps(args)  # Frontend expects stringified JSON
                })
                
                # Execute the tool
                result = await asyncio.to_thread(
                    self._execute_tool,
                    tc.function.name,
                    args,
                )
                
                # Handle file artifacts specially
                if isinstance(result, dict) and result.get("_file_artifact"):
                    # Emit file event for frontend to handle
                    emit("file", {
                        "filename": result.get("filename", "file"),
                        "mime_type": result.get("mime_type", "application/octet-stream"),
                        "size": result.get("size", 0),
                        "data": result.get("data", ""),
                        "description": result.get("description", ""),
                    })
                    # Convert to string result for the LLM
                    result = f"File '{result.get('filename')}' ({result.get('size')} bytes) has been sent to the user for download."
                elif isinstance(result, dict) and result.get("error"):
                    result = result.get("error")
                elif isinstance(result, dict):
                    result = json.dumps(result)
                
                # Truncate very long results
                if len(result) > 8000:
                    result = result[:8000] + "\n\n... (output truncated)"
                
                emit("tool_result", {
                    "id": tc.id,
                    "name": tc.function.name,
                    "result": result[:1000] + ("..." if len(result) > 1000 else "")
                })
                
                # Add tool result to messages
                messages.append({
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": result
                })
        
        return "Maximum iterations reached. Please try a simpler request."


# =============================================================================
# CLI for Testing
# =============================================================================

async def main():
    """Command-line interface for testing."""
    import sys
    
    if len(sys.argv) < 2:
        print("Dynamiq Agent CLI")
        print()
        print("Usage: python main.py '<message>' [user_id] [session_token] [mcp_proxy_url]")
        print()
        print("Environment variables:")
        print("  LLM_API_KEY or OPENAI_API_KEY - OpenAI API key")
        print("  E2B_API_KEY - E2B API key")
        print("  MCP_PROXY_URL - Backend MCP proxy URL (defaults to BACKEND_URL + /api/mcp/proxy)")
        print("  LLM_MODEL - Model to use (default: gpt-4o)")
        sys.exit(0)
    
    message = sys.argv[1]
    user_id = sys.argv[2] if len(sys.argv) > 2 else "default-user"
    session_token = sys.argv[3] if len(sys.argv) > 3 else "test-token"
    mcp_proxy_url = sys.argv[4] if len(sys.argv) > 4 else ""
    
    print(f"[CLI] Message: {message[:50]}...")
    print(f"[CLI] User: {user_id}")
    print(f"[CLI] MCP Proxy: {mcp_proxy_url or DEFAULT_MCP_PROXY_URL or '(not set)'}")
    print()
    
    agent = DynamiqAgent(user_id, session_token, mcp_proxy_url)
    
    try:
        await agent.setup()
        print()
        result = await agent.run(message)
        print()
        print("=" * 60)
        print("FINAL RESPONSE:")
        print("=" * 60)
        print(result)
    finally:
        await agent.cleanup()


if __name__ == "__main__":
    asyncio.run(main())
