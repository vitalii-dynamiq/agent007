#!/usr/bin/env python3
"""
Dynamiq Agent - AI Agent with E2B Sandbox

Uses OpenAI for LLM and E2B for isolated code execution.
MCP tools are accessed via CLI commands in the sandbox calling our backend proxy.
"""
import os
import json
import asyncio
import urllib.request
from typing import Any, Callable
from dotenv import load_dotenv
from openai import OpenAI
from e2b import Sandbox

load_dotenv()

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

# List available tools for an app
mcp list-tools <app>

# Call a tool with JSON input
mcp call <app> <tool> '<json-input>'
```

### Examples

**Gmail:**
```bash
mcp list-tools gmail
mcp call gmail gmail-find-email '{"instruction": "newer_than:1d"}'
mcp call gmail gmail-send-email '{"instruction": "Send an email to user@example.com with subject Hello and body Hi!"}'
```

**Slack:**
```bash
mcp list-tools slack
mcp call slack send_message '{"channel": "#general", "text": "Hello from Dynamiq!"}'
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

## Workflow

1. For **GitHub**: Use `gh` CLI directly (e.g., `gh repo list`, `gh issue list`)
2. For **AWS/GCP/Azure**: Use cloud CLIs directly (`aws`, `gcloud`, `az`)
3. For **other services** (Gmail, Slack, Notion, etc.): Use `mcp list-apps` first, then `mcp list-tools <app>` and `mcp call`
4. If a service isn't connected, tell the user to connect it via the Integrations panel

## Important Notes

- **GitHub uses `gh` CLI**, not MCP
- Most MCP tools accept a single `instruction` string. Avoid using `q`/`query`.
- Always check connected apps before trying to use them
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
    ):
        self.user_id = user_id
        self.session_token = session_token
        self.mcp_proxy_url = mcp_proxy_url or DEFAULT_MCP_PROXY_URL
        self.conversation_id = conversation_id
        self.existing_sandbox_id = sandbox_id  # For reconnecting to existing sandbox
        self.messages_history = messages or []  # Conversation history
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
                # Update environment variables for the new session in bashrc so they're available
                # Note: E2B sandbox commands run in their own shell, so we update bashrc
                env_setup = f'''
export MCP_SESSION_TOKEN="{self.session_token}"
export MCP_PROXY_URL="{self.mcp_proxy_url}"
export MCP_USER_ID="{self.user_id}"
'''
                self.sandbox.files.write("/home/user/.env_session", env_setup)
                # Source in bashrc if not already there
                self.sandbox.commands.run(
                    'grep -q ".env_session" ~/.bashrc || echo \'[ -f ~/.env_session ] && source ~/.env_session\' >> ~/.bashrc'
                )
                self.sandbox_reused = True
                print(f"[Agent] Reconnected to existing sandbox: {self.sandbox.sandbox_id}")
                return self.sandbox.sandbox_id
            except Exception as e:
                print(f"[Agent] Failed to reconnect to sandbox: {e}, creating new one...")
                # Fall through to create new sandbox
        
        print("[Agent] Creating new E2B sandbox...")
        
        # Create sandbox with environment variables using Sandbox.create()
        self.sandbox = await asyncio.to_thread(
            Sandbox.create,
            timeout=600,  # 10 minutes
            envs={
                "MCP_PROXY_URL": self.mcp_proxy_url,
                "MCP_SESSION_TOKEN": self.session_token,
                "MCP_USER_ID": self.user_id,
            }
        )
        
        print(f"[Agent] Sandbox created: {self.sandbox.sandbox_id}")
        
        # Install MCP CLI
        await self._install_mcp_cli()

        # Install cloud credential helpers (AWS/GCP)
        await self._setup_cloud_credentials()

        # Install GitHub CLI wrapper (gh)
        await self._setup_github_cli()
        
        return self.sandbox.sandbox_id
    
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

        print("[Agent] Fetching sandbox cloud credential config...")
        payload = json.dumps({
            "userId": self.user_id,
            "sandboxId": self.sandbox.sandbox_id,
            "conversationId": self.conversation_id,
        }).encode()

        try:
            req = urllib.request.Request(
                f"{self.backend_url}/api/cloud/sandbox-config",
                data=payload,
                headers={"Content-Type": "application/json"},
            )
            with urllib.request.urlopen(req, timeout=15) as resp:
                config = json.loads(resp.read())
        except Exception as e:
            print(f"[Agent] Cloud config fetch failed: {e}")
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
    
    def _execute_tool(self, name: str, args: dict) -> str:
        """Execute tool by name."""
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
                
                # Add tool results as separate messages if they exist
                if msg.get("tool_calls") and msg["role"] == "assistant":
                    for tc in msg["tool_calls"]:
                        if tc.get("result"):
                            messages.append({
                                "role": "tool",
                                "tool_call_id": tc.get("id", ""),
                                "content": tc.get("result", ""),
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
