import { useEffect, useRef, useState } from 'react'
import mermaid from 'mermaid'
import { Loader2, AlertCircle, Maximize2, Minimize2, Download, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

// Initialize mermaid with dark/light theme support
mermaid.initialize({
  startOnLoad: false,
  theme: 'neutral',
  securityLevel: 'loose',
  fontFamily: 'system-ui, -apple-system, sans-serif',
})

/**
 * Preprocess mermaid code to fix common issues:
 * - Replace pipe characters in node labels with safer alternatives
 * - Handle special characters that break mermaid parsing
 * - Fix common LLM-generated syntax issues
 */
function preprocessMermaidCode(code: string): string {
  let processed = code.trim()
  
  // Process line by line for better control
  const lines = processed.split('\n')
  const processedLines = lines.map(line => {
    // Skip lines that are subgraph declarations, direction, or comments
    if (line.trim().startsWith('subgraph') || 
        line.trim().startsWith('end') ||
        line.trim().startsWith('%%') ||
        line.trim().startsWith('direction') ||
        line.trim().match(/^(graph|flowchart|sequenceDiagram|classDiagram|stateDiagram|erDiagram|gantt|pie)/)) {
      // For subgraph lines, still need to fix the label if present
      if (line.includes('[')) {
        return line.replace(/\[([^\]]*)\]/g, (match, content) => {
          const fixed = content
            .replace(/\|/g, ' · ')  // Replace all pipes
            .replace(/"/g, "'")     // Replace double quotes with single
          return `[${fixed}]`
        })
      }
      return line
    }
    
    // Fix node labels in square brackets: [label]
    line = line.replace(/\[([^\]]*)\]/g, (match, content) => {
      const fixed = content
        .replace(/\|/g, ' · ')      // Replace all pipe characters
        .replace(/"/g, "'")         // Replace double quotes
        .replace(/</g, '‹')         // Replace < with similar char
        .replace(/>/g, '›')         // Replace > with similar char
      return `[${fixed}]`
    })
    
    // Fix node labels in parentheses: (label)
    line = line.replace(/\(([^)]*)\)/g, (match, content) => {
      // Don't process if it looks like a function call or subgraph reference
      if (content.includes('-->') || content.includes('---') || content.length < 2) {
        return match
      }
      const fixed = content
        .replace(/\|/g, ' · ')
        .replace(/"/g, "'")
      return `(${fixed})`
    })
    
    // Fix node labels in curly braces: {label}
    line = line.replace(/\{([^}]*)\}/g, (match, content) => {
      const fixed = content
        .replace(/\|/g, ' · ')
        .replace(/"/g, "'")
      return `{${fixed}}`
    })
    
    // Fix link text: -->|text|
    // Mermaid uses |text| for link labels, but text shouldn't contain pipes
    line = line.replace(/-->\|([^|]*)\|/g, (match, content) => {
      const fixed = content.replace(/\|/g, ' · ')
      return `-->|${fixed}|`
    })
    
    return line
  })
  
  return processedLines.join('\n')
}

interface MermaidDiagramProps {
  code: string
  className?: string
}

export function MermaidDiagram({ code, className }: MermaidDiagramProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [svg, setSvg] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isExpanded, setIsExpanded] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [showRawCode, setShowRawCode] = useState(false)

  useEffect(() => {
    const renderDiagram = async () => {
      if (!code.trim()) {
        setError('No diagram code provided')
        setIsLoading(false)
        return
      }

      try {
        setIsLoading(true)
        setError(null)
        
        // Preprocess code to fix common issues
        const processedCode = preprocessMermaidCode(code)
        
        // Generate unique ID for this diagram
        const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2)}`
        
        // Render the diagram
        const { svg: renderedSvg } = await mermaid.render(id, processedCode)
        setSvg(renderedSvg)
      } catch (err) {
        console.error('Mermaid rendering error:', err)
        setError(err instanceof Error ? err.message : 'Failed to render diagram')
      } finally {
        setIsLoading(false)
      }
    }

    renderDiagram()
  }, [code])

  const handleDownload = () => {
    if (!svg) return
    
    const blob = new Blob([svg], { type: 'image/svg+xml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'diagram.svg'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return (
      <div className={cn(
        "flex items-center justify-center p-8 rounded-lg border bg-muted/30",
        className
      )}>
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        <span className="ml-2 text-sm text-muted-foreground">Rendering diagram...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className={cn(
        "rounded-lg border border-amber-500/50 bg-amber-500/10 p-4",
        className
      )}>
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2 text-amber-600">
            <AlertCircle className="h-4 w-4" />
            <span className="text-sm font-medium">Diagram Syntax Issue</span>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs cursor-pointer"
            onClick={() => setShowRawCode(!showRawCode)}
          >
            {showRawCode ? 'Hide code' : 'Show code'}
          </Button>
        </div>
        <p className="text-xs text-amber-700 mb-2">{error}</p>
        <p className="text-xs text-muted-foreground">
          Tip: Avoid using pipe characters (|) inside node labels. Use alternatives like "·" or line breaks (\n).
        </p>
        {showRawCode && (
          <pre className="mt-3 text-xs bg-muted p-3 rounded-md overflow-x-auto font-mono">
            {code}
          </pre>
        )}
      </div>
    )
  }

  return (
    <div
      className={cn(
        "relative rounded-lg border bg-background/50 my-3",
        isExpanded && "fixed inset-4 z-50 bg-background overflow-auto",
        className
      )}
    >
      {/* Toolbar */}
      <div className="absolute top-2 right-2 flex items-center gap-1 z-10">
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 bg-background/80 backdrop-blur-sm cursor-pointer"
          onClick={handleDownload}
          title="Download SVG"
        >
          <Download className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 bg-background/80 backdrop-blur-sm cursor-pointer"
          onClick={() => setIsExpanded(!isExpanded)}
          title={isExpanded ? "Minimize" : "Expand"}
        >
          {isExpanded ? (
            <Minimize2 className="h-3.5 w-3.5" />
          ) : (
            <Maximize2 className="h-3.5 w-3.5" />
          )}
        </Button>
      </div>

      {/* Diagram */}
      <div
        ref={containerRef}
        className={cn(
          "overflow-x-auto p-4",
          isExpanded && "h-full flex items-center justify-center"
        )}
        dangerouslySetInnerHTML={{ __html: svg || '' }}
      />
    </div>
  )
}
