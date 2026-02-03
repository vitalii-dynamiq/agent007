import { useEffect, useRef, useState, useCallback } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Message, ToolCall, ToolCallsGroup } from './message'
import { MessageInput, type UploadedFile } from './message-input'
import { ToolSelector } from './tool-selector'
import { api, type Conversation, type SSEEvent } from '@/lib/api'
import { Loader2, Terminal, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ChatViewProps {
  conversation: Conversation | null
  onConversationUpdate: () => void
}

interface StreamingState {
  status?: string
  toolCalls: Map<string, { name: string; arguments: string; result?: string; isExecuting: boolean }>
  assistantContent: string
  userMessage?: string  // The message the user just sent
}

export function ChatView({ conversation, onConversationUpdate }: ChatViewProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [streamingState, setStreamingState] = useState<StreamingState | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<(() => void) | null>(null)

  const enabledTools = conversation?.enabledTools || []

  const handleToolsChange = useCallback(() => {
    onConversationUpdate()
  }, [onConversationUpdate])

  useEffect(() => {
    // Scroll to bottom when messages change
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [conversation?.messages, streamingState])

  const handleSend = async (content: string, files?: UploadedFile[]) => {
    if (!conversation) return

    // Build user message with file info
    let userMessage = content
    if (files && files.length > 0) {
      const fileList = files.map(f => f.name).join(', ')
      userMessage = content 
        ? `${content}\n\n📎 Attached files: ${fileList}`
        : `📎 Attached files: ${fileList}`
    }

    setIsLoading(true)
    setStreamingState({
      status: files?.length ? 'Uploading files...' : 'Sending...',
      toolCalls: new Map(),
      assistantContent: '',
      userMessage,  // Store the user's message to show immediately
    })

    abortRef.current = api.sendMessage(conversation.id, content, (event: SSEEvent) => {
      handleSSEEvent(event)
    }, files)
  }

  const handleSSEEvent = (event: SSEEvent) => {
    switch (event.type) {
      case 'status':
        setStreamingState((prev) => prev ? {
          ...prev,
          status: event.data.message as string,
        } : null)
        break

      case 'thinking':
        setStreamingState((prev) => prev ? {
          ...prev,
          status: typeof event.data === 'object' 
            ? `Processing... (iteration ${(event.data as { iteration?: number }).iteration || 1})`
            : String(event.data),
        } : null)
        break

      case 'tool_call':
        console.log('[ChatView] tool_call event:', event.data)
        setStreamingState((prev) => {
          if (!prev) return null
          const newToolCalls = new Map(prev.toolCalls)
          const data = event.data as { id?: string; name: string; arguments?: string; args?: unknown }
          // Generate id if not provided
          const toolId = data.id || `tool_${Date.now()}_${Math.random().toString(36).slice(2)}`
          // Handle both 'arguments' (string) and 'args' (object)
          const args = data.arguments || (data.args ? JSON.stringify(data.args) : '{}')
          newToolCalls.set(toolId, {
            name: data.name,
            arguments: args,
            isExecuting: true,
          })
          return { ...prev, toolCalls: newToolCalls, status: `Executing ${data.name}...` }
        })
        break

      case 'tool_result':
        console.log('[ChatView] tool_result event:', event.data)
        setStreamingState((prev) => {
          if (!prev) return null
          const newToolCalls = new Map(prev.toolCalls)
          const data = event.data as { id?: string; name?: string; result: string }
          // Try to find the tool call by id, or by name if id not provided
          let toolId = data.id
          if (!toolId && data.name) {
            // Find by name
            for (const [id, tc] of newToolCalls.entries()) {
              if (tc.name === data.name && tc.isExecuting) {
                toolId = id
                break
              }
            }
          }
          if (toolId) {
            const existing = newToolCalls.get(toolId)
            if (existing) {
              newToolCalls.set(toolId, {
                ...existing,
                result: data.result,
                isExecuting: false,
              })
            }
          }
          return { ...prev, toolCalls: newToolCalls, status: undefined }
        })
        break

      case 'message':
        setStreamingState((prev) => prev ? {
          ...prev,
          assistantContent: (event.data as { content: string }).content,
          status: undefined,
        } : null)
        break

      case 'error':
        setStreamingState((prev) => prev ? {
          ...prev,
          status: `Error: ${(event.data as { message: string }).message}`,
        } : null)
        setIsLoading(false)
        break

      case 'done':
        setIsLoading(false)
        setStreamingState(null)
        onConversationUpdate()
        break
    }
  }

  if (!conversation) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center text-muted-foreground">
          <Terminal className="mx-auto h-12 w-12 mb-4 opacity-50" />
          <p>Select a conversation or create a new one</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col min-w-0 overflow-hidden">
      <div className="border-b px-4 py-3 flex items-center justify-between">
        <div className="min-w-0 flex-1">
          <h2 className="font-semibold truncate">
            {conversation.title || 'New conversation'}
          </h2>
          {conversation.sandboxId && (
            <p className="text-xs text-muted-foreground">
              Sandbox: {conversation.sandboxId}
            </p>
          )}
        </div>
        <ToolSelector
          conversationId={conversation.id}
          enabledTools={enabledTools}
          onToolsChange={handleToolsChange}
        />
      </div>

      <ScrollArea className="flex-1 overflow-hidden" ref={scrollRef}>
        <div className="divide-y divide-border/40 overflow-hidden">
          {conversation.messages.map((msg) => (
            <div key={msg.id}>
              {/* Show tool calls for this message if any */}
              {msg.toolCalls && msg.toolCalls.length > 0 && (
                <ToolCallsGroup count={msg.toolCalls.length}>
                  {msg.toolCalls.map((tc) => (
                    <ToolCall
                      key={tc.id}
                      name={tc.name}
                      arguments={tc.arguments}
                      result={tc.result}
                      isExecuting={false}
                    />
                  ))}
                </ToolCallsGroup>
              )}
              {/* Show the message content */}
              {msg.content && (
                <Message
                  role={msg.role}
                  content={msg.content}
                />
              )}
            </div>
          ))}

          {/* Streaming state */}
          {streamingState && (
            <div className="py-1">
              {/* Show user's message immediately */}
              {streamingState.userMessage && (
                <Message
                  role="user"
                  content={streamingState.userMessage}
                />
              )}
              
              {/* Compact status indicator */}
              {streamingState.status && !streamingState.assistantContent && (
                <div className={cn(
                  "flex items-center gap-2 px-4 py-2 text-xs",
                  "text-muted-foreground/70"
                )}>
                  <div className="flex items-center gap-1.5">
                    <Zap className="h-3 w-3 text-amber-500/70" />
                    <Loader2 className="h-3 w-3 animate-spin" />
                  </div>
                  <span className="truncate">{streamingState.status}</span>
                </div>
              )}

              {/* Tool calls - grouped */}
              {streamingState.toolCalls.size > 0 && (
                <ToolCallsGroup count={streamingState.toolCalls.size}>
                  {Array.from(streamingState.toolCalls.entries()).map(([id, tc]) => (
                    <ToolCall
                      key={id}
                      name={tc.name}
                      arguments={tc.arguments}
                      result={tc.result}
                      isExecuting={tc.isExecuting}
                    />
                  ))}
                </ToolCallsGroup>
              )}

              {/* Assistant response */}
              {streamingState.assistantContent && (
                <Message
                  role="assistant"
                  content={streamingState.assistantContent}
                  isStreaming={isLoading}
                />
              )}
            </div>
          )}
        </div>
      </ScrollArea>

      <MessageInput
        onSend={handleSend}
        disabled={isLoading}
        isLoading={isLoading}
      />
    </div>
  )
}
