import { useEffect, useRef, useState, useCallback } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Message, ToolCall, ToolCallsGroup } from './message'
import { MessageInput, type UploadedFile } from './message-input'
import { api, type Conversation, type SSEEvent } from '@/lib/api'
import { Loader2, Zap, Download, FileText, FileSpreadsheet, FileImage, File, Maximize2, Minimize2, ExternalLink } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface ChatViewProps {
  conversation: Conversation | null
  onConversationUpdate: () => void
  onManageIntegrations?: () => void
  onCreateAndSend?: (content: string, files?: UploadedFile[]) => Promise<Conversation>
}

interface IntegrationTool {
  id: string
  name: string
  connected: boolean
  enabled: boolean
}

interface FileArtifact {
  filename: string
  mime_type: string
  size: number
  data: string  // base64 encoded
  description: string
}

// Helper to get icon for file type
function getFileIcon(mimeType: string) {
  if (mimeType.startsWith('image/')) return FileImage
  if (mimeType.includes('spreadsheet') || mimeType.includes('csv') || mimeType.includes('excel')) return FileSpreadsheet
  if (mimeType.includes('text') || mimeType.includes('json') || mimeType.includes('pdf')) return FileText
  return File
}

// Helper to format file size
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// File download/preview component
function FileDownload({ file }: { file: FileArtifact }) {
  const Icon = getFileIcon(file.mime_type)
  const [isExpanded, setIsExpanded] = useState(false)
  
  const handleDownload = () => {
    // Create blob from base64 data
    const byteCharacters = atob(file.data)
    const byteNumbers = new Array(byteCharacters.length)
    for (let i = 0; i < byteCharacters.length; i++) {
      byteNumbers[i] = byteCharacters.charCodeAt(i)
    }
    const byteArray = new Uint8Array(byteNumbers)
    const blob = new Blob([byteArray], { type: file.mime_type })
    
    // Create download link
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = file.filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }
  
  const isImage = file.mime_type.startsWith('image/')
  const isPdf = file.mime_type === 'application/pdf'
  const isSvg = file.mime_type === 'image/svg+xml'
  
  // For images, render inline with expand option
  if (isImage) {
    const dataUrl = isSvg 
      ? `data:image/svg+xml;base64,${file.data}`
      : `data:${file.mime_type};base64,${file.data}`
    
    return (
      <div className={cn(
        "rounded-lg border bg-muted/30 overflow-hidden",
        isExpanded ? "fixed inset-4 z-50 bg-background" : "max-w-lg"
      )}>
        {/* Image header */}
        <div className="flex items-center justify-between p-2 border-b bg-background/80">
          <div className="flex items-center gap-2">
            <Icon className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium truncate">{file.filename}</span>
            <span className="text-xs text-muted-foreground">{formatSize(file.size)}</span>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 cursor-pointer"
              onClick={() => setIsExpanded(!isExpanded)}
              title={isExpanded ? "Minimize" : "Expand"}
            >
              {isExpanded ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 cursor-pointer"
              onClick={handleDownload}
              title="Download"
            >
              <Download className="h-4 w-4" />
            </Button>
          </div>
        </div>
        {/* Image preview */}
        <div className={cn(
          "flex items-center justify-center p-2",
          isExpanded ? "h-[calc(100%-48px)]" : "max-h-96"
        )}>
          <img 
            src={dataUrl} 
            alt={file.description || file.filename}
            className={cn(
              "max-w-full rounded",
              isExpanded ? "max-h-full object-contain" : "max-h-80"
            )}
          />
        </div>
      </div>
    )
  }
  
  // For PDFs, show a preview link option
  if (isPdf) {
    const dataUrl = `data:application/pdf;base64,${file.data}`
    
    return (
      <div className="rounded-lg border bg-muted/30 overflow-hidden max-w-md">
        <div className="flex items-center gap-3 p-3">
          <div className="flex-shrink-0 p-2 rounded-md bg-red-500/10">
            <FileText className="h-5 w-5 text-red-500" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">{file.filename}</p>
            <p className="text-xs text-muted-foreground">
              {formatSize(file.size)} • PDF Document
            </p>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.open(dataUrl, '_blank')}
              className="flex-shrink-0 cursor-pointer"
            >
              <ExternalLink className="h-4 w-4 mr-1" />
              Open
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 cursor-pointer"
              onClick={handleDownload}
            >
              <Download className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    )
  }
  
  // Default file download UI
  return (
    <div className="flex items-center gap-3 p-3 rounded-lg border bg-muted/30 max-w-md">
      <div className="flex-shrink-0 p-2 rounded-md bg-primary/10">
        <Icon className="h-5 w-5 text-primary" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{file.filename}</p>
        <p className="text-xs text-muted-foreground">
          {formatSize(file.size)} • {file.description || 'Ready for download'}
        </p>
      </div>
      <Button
        variant="outline"
        size="sm"
        onClick={handleDownload}
        className="flex-shrink-0 cursor-pointer"
      >
        <Download className="h-4 w-4 mr-1" />
        Download
      </Button>
    </div>
  )
}

interface StreamingState {
  status?: string
  toolCalls: Map<string, { name: string; arguments: string; result?: string; isExecuting: boolean }>
  assistantContent: string
  userMessage?: string  // The message the user just sent
  files: FileArtifact[]  // Files returned by the agent
}

// Type for pending messages (exchanges not yet persisted to conversation.messages)
interface PendingMessage {
  userMessage: string
  assistantContent: string
  toolCalls: Array<{ id: string; name: string; arguments: string; result?: string }>
  files: FileArtifact[]  // Files generated during this exchange
}

export function ChatView({ conversation, onConversationUpdate, onManageIntegrations, onCreateAndSend }: ChatViewProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [streamingState, setStreamingState] = useState<StreamingState | null>(null)
  const [integrations, setIntegrations] = useState<IntegrationTool[]>([])
  const [integrationsLoaded, setIntegrationsLoaded] = useState(false)
  const [quickStartValue, setQuickStartValue] = useState('')
  // Keep files separately so they persist after streaming state is cleared
  const [sessionFiles, setSessionFiles] = useState<FileArtifact[]>([])
  // Track pending messages (exchanges not yet persisted to conversation.messages)
  const [pendingMessages, setPendingMessages] = useState<PendingMessage[]>([])
  const scrollRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<(() => void) | null>(null)
  const prevConvIdRef = useRef<string | undefined>(undefined)

  // Clear streaming state when conversation gets messages (after refresh)
  // Files are now added directly to sessionFiles in the event handler
  useEffect(() => {
    if (conversation && conversation.messages && conversation.messages.length > 0 && streamingState && !isLoading) {
      console.log('[ChatView] Clearing streaming state - conversation has messages, sessionFiles:', sessionFiles.length)
      setStreamingState(null)
    }
  }, [conversation, conversation?.messages?.length, streamingState, isLoading, sessionFiles.length])
  
  // Clear session files and pending messages when conversation changes
  useEffect(() => {
    if (conversation?.id !== prevConvIdRef.current) {
      console.log('[ChatView] Conversation ID changed, clearing sessionFiles and pendingMessages. Old:', prevConvIdRef.current, 'New:', conversation?.id)
      setSessionFiles([])
      setPendingMessages([])
      prevConvIdRef.current = conversation?.id
    }
  }, [conversation?.id])

  // Fetch integrations once on mount
  useEffect(() => {
    if (integrationsLoaded) return
    
    const fetchIntegrations = async () => {
      try {
        const data = await api.listIntegrations()
        const tools: IntegrationTool[] = (data.integrations || []).map((int: any) => ({
          id: int.id,
          name: int.name,
          connected: int.connected || false,
          // Default: all connected tools are enabled
          enabled: int.connected || false,
        }))
        setIntegrations(tools)
        setIntegrationsLoaded(true)
      } catch (err) {
        console.error('Failed to fetch integrations:', err)
      }
    }
    fetchIntegrations()
  }, [integrationsLoaded])

  const handleToolToggle = useCallback(async (toolId: string, enabled: boolean) => {
    // Update local state immediately for responsive UI
    setIntegrations(prev => prev.map(t => 
      t.id === toolId ? { ...t, enabled } : t
    ))
    
    // Persist to backend if we have a conversation
    if (conversation) {
      try {
        const currentEnabled = integrations.filter(t => t.enabled).map(t => t.id)
        const newEnabledTools = enabled
          ? [...currentEnabled, toolId]
          : currentEnabled.filter(id => id !== toolId)
        
        await api.setConversationTools(conversation.id, newEnabledTools)
      } catch (err) {
        console.error('Failed to toggle tool:', err)
        // Revert on error
        setIntegrations(prev => prev.map(t => 
          t.id === toolId ? { ...t, enabled: !enabled } : t
        ))
      }
    }
  }, [conversation, integrations])

  useEffect(() => {
    // Scroll to bottom when messages change
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [conversation?.messages, streamingState])

  const handleSend = async (content: string, files?: UploadedFile[]) => {
    if (!conversation) return

    // Save current streaming exchange to pending if it has content
    if (streamingState?.userMessage && streamingState?.assistantContent) {
      console.log('[ChatView] Saving current exchange to pendingMessages before new send, files:', sessionFiles.length)
      setPendingMessages(prev => [...prev, {
        userMessage: streamingState.userMessage!,
        assistantContent: streamingState.assistantContent,
        toolCalls: Array.from(streamingState.toolCalls.entries()).map(([id, tc]) => ({
          id,
          name: tc.name,
          arguments: tc.arguments,
          result: tc.result,
        })),
        files: [...sessionFiles],  // Include files from this exchange
      }])
      // Clear sessionFiles since they're now part of the pending message
      setSessionFiles([])
    }

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
      files: [],  // Files returned by agent
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

      case 'file':
        console.log('[ChatView] file event received:', event.data)
        const fileData = event.data as unknown as FileArtifact
        console.log('[ChatView] Adding file to sessionFiles:', fileData.filename, fileData.size, 'bytes')
        // Only add to sessionFiles - don't duplicate in streamingState
        setSessionFiles(prev => [...prev, fileData])
        setStreamingState((prev) => prev ? {
          ...prev,
          status: `File ready: ${fileData.filename}`,
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
        // Refresh conversation - this will update messages
        onConversationUpdate()
        // Clear pending messages since conversation will be refreshed with all messages
        setPendingMessages([])
        // Don't clear streaming state immediately - let it persist
        // It will be cleared when user sends another message or component remounts
        // This prevents the blank screen issue when conversation.messages is empty
        break
    }
  }

  // Handle sending message when no conversation exists (new chat)
  const handleNewChatSend = async (content: string, files?: UploadedFile[]) => {
    if (!onCreateAndSend) return

    // Reset quick start value
    setQuickStartValue('')

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
      status: 'Creating conversation...',
      toolCalls: new Map(),
      assistantContent: '',
      userMessage,
      files: [],
    })

    try {
      // Create the conversation first
      const newConv = await onCreateAndSend(content, files)
      
      // Now send the message to the new conversation
      setStreamingState(prev => prev ? { ...prev, status: files?.length ? 'Uploading files...' : 'Sending...' } : null)
      
      abortRef.current = api.sendMessage(newConv.id, content, (event: SSEEvent) => {
        handleSSEEvent(event)
      }, files)
    } catch (err) {
      console.error('Failed to create conversation and send message:', err)
      setIsLoading(false)
      setStreamingState(null)
    }
  }

  // New chat state - show welcome screen with input
  // BUT if we're streaming (after sending first message), show streaming UI instead
  if (!conversation && !streamingState) {
    return (
      <div className="flex h-full flex-col min-w-0 overflow-hidden">
        {/* Empty scrollable area with welcome message */}
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center max-w-md px-4">
            <div className="mb-6">
              <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-primary/10 mb-4">
                <Zap className="h-8 w-8 text-primary" />
              </div>
              <h2 className="text-2xl font-semibold mb-2">How can I help you today?</h2>
              <p className="text-muted-foreground">
                Ask me to analyze data, write SQL queries, create visualizations, or explore your connected databases.
              </p>
            </div>
                    <div className="flex flex-wrap gap-2 justify-center">
                      <button
                        className="px-3 py-1.5 text-sm rounded-full border bg-background hover:bg-muted transition-colors cursor-pointer"
                        onClick={() => setQuickStartValue('Show me the tables in my database')}
                      >
                        Show database tables
                      </button>
                      <button 
                        className="px-3 py-1.5 text-sm rounded-full border bg-background hover:bg-muted transition-colors cursor-pointer"
                        onClick={() => setQuickStartValue('Analyze and visualize trends in my data')}
                      >
                        Analyze data trends
                      </button>
                      <button 
                        className="px-3 py-1.5 text-sm rounded-full border bg-background hover:bg-muted transition-colors cursor-pointer"
                        onClick={() => setQuickStartValue('Help me write an optimized SQL query')}
                      >
                        Write SQL query
                      </button>
                    </div>
          </div>
        </div>

        {/* Message input */}
        <MessageInput
          onSend={handleNewChatSend}
          disabled={isLoading}
          isLoading={isLoading}
          tools={integrations}
          onToolToggle={handleToolToggle}
          onManageIntegrations={onManageIntegrations}
          initialValue={quickStartValue}
        />
      </div>
    )
  }
  
  // If streaming but no conversation yet (new chat in progress), show streaming UI
  if (!conversation && streamingState) {
    return (
      <div className="flex h-full flex-col min-w-0 overflow-hidden">
        <div className="border-b px-4 py-3">
          <h2 className="font-semibold truncate">New conversation</h2>
        </div>

        <ScrollArea className="flex-1 overflow-hidden" ref={scrollRef}>
          <div className="divide-y divide-border/40 overflow-hidden">
            {/* Streaming state for new chat */}
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

              {/* Assistant response as it streams */}
              {streamingState.assistantContent && (
                <Message
                  role="assistant"
                  content={streamingState.assistantContent}
                />
              )}
            </div>
          </div>
        </ScrollArea>

        <MessageInput
          onSend={handleNewChatSend}
          disabled={isLoading}
          isLoading={isLoading}
          tools={integrations}
          onToolToggle={handleToolToggle}
          onManageIntegrations={onManageIntegrations}
        />
      </div>
    )
  }

  // At this point, conversation must exist
  if (!conversation) return null

  return (
    <div className="flex h-full flex-col min-w-0 overflow-hidden">
      <div className="border-b px-4 py-3">
        <h2 className="font-semibold truncate">
          {conversation.title || 'New conversation'}
        </h2>
        {conversation.sandboxId && (
          <p className="text-xs text-muted-foreground">
            Sandbox: {conversation.sandboxId}
          </p>
        )}
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

          {/* Pending messages (exchanges not yet persisted to conversation.messages) */}
          {pendingMessages.map((pending, idx) => (
            <div key={`pending-${idx}`}>
              <Message role="user" content={pending.userMessage} />
              {pending.toolCalls.length > 0 && (
                <ToolCallsGroup count={pending.toolCalls.length}>
                  {pending.toolCalls.map((tc) => (
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
              <Message role="assistant" content={pending.assistantContent} />
              {/* Files generated during this exchange */}
              {pending.files.length > 0 && (
                <div className="px-4 py-2 space-y-2">
                  {pending.files.map((file, fileIdx) => (
                    <FileDownload key={`pending-file-${fileIdx}`} file={file} />
                  ))}
                </div>
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
          
          {/* Files generated during current session - render after streaming or messages */}
          {sessionFiles.length > 0 && (
            <div className="px-4 py-2 space-y-2">
              {sessionFiles.map((file, idx) => (
                <FileDownload key={`session-${idx}`} file={file} />
              ))}
            </div>
          )}
        </div>
      </ScrollArea>

      <MessageInput
        onSend={handleSend}
        disabled={isLoading}
        isLoading={isLoading}
        tools={integrations}
        onToolToggle={handleToolToggle}
        onManageIntegrations={onManageIntegrations}
      />
    </div>
  )
}
