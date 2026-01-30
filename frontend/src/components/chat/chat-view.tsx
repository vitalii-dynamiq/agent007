import { useEffect, useRef, useState, useCallback } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Message, ToolCall } from './message'
import { MessageInput } from './message-input'
import { ToolSelector } from './tool-selector'
import { api, type Conversation, type SSEEvent } from '@/lib/api'
import { Loader2, Terminal } from 'lucide-react'

interface ChatViewProps {
  conversation: Conversation | null
  onConversationUpdate: () => void
}

interface StreamingState {
  status?: string
  toolCalls: Map<string, { name: string; arguments: string; result?: string; isExecuting: boolean }>
  assistantContent: string
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

  const handleSend = async (content: string) => {
    if (!conversation) return

    setIsLoading(true)
    setStreamingState({
      status: 'Sending...',
      toolCalls: new Map(),
      assistantContent: '',
    })

    abortRef.current = api.sendMessage(conversation.id, content, (event: SSEEvent) => {
      handleSSEEvent(event)
    })
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
    <div className="flex h-full flex-col">
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

      <ScrollArea className="flex-1" ref={scrollRef}>
        <div className="divide-y">
          {conversation.messages.map((msg) => (
            <div key={msg.id}>
              {/* Show tool calls for this message if any */}
              {msg.toolCalls && msg.toolCalls.length > 0 && (
                <>
                  {msg.toolCalls.map((tc) => (
                    <ToolCall
                      key={tc.id}
                      name={tc.name}
                      arguments={tc.arguments}
                      result={tc.result}
                      isExecuting={false}
                    />
                  ))}
                </>
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
            <>
              {/* Status indicator */}
              {streamingState.status && (
                <div className="flex items-center gap-2 px-4 py-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {streamingState.status}
                </div>
              )}

              {/* Tool calls */}
              {Array.from(streamingState.toolCalls.entries()).map(([id, tc]) => (
                <ToolCall
                  key={id}
                  name={tc.name}
                  arguments={tc.arguments}
                  result={tc.result}
                  isExecuting={tc.isExecuting}
                />
              ))}

              {/* Assistant response */}
              {streamingState.assistantContent && (
                <Message
                  role="assistant"
                  content={streamingState.assistantContent}
                  isStreaming={isLoading}
                />
              )}
            </>
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
