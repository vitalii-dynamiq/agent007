/**
 * Tool Selector Component
 * 
 * Elegant inline tool/integration selector for the chat input area.
 * Shows connected tools as icons and provides a dropdown to toggle them.
 */

import { useState, useRef, useEffect } from 'react'
import { 
  Plus, 
  Settings2, 
  Check, 
  ChevronRight,
  Database,
  Cloud,
  Github,
  Mail,
  Calendar,
  HardDrive,
  MessageSquare,
  Plug,
  X
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { IntegrationIcon, getIntegrationColor } from '../integrations/integration-icons'

interface Tool {
  id: string
  name: string
  icon?: string
  connected: boolean
  enabled: boolean
}

interface ToolSelectorProps {
  tools: Tool[]
  onToggle: (toolId: string, enabled: boolean) => void
  onManageIntegrations: () => void
}

export function ToolSelector({ tools, onToggle, onManageIntegrations }: ToolSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen])

  // Get connected and enabled tools
  const connectedTools = tools.filter(t => t.connected)
  const enabledTools = connectedTools.filter(t => t.enabled)
  const enabledCount = enabledTools.length

  // Show first 3 enabled tool icons, then +N
  const visibleTools = enabledTools.slice(0, 3)
  const extraCount = enabledCount - 3

  return (
    <div className="relative" ref={dropdownRef}>
      {/* Compact pill showing enabled tools */}
      <div className="flex items-center gap-2">
        {/* Add tool button */}
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setIsOpen(!isOpen)}
          className={cn(
            "h-8 w-8 p-0 rounded-full cursor-pointer",
            "bg-muted/50 hover:bg-muted",
            "transition-colors"
          )}
        >
          <Plus className="h-4 w-4" />
        </Button>

        {/* Enabled tools pill */}
        {enabledCount > 0 && (
          <button
            onClick={() => setIsOpen(!isOpen)}
            className={cn(
              "flex items-center gap-1 px-2.5 py-1.5 rounded-full cursor-pointer",
              "bg-muted/50 hover:bg-muted",
              "transition-colors"
            )}
          >
            {visibleTools.map(tool => (
              <div 
                key={tool.id} 
                className="h-5 w-5 flex items-center justify-center"
                title={tool.name}
              >
                <IntegrationIcon 
                  id={tool.id} 
                  className="h-4 w-4" 
                  style={{ color: getIntegrationColor(tool.id) }}
                />
              </div>
            ))}
            {extraCount > 0 && (
              <span className="text-xs text-muted-foreground ml-0.5">
                +{extraCount}
              </span>
            )}
          </button>
        )}

        {/* Settings button */}
        <Button
          variant="ghost"
          size="sm"
          onClick={onManageIntegrations}
          className={cn(
            "h-8 w-8 p-0 rounded-full cursor-pointer",
            "bg-muted/50 hover:bg-muted",
            "transition-colors"
          )}
          title="Manage integrations"
        >
          <Settings2 className="h-4 w-4" />
        </Button>
      </div>

      {/* Dropdown */}
      {isOpen && (
        <div 
          className={cn(
            "absolute bottom-full left-0 mb-2 z-50",
            "w-72 rounded-xl border bg-popover shadow-xl"
          )}
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-center justify-between px-3 py-2.5 border-b">
            <span className="text-sm font-medium">Tools & Integrations</span>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 cursor-pointer"
              onClick={() => setIsOpen(false)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>

          {/* Tool list */}
          <div className="max-h-80 overflow-y-auto py-1">
            {connectedTools.length === 0 ? (
              <div className="px-3 py-6 text-center">
                <Plug className="h-8 w-8 mx-auto text-muted-foreground/40 mb-2" />
                <p className="text-sm text-muted-foreground">No tools connected</p>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => {
                    setIsOpen(false)
                    onManageIntegrations()
                  }}
                  className="mt-1 cursor-pointer"
                >
                  Add your first integration
                </Button>
              </div>
            ) : (
              connectedTools.map(tool => (
                <ToolItem
                  key={tool.id}
                  tool={tool}
                  onToggle={(enabled) => onToggle(tool.id, enabled)}
                />
              ))
            )}
          </div>

          {/* Footer */}
          <div className="border-t px-1 py-1">
            <button
              onClick={() => {
                setIsOpen(false)
                onManageIntegrations()
              }}
              className={cn(
                "flex items-center gap-3 w-full px-3 py-2 rounded-lg",
                "text-sm text-muted-foreground",
                "hover:bg-accent hover:text-foreground",
                "transition-colors cursor-pointer"
              )}
            >
              <Plus className="h-4 w-4" />
              <span>Add connectors</span>
              <div className="flex items-center gap-1 ml-auto">
                <div className="h-5 w-5 rounded bg-muted flex items-center justify-center">
                  <Database className="h-3 w-3" />
                </div>
                <div className="h-5 w-5 rounded bg-muted flex items-center justify-center">
                  <Cloud className="h-3 w-3" />
                </div>
                <span className="text-xs text-muted-foreground">+40</span>
              </div>
            </button>
            <button
              onClick={() => {
                setIsOpen(false)
                onManageIntegrations()
              }}
              className={cn(
                "flex items-center gap-3 w-full px-3 py-2 rounded-lg",
                "text-sm text-muted-foreground",
                "hover:bg-accent hover:text-foreground",
                "transition-colors cursor-pointer"
              )}
            >
              <Settings2 className="h-4 w-4" />
              <span>Manage connectors</span>
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// Individual tool item with toggle
function ToolItem({ tool, onToggle }: { tool: Tool; onToggle: (enabled: boolean) => void }) {
  const color = getIntegrationColor(tool.id)
  
  return (
    <button
      onClick={(e) => {
        e.stopPropagation()
        onToggle(!tool.enabled)
      }}
      className={cn(
        "flex items-center gap-3 w-full px-3 py-2.5",
        "hover:bg-accent transition-colors cursor-pointer"
      )}
    >
      {/* Icon */}
      <div 
        className="h-8 w-8 rounded-lg flex items-center justify-center"
        style={{ backgroundColor: `${color}15` }}
      >
        <IntegrationIcon id={tool.id} className="h-4 w-4" style={{ color }} />
      </div>

      {/* Name */}
      <span className="flex-1 text-sm text-left font-medium">{tool.name}</span>

      {/* Toggle */}
      <div className={cn(
        "relative w-10 h-6 rounded-full transition-colors",
        tool.enabled ? "bg-primary" : "bg-muted"
      )}>
        <div className={cn(
          "absolute top-1 w-4 h-4 rounded-full bg-white shadow transition-transform",
          tool.enabled ? "translate-x-5" : "translate-x-1"
        )} />
      </div>
    </button>
  )
}

// Compact version for showing in empty state
export function ToolSelectorCompact({ 
  onOpenSettings 
}: { 
  onOpenSettings: () => void 
}) {
  return (
    <div className="flex items-center gap-2">
      <Button
        variant="ghost"
        size="sm"
        onClick={onOpenSettings}
        className={cn(
          "h-8 px-3 rounded-full cursor-pointer",
          "bg-muted/50 hover:bg-muted",
          "transition-colors text-muted-foreground"
        )}
      >
        <Plus className="h-4 w-4 mr-1.5" />
        <span className="text-xs">Add tools</span>
      </Button>
    </div>
  )
}
