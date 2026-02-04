#!/usr/bin/env python3
"""
Dynamiq Agent HTTP Server

Exposes the agent as an HTTP API that the Go backend calls.
Supports SSE streaming for real-time events.
"""
import os
import json
import asyncio
import time
from typing import AsyncGenerator
from contextlib import asynccontextmanager
from dotenv import load_dotenv

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from main import DynamiqAgent

load_dotenv()


# Warm sandbox pool - stores pre-warmed sandboxes per user
# Key: user_id, Value: {"sandbox_id": str, "agent": DynamiqAgent, "created_at": float, "ready": bool}
warm_sandbox_pool: dict[str, dict] = {}
warm_sandbox_lock = asyncio.Lock()

# How long to keep warm sandboxes (25 minutes, slightly less than E2B timeout)
WARM_SANDBOX_TTL = 25 * 60


async def cleanup_expired_sandboxes():
    """Clean up expired warm sandboxes."""
    async with warm_sandbox_lock:
        now = time.time()
        expired = [uid for uid, info in warm_sandbox_pool.items() 
                   if now - info["created_at"] > WARM_SANDBOX_TTL]
        for uid in expired:
            print(f"[Server] Cleaning up expired warm sandbox for user {uid}")
            try:
                agent = warm_sandbox_pool[uid].get("agent")
                if agent:
                    await agent.cleanup(keep_sandbox=False)
            except Exception as e:
                print(f"[Server] Error cleaning up sandbox: {e}")
            del warm_sandbox_pool[uid]


@asynccontextmanager
async def lifespan(app: FastAPI):
    """App lifespan."""
    print("[Server] Starting Dynamiq Agent server...")
    yield
    print("[Server] Shutting down...")
    # Cleanup all warm sandboxes on shutdown
    for uid, info in warm_sandbox_pool.items():
        try:
            agent = info.get("agent")
            if agent:
                await agent.cleanup(keep_sandbox=False)
        except Exception:
            pass


app = FastAPI(
    title="Dynamiq Agent",
    description="AI Agent with E2B Sandbox and MCP Tools",
    lifespan=lifespan,
)

# CORS for local development
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


class MessageHistory(BaseModel):
    """A message in the conversation history."""
    role: str
    content: str
    tool_calls: list[dict] | None = None


class UploadedFile(BaseModel):
    """A file uploaded by the user."""
    name: str
    size: int
    type: str
    data: str  # base64 encoded


class RunRequest(BaseModel):
    """Request to run agent."""
    message: str
    messages: list[MessageHistory] | None = None  # Full conversation history
    user_id: str
    session_token: str
    conversation_id: str | None = None
    sandbox_id: str | None = None  # Reuse existing sandbox
    mcp_proxy_url: str | None = None  # Backend passes this
    files: list[UploadedFile] | None = None  # Files to upload to sandbox


class RunResponse(BaseModel):
    """Response from non-streaming run."""
    response: str
    sandbox_id: str | None = None


class WarmRequest(BaseModel):
    """Request to pre-warm a sandbox."""
    user_id: str
    session_token: str
    mcp_proxy_url: str | None = None


class WarmResponse(BaseModel):
    """Response from warm endpoint."""
    status: str
    sandbox_id: str | None = None
    ready: bool = False
    message: str | None = None


@app.get("/health")
async def health():
    """Health check."""
    return {"status": "ok", "service": "dynamiq-agent"}


@app.post("/warm", response_model=WarmResponse)
async def warm_sandbox(req: WarmRequest):
    """
    Pre-warm a sandbox for a user.
    
    This creates and fully initializes a sandbox in the background,
    so when the user sends their first message, it's ready immediately.
    """
    # Clean up expired sandboxes first
    await cleanup_expired_sandboxes()
    
    async with warm_sandbox_lock:
        # Check if we already have a warm sandbox for this user
        if req.user_id in warm_sandbox_pool:
            info = warm_sandbox_pool[req.user_id]
            if info.get("ready"):
                print(f"[Server] Warm sandbox already ready for user {req.user_id}: {info['sandbox_id']}")
                return WarmResponse(
                    status="ready",
                    sandbox_id=info["sandbox_id"],
                    ready=True,
                    message="Sandbox already warmed and ready"
                )
            else:
                print(f"[Server] Warm sandbox still initializing for user {req.user_id}")
                return WarmResponse(
                    status="warming",
                    sandbox_id=info.get("sandbox_id"),
                    ready=False,
                    message="Sandbox is being initialized"
                )
        
        # Mark as warming (not ready yet)
        warm_sandbox_pool[req.user_id] = {
            "sandbox_id": None,
            "agent": None,
            "created_at": time.time(),
            "ready": False,
        }
    
    # Start warming in background (outside lock)
    asyncio.create_task(_do_warm_sandbox(req))
    
    return WarmResponse(
        status="warming",
        sandbox_id=None,
        ready=False,
        message="Sandbox warming started"
    )


async def _do_warm_sandbox(req: WarmRequest):
    """Background task to warm a sandbox."""
    try:
        print(f"[Server] Starting sandbox warm-up for user {req.user_id}")
        
        agent = DynamiqAgent(
            user_id=req.user_id,
            session_token=req.session_token,
            mcp_proxy_url=req.mcp_proxy_url or "",
            conversation_id="",  # No conversation yet
            sandbox_id=None,
            messages=None,
            files=None,
        )
        
        # Setup the sandbox (this does all the initialization)
        sandbox_id = await agent.setup()
        
        # Update the pool
        async with warm_sandbox_lock:
            if req.user_id in warm_sandbox_pool:
                warm_sandbox_pool[req.user_id].update({
                    "sandbox_id": sandbox_id,
                    "agent": agent,
                    "ready": True,
                })
                print(f"[Server] Sandbox warmed successfully for user {req.user_id}: {sandbox_id}")
            else:
                # User was removed from pool (maybe expired), cleanup
                print(f"[Server] User {req.user_id} no longer in pool, cleaning up sandbox")
                await agent.cleanup(keep_sandbox=False)
                
    except Exception as e:
        print(f"[Server] Error warming sandbox for user {req.user_id}: {e}")
        async with warm_sandbox_lock:
            if req.user_id in warm_sandbox_pool:
                del warm_sandbox_pool[req.user_id]


@app.get("/warm/status/{user_id}")
async def warm_status(user_id: str):
    """Check status of a warm sandbox for a user."""
    # Simple non-blocking check
    if user_id not in warm_sandbox_pool:
        return WarmResponse(
            status="none",
            sandbox_id=None,
            ready=False,
            message="No warm sandbox for this user"
        )
    
    info = warm_sandbox_pool[user_id]
    return WarmResponse(
        status="ready" if info["ready"] else "warming",
        sandbox_id=info.get("sandbox_id"),
        ready=info["ready"],
        message="Sandbox ready" if info["ready"] else "Sandbox still initializing"
    )


@app.post("/run", response_model=RunResponse)
async def run_agent(req: RunRequest):
    """Run agent and return result (non-streaming)."""
    # Convert message history to list of dicts
    messages = None
    if req.messages:
        messages = [{"role": m.role, "content": m.content, "tool_calls": m.tool_calls} for m in req.messages]
    
    # Convert files to list of dicts
    files = None
    if req.files:
        files = [{"name": f.name, "size": f.size, "type": f.type, "data": f.data} for f in req.files]
    
    agent = DynamiqAgent(
        user_id=req.user_id,
        session_token=req.session_token,
        mcp_proxy_url=req.mcp_proxy_url or "",
        conversation_id=req.conversation_id or "",
        sandbox_id=req.sandbox_id,  # Pass existing sandbox ID
        messages=messages,  # Pass conversation history
        files=files,  # Pass uploaded files
    )
    
    try:
        sandbox_id = await agent.setup()
        result = await agent.run(req.message)
        return RunResponse(response=result, sandbox_id=sandbox_id)
    finally:
        await agent.cleanup(keep_sandbox=True)  # Don't kill sandbox for conversation continuity


@app.post("/run/stream")
async def run_agent_stream(req: RunRequest):
    """
    Run agent with SSE streaming.
    
    Events:
    - status: Status updates (sandbox creation, etc.)
    - thinking: Agent is processing
    - tool_call: Tool being called with args
    - tool_result: Tool execution result
    - message: Final response content
    - error: Error occurred
    - done: Stream complete
    """
    async def generate_events() -> AsyncGenerator[str, None]:
        # Convert message history to list of dicts
        messages = None
        if req.messages:
            messages = [{"role": m.role, "content": m.content, "tool_calls": m.tool_calls} for m in req.messages]
        
        # Convert files to list of dicts
        files = None
        if req.files:
            print(f"[Server] Received {len(req.files)} file(s)")
            for f in req.files:
                print(f"[Server]   - {f.name} ({f.size} bytes, {f.type})")
            files = [{"name": f.name, "size": f.size, "type": f.type, "data": f.data} for f in req.files]
        else:
            print("[Server] No files in request")
        
        # Check if we have a warm sandbox for this user
        warm_agent = None
        warm_sandbox_id = None
        async with warm_sandbox_lock:
            if req.user_id in warm_sandbox_pool and warm_sandbox_pool[req.user_id].get("ready"):
                warm_info = warm_sandbox_pool.pop(req.user_id)  # Take ownership
                warm_agent = warm_info.get("agent")
                warm_sandbox_id = warm_info.get("sandbox_id")
                print(f"[Server] Using warm sandbox for user {req.user_id}: {warm_sandbox_id}")
        
        # Use warm agent if available, otherwise create new
        if warm_agent and warm_sandbox_id and not req.sandbox_id:
            # Update the warm agent with conversation-specific data
            warm_agent.conversation_id = req.conversation_id or ""
            warm_agent.pending_files = files or []  # Correct attribute name
            warm_agent.messages_history = messages or []  # Correct attribute name
            agent = warm_agent
            sandbox_id = warm_sandbox_id
            using_warm = True
        else:
            agent = DynamiqAgent(
                user_id=req.user_id,
                session_token=req.session_token,
                mcp_proxy_url=req.mcp_proxy_url or "",
                conversation_id=req.conversation_id or "",
                sandbox_id=req.sandbox_id,  # Pass existing sandbox ID
                files=files,  # Pass uploaded files
                messages=messages,  # Pass conversation history
            )
            using_warm = False
        
        # Queue for collecting events from agent
        event_queue: asyncio.Queue = asyncio.Queue()
        
        def on_event(event_type: str, data):
            """Callback for agent events."""
            try:
                # Log all events for debugging
                print(f"[Server] Event: {event_type} = {str(data)[:200]}")
                # Use call_soon_threadsafe since agent runs sync operations
                loop = asyncio.get_event_loop()
                loop.call_soon_threadsafe(
                    event_queue.put_nowait,
                    (event_type, data)
                )
            except Exception as e:
                print(f"[Server] Event queue error: {e}")
        
        try:
            # Setup sandbox (skip if using warm sandbox)
            if using_warm:
                # Upload any files to the warm sandbox
                if files:
                    yield sse_event("status", {"message": "Uploading files..."})
                    await agent._upload_files()
                yield sse_event("status", {
                    "message": "Ready",
                    "sandbox_id": sandbox_id,
                    "reused": True,
                    "warm": True
                })
            elif req.sandbox_id:
                sandbox_id = await agent.setup()
                yield sse_event("status", {
                    "message": "Ready",
                    "sandbox_id": sandbox_id,
                    "reused": req.sandbox_id is not None and req.sandbox_id == sandbox_id
                })
            else:
                yield sse_event("status", {"message": "Preparing..."})
                sandbox_id = await agent.setup()
                yield sse_event("status", {
                    "message": "Ready",
                    "sandbox_id": sandbox_id,
                    "reused": False
                })
            
            # Run agent in background task
            async def run_agent_task():
                try:
                    result = await agent.run(req.message, on_event=on_event)
                    await event_queue.put(("_final_result", result))
                except Exception as e:
                    await event_queue.put(("error", {"message": str(e)}))
                finally:
                    await event_queue.put(("_done", None))
            
            # Start agent task
            task = asyncio.create_task(run_agent_task())
            
            # Stream events as they come
            while True:
                try:
                    event_type, data = await asyncio.wait_for(
                        event_queue.get(),
                        timeout=300  # 5 min timeout
                    )
                except asyncio.TimeoutError:
                    yield sse_event("error", {"message": "Agent timeout"})
                    break
                
                # Check for completion signals
                if event_type == "_done":
                    break
                elif event_type == "_final_result":
                    yield sse_event("message", {"content": data})
                    continue
                
                # Regular events
                yield sse_event(event_type, data)
            
            # Wait for task to complete
            await task
            
            # Send done event
            yield sse_event("done", {})
            
        except Exception as e:
            print(f"[Server] Stream error: {e}")
            yield sse_event("error", {"message": str(e)})
            yield sse_event("done", {})
        
        finally:
            # Keep sandbox alive for conversation continuity
            await agent.cleanup(keep_sandbox=True)
    
    return StreamingResponse(
        generate_events(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",  # Disable nginx buffering
        },
    )


def sse_event(event_type: str, data: dict | str) -> str:
    """Format SSE event."""
    if isinstance(data, str):
        data = {"content": data}
    return f"event: {event_type}\ndata: {json.dumps(data)}\n\n"


if __name__ == "__main__":
    import uvicorn
    
    port = int(os.getenv("AGENT_PORT", "8081"))
    host = os.getenv("AGENT_HOST", "0.0.0.0")
    
    print(f"[Server] Starting on {host}:{port}")
    uvicorn.run(app, host=host, port=port, log_level="info")
