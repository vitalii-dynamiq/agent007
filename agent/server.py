#!/usr/bin/env python3
"""
Dynamiq Agent HTTP Server

Exposes the agent as an HTTP API that the Go backend calls.
Supports SSE streaming for real-time events.
"""
import os
import json
import asyncio
from typing import AsyncGenerator
from contextlib import asynccontextmanager
from dotenv import load_dotenv

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from main import DynamiqAgent

load_dotenv()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """App lifespan."""
    print("[Server] Starting Dynamiq Agent server...")
    yield
    print("[Server] Shutting down...")


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


class RunRequest(BaseModel):
    """Request to run agent."""
    message: str
    messages: list[MessageHistory] | None = None  # Full conversation history
    user_id: str
    session_token: str
    conversation_id: str | None = None
    sandbox_id: str | None = None  # Reuse existing sandbox
    mcp_proxy_url: str | None = None  # Backend passes this


class RunResponse(BaseModel):
    """Response from non-streaming run."""
    response: str
    sandbox_id: str | None = None


@app.get("/health")
async def health():
    """Health check."""
    return {"status": "ok", "service": "dynamiq-agent"}


@app.post("/run", response_model=RunResponse)
async def run_agent(req: RunRequest):
    """Run agent and return result (non-streaming)."""
    # Convert message history to list of dicts
    messages = None
    if req.messages:
        messages = [{"role": m.role, "content": m.content, "tool_calls": m.tool_calls} for m in req.messages]
    
    agent = DynamiqAgent(
        user_id=req.user_id,
        session_token=req.session_token,
        mcp_proxy_url=req.mcp_proxy_url or "",
        conversation_id=req.conversation_id or "",
        sandbox_id=req.sandbox_id,  # Pass existing sandbox ID
        messages=messages,  # Pass conversation history
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
        
        agent = DynamiqAgent(
            user_id=req.user_id,
            session_token=req.session_token,
            mcp_proxy_url=req.mcp_proxy_url or "",
            conversation_id=req.conversation_id or "",
            sandbox_id=req.sandbox_id,  # Pass existing sandbox ID
            messages=messages,  # Pass conversation history
        )
        
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
            # Status: Creating or reconnecting to sandbox
            if req.sandbox_id:
                yield sse_event("status", {"message": "Reconnecting to sandbox..."})
            else:
                yield sse_event("status", {"message": "Creating sandbox..."})
            
            # Setup sandbox (will reuse existing if sandbox_id provided)
            sandbox_id = await agent.setup()
            yield sse_event("status", {
                "message": "Sandbox ready",
                "sandbox_id": sandbox_id,
                "reused": req.sandbox_id is not None and req.sandbox_id == sandbox_id
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
