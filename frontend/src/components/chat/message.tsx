import { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { User, Bot, Wrench, Loader2, ChevronRight, CheckCircle2, XCircle, Clock, Copy, Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { VegaChart, parseVegaSpec } from './vega-chart'
import { MermaidDiagram } from './mermaid-diagram'

interface MessageProps {
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  isStreaming?: boolean
}

export function Message({ role, content, isStreaming }: MessageProps) {
  const isUser = role === 'user'
  const isTool = role === 'tool'

  return (
    <div
      className={cn(
        "flex gap-3 p-4 overflow-hidden",
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
      <div className="flex-1 space-y-2 min-w-0 overflow-hidden w-full">
        <div className="text-sm font-medium">
          {isUser ? 'You' : isTool ? 'Tool' : 'Assistant'}
        </div>
        <div className="prose prose-sm dark:prose-invert max-w-full overflow-x-auto w-full">
          {isUser ? (
            <div className="whitespace-pre-wrap break-words">
              {content}
            </div>
          ) : content ? (
            <MarkdownContent content={content} />
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

/**
 * Renders markdown content with support for Vega charts, code highlighting, etc.
 */
function MarkdownContent({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeRaw]}
      components={{
        // Custom code block renderer with Vega chart support
        code({ className, children, ...props }) {
          // Extract language from className (e.g., "language-javascript" -> "javascript")
          const match = /language-([\w-]+)/.exec(className || '')
          const language = match ? match[1] : ''
          const codeContent = String(children).replace(/\n$/, '')
          
          // Determine if this is inline code or a code block
          // Inline code: no language specified AND no newlines AND short content
          // This is a heuristic since react-markdown doesn't clearly distinguish inline vs block
          const isInline = !className && !codeContent.includes('\n') && codeContent.length < 100
          
          if (isInline) {
            return (
              <code className="bg-muted px-1.5 py-0.5 rounded text-sm font-mono" {...props}>
                {children}
              </code>
            )
          }

          // Try to parse as Vega spec (supports vega, vega-lite, vegalite)
          const vegaSpec = parseVegaSpec(codeContent, language)
          if (vegaSpec) {
            return <VegaChart spec={vegaSpec} />
          }

          // Render Mermaid diagrams
          if (language === 'mermaid') {
            return <MermaidDiagram code={codeContent} />
          }

          // Regular code block with syntax highlighting
          return (
            <CodeBlock language={language} code={codeContent} />
          )
        },
        // Custom table styling
        table({ children }) {
          return (
            <div className="overflow-x-auto my-4">
              <table className="min-w-full border-collapse border border-border">
                {children}
              </table>
            </div>
          )
        },
        th({ children }) {
          return (
            <th className="border border-border bg-muted px-3 py-2 text-left font-semibold">
              {children}
            </th>
          )
        },
        td({ children }) {
          return (
            <td className="border border-border px-3 py-2">
              {children}
            </td>
          )
        },
        // Custom link styling
        a({ href, children }) {
          return (
            <a 
              href={href} 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              {children}
            </a>
          )
        },
        // Custom blockquote styling
        blockquote({ children }) {
          return (
            <blockquote className="border-l-4 border-primary/50 pl-4 italic text-muted-foreground my-4">
              {children}
            </blockquote>
          )
        },
        // Custom heading styles
        h1({ children }) {
          return <h1 className="text-2xl font-bold mt-6 mb-3">{children}</h1>
        },
        h2({ children }) {
          return <h2 className="text-xl font-semibold mt-5 mb-2">{children}</h2>
        },
        h3({ children }) {
          return <h3 className="text-lg font-semibold mt-4 mb-2">{children}</h3>
        },
        // Custom list styling  
        ul({ children }) {
          return <ul className="list-disc pl-6 my-2 space-y-1">{children}</ul>
        },
        ol({ children }) {
          return <ol className="list-decimal pl-6 my-2 space-y-1">{children}</ol>
        },
        // Paragraph styling
        p({ children }) {
          return <p className="mb-3 last:mb-0">{children}</p>
        },
      }}
    >
      {content}
    </ReactMarkdown>
  )
}

/**
 * Code block with syntax highlighting and copy button
 */
function CodeBlock({ language, code }: { language: string; code: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="relative group my-3 max-w-full overflow-hidden">
      {/* Language badge and copy button */}
      <div className="absolute top-0 right-0 flex items-center gap-1 p-1.5 z-10">
        {language && (
          <span className="text-xs text-muted-foreground bg-muted/80 px-2 py-0.5 rounded">
            {language}
          </span>
        )}
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity"
          onClick={handleCopy}
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-green-500" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
        </Button>
      </div>
      
      <SyntaxHighlighter
        style={oneDark}
        language={language || 'text'}
        PreTag="div"
        customStyle={{
          margin: 0,
          borderRadius: '0.5rem',
          fontSize: '0.875rem',
          maxWidth: '100%',
          overflowX: 'auto',
        }}
        wrapLongLines={false}
      >
        {code}
      </SyntaxHighlighter>
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
  // Collapsed by default, auto-expand only while executing
  const [isExpanded, setIsExpanded] = useState(false)
  const [showFullResult, setShowFullResult] = useState(false)
  
  // Parse arguments and check if this is an execute_command
  const isCommand = name === 'execute_command'
  const parsedArgsObj = tryParseJSON(args)
  const commandText = isCommand && parsedArgsObj?.command && typeof parsedArgsObj.command === 'string' 
    ? parsedArgsObj.command 
    : null
  const parsedArgs = commandText ? commandText : formatJSON(args)
  const parsedResult = result ? parseToolResultPayload(result) : null
  const resultContent = parsedResult?.content ?? result ?? ''
  const statusContent = resultContent.trim()
  const isSuccess = result
    ? !(parsedResult?.isError ?? false) && !/^error\b/i.test(statusContent)
    : false
  const isLongResult = resultContent.length > 300

  // Get a friendly name for the tool
  const friendlyName = getToolDisplayName(name)
  const appName = getAppFromTool(name)
  
  // Get a brief summary of the result for collapsed view
  const resultPreview = resultContent.slice(0, 60).replace(/\n/g, ' ') + (resultContent.length > 60 ? '...' : '')

  return (
    <div className={cn(
      "mx-3 my-1.5 rounded-md border text-xs transition-all duration-200",
      isExecuting && "border-blue-500/40 bg-gradient-to-r from-blue-500/5 to-transparent animate-pulse",
      !isExecuting && result && isSuccess && "border-emerald-500/20 bg-emerald-500/[0.02]",
      !isExecuting && result && !isSuccess && "border-red-500/20 bg-red-500/[0.02]",
      !isExecuting && !result && "border-border/50 bg-muted/20"
    )}>
      {/* Compact Header */}
      <div 
        className={cn(
          "flex items-center gap-1.5 px-2.5 py-1.5 cursor-pointer select-none transition-colors",
          "hover:bg-muted/30"
        )}
        onClick={() => setIsExpanded(!isExpanded)}
      >
        {/* Expand/Collapse indicator */}
        <ChevronRight className={cn(
          "h-3 w-3 text-muted-foreground/60 transition-transform duration-200 flex-shrink-0",
          isExpanded && "rotate-90"
        )} />
        
        {/* Status indicator */}
        <div className={cn(
          "flex h-4 w-4 items-center justify-center rounded-sm flex-shrink-0",
          isExecuting && "text-blue-500",
          !isExecuting && result && isSuccess && "text-emerald-500",
          !isExecuting && result && !isSuccess && "text-red-500",
          !isExecuting && !result && "text-muted-foreground/50"
        )}>
          {isExecuting ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : result ? (
            isSuccess ? <CheckCircle2 className="h-3 w-3" /> : <XCircle className="h-3 w-3" />
          ) : (
            <Clock className="h-3 w-3" />
          )}
        </div>
        
        {/* Tool name and app badge */}
        <div className="flex items-center gap-1.5 min-w-0 flex-1">
          <span className={cn(
            "font-medium truncate",
            isExecuting && "text-blue-600",
            !isExecuting && result && !isSuccess && "text-red-600"
          )}>
            {friendlyName}
          </span>
          {appName && (
            <span className="text-[10px] px-1 py-0.5 rounded bg-muted/60 text-muted-foreground/70 flex-shrink-0">
              {appName}
            </span>
          )}
        </div>
        
        {/* Status text or result preview */}
        <div className="flex items-center gap-1.5 text-muted-foreground/60 flex-shrink-0">
          {isExecuting ? (
            <span className="text-blue-500 text-[10px] font-medium">Running</span>
          ) : result && !isExpanded ? (
            <span className="text-[10px] max-w-[120px] truncate hidden sm:block">{resultPreview}</span>
          ) : null}
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-border/30 px-2.5 py-2 space-y-2">
          {/* Arguments - compact view */}
          <div>
            <div className="text-[10px] uppercase tracking-wide font-medium text-muted-foreground/50 mb-1">
              {commandText ? 'Command' : 'Parameters'}
            </div>
            {commandText ? (
              <div className="relative rounded bg-zinc-900 text-zinc-100 px-3 py-2 font-mono text-[11px] overflow-x-auto">
                <span className="text-emerald-400 select-none">$ </span>
                <span>{commandText}</span>
              </div>
            ) : (
              <pre className="text-[11px] leading-relaxed overflow-x-auto rounded bg-muted/30 px-2 py-1.5 text-muted-foreground/80 max-h-24 overflow-y-auto">
                {parsedArgs}
              </pre>
            )}
          </div>

          {/* Result - compact view */}
          {result && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <div className={cn(
                  "text-[10px] uppercase tracking-wide font-medium",
                  isSuccess ? "text-emerald-500/70" : "text-red-500/70"
                )}>
                  {isSuccess ? 'Output' : 'Error'}
                </div>
                {isLongResult && (
                  <button 
                    className="text-[10px] text-muted-foreground/50 hover:text-muted-foreground transition-colors"
                    onClick={(e) => {
                      e.stopPropagation()
                      setShowFullResult(!showFullResult)
                    }}
                  >
                    {showFullResult ? 'Collapse' : 'Expand'}
                  </button>
                )}
              </div>
              <pre className={cn(
                "text-[11px] leading-relaxed overflow-auto rounded px-2 py-1.5 whitespace-pre-wrap break-words",
                isSuccess ? "bg-emerald-500/5 text-muted-foreground/80" : "bg-red-500/5 text-red-400/80",
                !showFullResult && isLongResult && "max-h-20",
                showFullResult && "max-h-64"
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

/**
 * Container for grouping multiple tool calls with a header
 */
interface ToolCallsGroupProps {
  children: React.ReactNode
  count?: number
}

export function ToolCallsGroup({ children, count }: ToolCallsGroupProps) {
  const [isCollapsed, setIsCollapsed] = useState(false)
  
  if (!count || count <= 1) {
    return <>{children}</>
  }

  return (
    <div className="my-2">
      <button
        onClick={() => setIsCollapsed(!isCollapsed)}
        className="flex items-center gap-1.5 px-3 py-1 text-[10px] text-muted-foreground/60 hover:text-muted-foreground transition-colors"
      >
        <ChevronRight className={cn(
          "h-3 w-3 transition-transform",
          !isCollapsed && "rotate-90"
        )} />
        <Wrench className="h-3 w-3" />
        <span className="font-medium">{count} tool calls</span>
      </button>
      {!isCollapsed && children}
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

function tryParseJSON(str: string): Record<string, unknown> | null {
  try {
    return JSON.parse(str)
  } catch {
    return null
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
  // Special case mappings
  const specialNames: Record<string, string> = {
    'execute_command': 'Run Command',
    'list_app_tools': 'List Available Tools',
    'call_app_tool': 'Call Tool',
    'list_connected_apps': 'List Connected Apps',
    'write_file': 'Write File',
    'read_file': 'Read File',
  }
  
  if (specialNames[name]) {
    return specialNames[name]
  }
  
  // Convert tool names like "github-create-issue" to friendly names
  const cleanName = name
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
