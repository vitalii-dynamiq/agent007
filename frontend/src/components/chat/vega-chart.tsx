import { useEffect, useRef, useState } from 'react'
import embed, { type VisualizationSpec, type Result } from 'vega-embed'
import { cn } from '@/lib/utils'
import { AlertCircle, Maximize2, Minimize2, Download } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface VegaChartProps {
  spec: VisualizationSpec
  className?: string
}

export function VegaChart({ spec, className }: VegaChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<Result | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isExpanded, setIsExpanded] = useState(false)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const renderChart = async () => {
      try {
        setError(null)
        
        // Clean up previous view
        if (viewRef.current) {
          viewRef.current.finalize()
        }

        // Apply some default configurations for better appearance
        // Cast to any for width/height since not all spec types have these
        const specAny = spec as Record<string, unknown>
        
        // Use spec width if provided, otherwise calculate based on data
        // For bar charts with many categories, make it wide enough to see all bars
        const chartWidth = (specAny.width as number) || 1200

        const enhancedSpec: VisualizationSpec = {
          ...spec,
          width: chartWidth,
          // Default to a reasonable height
          height: (specAny.height as number) || 300,
          // Use a nice theme
          config: {
            background: 'transparent',
            axis: {
              labelColor: '#888',
              titleColor: '#aaa',
              gridColor: '#333',
              domainColor: '#555',
              tickColor: '#555',
            },
            legend: {
              labelColor: '#888',
              titleColor: '#aaa',
            },
            title: {
              color: '#ccc',
            },
            view: {
              stroke: 'transparent',
            },
            ...((spec.config as object) || {}),
          },
        } as VisualizationSpec

        viewRef.current = await embed(container, enhancedSpec, {
          actions: false, // We'll provide our own actions
          renderer: 'svg',
          theme: 'dark',
          tooltip: { theme: 'dark' }, // Enable tooltips with dark theme
        })
      } catch (err) {
        console.error('Vega embed error:', err)
        setError(err instanceof Error ? err.message : 'Failed to render chart')
      }
    }

    renderChart()

    return () => {
      if (viewRef.current) {
        viewRef.current.finalize()
      }
    }
  }, [spec])

  const handleDownload = async () => {
    if (!viewRef.current) return

    try {
      const url = await viewRef.current.view.toImageURL('png', 2)
      const link = document.createElement('a')
      link.download = 'chart.png'
      link.href = url
      link.click()
    } catch (err) {
      console.error('Download error:', err)
    }
  }

  if (error) {
    return (
      <div className={cn(
        "flex items-center gap-2 p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400",
        className
      )}>
        <AlertCircle className="h-4 w-4 flex-shrink-0" />
        <div className="text-sm">
          <p className="font-medium">Chart rendering error</p>
          <p className="text-xs opacity-80">{error}</p>
        </div>
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
      style={{ maxWidth: '100%' }}
    >
      {/* Chart toolbar */}
      <div className="absolute top-2 right-2 z-10 flex gap-1">
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0 opacity-50 hover:opacity-100"
          onClick={handleDownload}
          title="Download as PNG"
        >
          <Download className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0 opacity-50 hover:opacity-100"
          onClick={() => setIsExpanded(!isExpanded)}
          title={isExpanded ? "Minimize" : "Maximize"}
        >
          {isExpanded ? (
            <Minimize2 className="h-3.5 w-3.5" />
          ) : (
            <Maximize2 className="h-3.5 w-3.5" />
          )}
        </Button>
      </div>

      {/* Scrollable chart container */}
      <div 
        className="overflow-x-auto overflow-y-hidden p-4"
      >
        <div 
          ref={containerRef} 
          className={cn(
            "inline-block",
            isExpanded && "h-full flex items-center justify-center"
          )}
        />
      </div>
    </div>
  )
}

/**
 * Attempts to parse a code block content as a Vega or Vega-Lite specification.
 * Returns the parsed spec if valid, null otherwise.
 */
export function parseVegaSpec(content: string, language?: string): VisualizationSpec | null {
  // Check if the language hint suggests Vega
  // Supports: vega, vega-lite, vegalite (with or without hyphen, case-insensitive)
  const normalizedLang = language?.toLowerCase().replace('-', '')
  const isVegaHint = normalizedLang && ['vega', 'vegalite'].includes(normalizedLang)
  
  try {
    const parsed = JSON.parse(content)
    
    // Check if it looks like a Vega or Vega-Lite spec
    const isVegaSpec = (
      parsed &&
      typeof parsed === 'object' &&
      (
        // Vega-Lite markers
        parsed.$schema?.includes('vega-lite') ||
        parsed.mark ||
        parsed.layer ||
        parsed.hconcat ||
        parsed.vconcat ||
        parsed.concat ||
        parsed.facet ||
        parsed.repeat ||
        // Vega markers
        parsed.$schema?.includes('vega') ||
        parsed.signals ||
        parsed.scales ||
        parsed.marks
      )
    )

    // If it has a vega schema or language hint, or looks like a vega spec
    if (isVegaHint || isVegaSpec) {
      return parsed as VisualizationSpec
    }
  } catch {
    // Not valid JSON - could be partial JSON during streaming, or invalid
  }

  return null
}
