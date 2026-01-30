import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { api, type Integration } from '@/lib/api'
import { IntegrationIcon, getIntegrationColor } from '@/components/integrations/integration-icons'
import { Check, Settings2 } from 'lucide-react'

interface ToolSelectorProps {
  conversationId: string
  enabledTools: string[]
  onToolsChange: (tools: string[]) => void
}

export function ToolSelector({ conversationId, enabledTools, onToolsChange }: ToolSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [selectedTools, setSelectedTools] = useState<Set<string>>(new Set(enabledTools))
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.listIntegrations().then((data) => {
      // Only show connected integrations
      setIntegrations(data.integrations.filter((i) => i.connected))
    })
  }, [])

  useEffect(() => {
    setSelectedTools(new Set(enabledTools))
  }, [enabledTools])

  const toggleTool = (id: string) => {
    const newSelected = new Set(selectedTools)
    if (newSelected.has(id)) {
      newSelected.delete(id)
    } else {
      newSelected.add(id)
    }
    setSelectedTools(newSelected)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const tools = Array.from(selectedTools)
      await api.setConversationTools(conversationId, tools)
      onToolsChange(tools)
      setIsOpen(false)
    } catch (err) {
      console.error('Failed to save tools:', err)
    } finally {
      setSaving(false)
    }
  }

  const handleSelectAll = () => {
    setSelectedTools(new Set(integrations.map((i) => i.id)))
  }

  const handleSelectNone = () => {
    setSelectedTools(new Set())
  }

  if (!isOpen) {
    return (
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setIsOpen(true)}
        className="gap-2 text-muted-foreground hover:text-foreground cursor-pointer"
      >
        <Settings2 className="h-4 w-4" />
        {enabledTools.length > 0 ? `${enabledTools.length} tools` : 'All tools'}
      </Button>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-lg w-full max-w-md max-h-[80vh] overflow-hidden">
        <div className="p-4 border-b">
          <h3 className="font-semibold">Select Tools for this Chat</h3>
          <p className="text-sm text-muted-foreground">
            Choose which integrations the AI can use in this conversation
          </p>
        </div>

        <div className="p-4 flex gap-2">
          <Button variant="outline" size="sm" onClick={handleSelectAll} className="cursor-pointer">
            Select All
          </Button>
          <Button variant="outline" size="sm" onClick={handleSelectNone} className="cursor-pointer">
            Select None
          </Button>
        </div>

        <div className="overflow-y-auto max-h-[50vh] p-4 pt-0">
          {integrations.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              No connected integrations. Connect integrations in the Integrations panel first.
            </p>
          ) : (
            <div className="space-y-2">
              {integrations.map((integration) => {
                const color = getIntegrationColor(integration.id)
                const isSelected = selectedTools.has(integration.id)

                return (
                  <button
                    key={integration.id}
                    onClick={() => toggleTool(integration.id)}
                    className={`w-full flex items-center gap-3 p-3 rounded-lg border transition-colors cursor-pointer ${
                      isSelected
                        ? 'border-primary bg-primary/5'
                        : 'border-border hover:bg-muted/50'
                    }`}
                  >
                    <div
                      className="w-8 h-8 rounded-md flex items-center justify-center"
                      style={{ backgroundColor: `${color}15` }}
                    >
                      <IntegrationIcon id={integration.id} style={{ color }} className="h-4 w-4" />
                    </div>
                    <div className="flex-1 text-left">
                      <div className="font-medium text-sm">{integration.name}</div>
                      <div className="text-xs text-muted-foreground truncate">
                        {integration.description}
                      </div>
                    </div>
                    {isSelected && (
                      <Check className="h-5 w-5 text-primary" />
                    )}
                  </button>
                )
              })}
            </div>
          )}
        </div>

        <div className="p-4 border-t flex justify-end gap-2">
          <Button variant="outline" onClick={() => setIsOpen(false)} className="cursor-pointer">
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving} className="cursor-pointer">
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </div>
    </div>
  )
}
