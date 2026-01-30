import { useState } from 'react'
import DOMPurify from 'dompurify'
import { User, Bot, Wrench, Loader2, ChevronDown, ChevronRight, CheckCircle2, XCircle, Clock } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface MessageProps {
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  isStreaming?: boolean
}

export function Message({ role, content, isStreaming }: MessageProps) {
  const isUser = role === 'user'
  const isTool = role === 'tool'

  // Parse markdown-style formatting in assistant messages
  const formattedContent = !isUser && content ? DOMPurify.sanitize(formatMarkdown(content)) : ''

  return (
    <div
      className={cn(
        "flex gap-3 p-4",
        isUser && "bg-muted/50"
      )}
    >
      <div
        className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
          isUser ? "bg-primary text-primary-foreground" : "bg-muted"
        )}
      >
        {isUser ? (
          <User className="h-4 w-4" />
        ) : isTool ? (
          <Wrench className="h-4 w-4" />
        ) : (
          <Bot className="h-4 w-4" />
        )}
      </div>
      <div className="flex-1 space-y-2 min-w-0">
        <div className="text-sm font-medium">
          {isUser ? 'You' : isTool ? 'Tool' : 'Assistant'}
        </div>
        <div className="prose prose-sm dark:prose-invert max-w-none">
          {isUser ? (
            <div className="whitespace-pre-wrap break-words">
              {content}
            </div>
          ) : formattedContent ? (
            <div 
              className="whitespace-pre-wrap break-words"
              dangerouslySetInnerHTML={{ __html: formattedContent }}
            />
          ) : isStreaming ? (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-sm">Thinking...</span>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}

interface ToolCallProps {
  name: string
  arguments: string
  result?: string
  isExecuting?: boolean
}

export function ToolCall({ name, arguments: args, result, isExecuting }: ToolCallProps) {
  const [isExpanded, setIsExpanded] = useState(true)
  const [showFullResult, setShowFullResult] = useState(false)
  
  const parsedArgs = formatJSON(args)
  const parsedResult = result ? parseToolResultPayload(result) : null
  const resultContent = parsedResult?.content ?? result ?? ''
  const statusContent = resultContent.trim()
  const isSuccess = result
    ? !(parsedResult?.isError ?? false) && !/^error\b/i.test(statusContent)
    : false
  const isLongResult = resultContent.length > 500

  // Get a friendly name for the tool
  const friendlyName = getToolDisplayName(name)
  const appName = getAppFromTool(name)

  return (
    <div className={cn(
      "mx-4 my-3 rounded-lg border overflow-hidden transition-all",
      isExecuting ? "border-blue-500/50 bg-blue-500/5" : 
      result ? (isSuccess ? "border-green-500/30 bg-green-500/5" : "border-red-500/30 bg-red-500/5") :
      "border-border bg-muted/30"
    )}>
      {/* Header */}
      <div 
        className="flex items-center gap-2 p-3 cursor-pointer hover:bg-muted/50 transition-colors"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <Button variant="ghost" size="sm" className="h-6 w-6 p-0">
          {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </Button>
        
        <div className={cn(
          "flex h-6 w-6 items-center justify-center rounded",
          isExecuting ? "bg-blue-500/20 text-blue-600" :
          result ? (isSuccess ? "bg-green-500/20 text-green-600" : "bg-red-500/20 text-red-600") :
          "bg-muted text-muted-foreground"
        )}>
          {isExecuting ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : result ? (
            isSuccess ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />
          ) : (
            <Clock className="h-3.5 w-3.5" />
          )}
        </div>
        
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm">{friendlyName}</span>
            {appName && (
              <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                {appName}
              </span>
            )}
          </div>
        </div>
        
        {isExecuting && (
          <span className="text-xs text-blue-600">Running...</span>
        )}
      </div>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t px-3 pb-3">
          {/* Arguments */}
          <div className="mt-3">
            <div className="text-xs font-medium text-muted-foreground mb-1.5 flex items-center gap-1">
              <Wrench className="h-3 w-3" />
              Input Parameters
            </div>
            <pre className="text-xs overflow-x-auto rounded-md bg-background/80 p-2.5 border">
              {parsedArgs}
            </pre>
          </div>

          {/* Result */}
          {result && (
            <div className="mt-3">
              <div className="text-xs font-medium text-muted-foreground mb-1.5 flex items-center justify-between">
                <span className="flex items-center gap-1">
                  {isSuccess ? <CheckCircle2 className="h-3 w-3 text-green-600" /> : <XCircle className="h-3 w-3 text-red-600" />}
                  {isSuccess ? 'Result' : 'Error'}
                </span>
                {isLongResult && (
                  <Button 
                    variant="ghost" 
                    size="sm" 
                    className="h-5 text-xs"
                    onClick={(e) => {
                      e.stopPropagation()
                      setShowFullResult(!showFullResult)
                    }}
                  >
                    {showFullResult ? 'Show Less' : 'Show Full'}
                  </Button>
                )}
              </div>
              <pre className={cn(
                "text-xs overflow-auto rounded-md bg-background/80 p-2.5 border whitespace-pre-wrap break-words",
                !showFullResult && isLongResult && "max-h-48"
              )}>
                {formatResultContent(resultContent)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// Helper functions
function formatJSON(str: string): string {
  try {
    return JSON.stringify(JSON.parse(str), null, 2)
  } catch {
    return str
  }
}

function parseToolResultPayload(result: string): { content: string; isError: boolean } | null {
  try {
    const parsed = JSON.parse(result)
    if (
      parsed &&
      typeof parsed === 'object' &&
      typeof parsed.content === 'string' &&
      typeof parsed.isError === 'boolean'
    ) {
      return { content: parsed.content, isError: parsed.isError }
    }
  } catch {
    // Ignore parse errors and fall back to raw result.
  }
  return null
}

function getToolDisplayName(name: string): string {
  // Convert tool names like "list_app_tools" or "github-create-issue" to friendly names
  const cleanName = name
    .replace(/^(list_app_tools|call_app_tool|list_connected_apps)$/, (match) => {
      switch (match) {
        case 'list_app_tools': return 'List Available Tools'
        case 'call_app_tool': return 'Call Tool'
        case 'list_connected_apps': return 'List Connected Apps'
        default: return match
      }
    })
    .replace(/-/g, ' ')
    .replace(/_/g, ' ')
    .split(' ')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
  
  return cleanName
}

function getAppFromTool(name: string): string | null {
  // Extract app name from tool names like "github-create-issue"
  const parts = name.split('-')
  if (parts.length > 1 && !['list', 'call'].includes(parts[0])) {
    return parts[0].charAt(0).toUpperCase() + parts[0].slice(1)
  }
  return null
}

function formatResultContent(result: string): string {
  // Try to detect and format JSON in results
  try {
    // Check if it's wrapped in markdown code blocks
    const jsonMatch = result.match(/```json\n?([\s\S]*?)\n?```/)
    if (jsonMatch) {
      const jsonContent = JSON.parse(jsonMatch[1])
      return JSON.stringify(jsonContent, null, 2)
    }
    
    // Try to parse as JSON directly
    const parsed = JSON.parse(result)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return result
  }
}

function formatMarkdown(content: string): string {
  // Basic markdown formatting
  return content
    // Code blocks
    .replace(/```(\w*)\n?([\s\S]*?)```/g, '<pre class="bg-muted rounded-md p-3 my-2 overflow-x-auto text-sm"><code>$2</code></pre>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code class="bg-muted px-1 py-0.5 rounded text-sm">$1</code>')
    // Bold
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    // Headers
    .replace(/^### (.+)$/gm, '<h3 class="text-base font-semibold mt-3 mb-1">$1</h3>')
    .replace(/^## (.+)$/gm, '<h2 class="text-lg font-semibold mt-4 mb-2">$1</h2>')
    // Lists
    .replace(/^- (.+)$/gm, '<li class="ml-4">$1</li>')
    // Line breaks
    .replace(/\n\n/g, '</p><p class="mb-2">')
}
