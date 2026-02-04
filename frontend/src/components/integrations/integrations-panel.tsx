/**
 * Integrations Panel Component
 * 
 * Displays all available integrations grouped by category.
 * Handles connection/disconnection of integrations.
 */

import { useState, useEffect, useCallback } from 'react'
import { X, Check, Loader2, Search, ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { api, type Integration } from '@/lib/api'
import { IntegrationIcon, getIntegrationColor } from './integration-icons'
import { ConnectDialog } from './connect-dialog'

interface IntegrationsPanelProps {
  onClose?: () => void
}

// Category display names and order
const CATEGORY_INFO: Record<string, { name: string; order: number }> = {
  cloud: { name: 'Cloud Providers', order: 1 },
  developer_tools: { name: 'Developer Tools', order: 2 },
  data: { name: 'Data & Analytics', order: 3 },
  productivity: { name: 'Productivity', order: 4 },
  communication: { name: 'Communication', order: 5 },
  monitoring: { name: 'Monitoring & Observability', order: 6 },
}

export function IntegrationsPanel({ onClose }: IntegrationsPanelProps) {
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(['cloud', 'developer_tools']))
  const [connectingId, setConnectingId] = useState<string | null>(null)
  const [dialogIntegration, setDialogIntegration] = useState<Integration | null>(null)

  const loadIntegrations = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.listIntegrations()
      setIntegrations(data.integrations || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load integrations')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadIntegrations()
  }, [loadIntegrations])

  // Filter integrations by search
  const filteredIntegrations = integrations.filter((int) => {
    if (!searchQuery) return true
    const query = searchQuery.toLowerCase()
    return (
      int.name.toLowerCase().includes(query) ||
      int.description.toLowerCase().includes(query) ||
      int.category.toLowerCase().includes(query)
    )
  })

  // Group by category
  const groupedIntegrations = filteredIntegrations.reduce((acc, int) => {
    const category = int.category || 'other'
    if (!acc[category]) acc[category] = []
    acc[category].push(int)
    return acc
  }, {} as Record<string, Integration[]>)

  // Sort categories
  const sortedCategories = Object.keys(groupedIntegrations).sort((a, b) => {
    const orderA = CATEGORY_INFO[a]?.order ?? 99
    const orderB = CATEGORY_INFO[b]?.order ?? 99
    return orderA - orderB
  })

  const toggleCategory = (category: string) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev)
      if (next.has(category)) {
        next.delete(category)
      } else {
        next.add(category)
      }
      return next
    })
  }

  const handleConnect = (integration: Integration) => {
    // For OAuth-based or complex integrations, show dialog
    if (integration.authType === 'oauth2' || integration.authType === 'service_account') {
      setDialogIntegration(integration)
    } else if (integration.authType === 'api_key' || integration.authType === 'token') {
      setDialogIntegration(integration)
    } else {
      // For IAM role based, just mark as needing configuration
      setDialogIntegration(integration)
    }
  }

  const handleDisconnect = async (integrationId: string) => {
    try {
      setConnectingId(integrationId)
      await api.disconnectIntegration(integrationId)
      await loadIntegrations()
    } catch (err) {
      console.error('Failed to disconnect:', err)
    } finally {
      setConnectingId(null)
    }
  }

  const handleConnectionComplete = async () => {
    setDialogIntegration(null)
    await loadIntegrations()
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-4">
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
          <Button variant="link" size="sm" onClick={loadIntegrations} className="ml-2">
            Retry
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header - only show if standalone */}
      {onClose && (
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div>
            <h3 className="font-semibold">Integrations</h3>
            <p className="text-xs text-muted-foreground">
              {integrations.length} available
            </p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="cursor-pointer">
            <X className="h-4 w-4" />
          </Button>
        </div>
      )}

      {/* Search */}
      <div className="border-b px-4 py-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search integrations..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      {/* Integrations List */}
      <ScrollArea className="flex-1">
        <div className="p-2">
          {sortedCategories.map((category) => {
            const items = groupedIntegrations[category]
            const isExpanded = expandedCategories.has(category)
            const categoryName = CATEGORY_INFO[category]?.name || category.replace('_', ' ')

            return (
              <div key={category} className="mb-2">
                {/* Category Header */}
                <button
                  onClick={() => toggleCategory(category)}
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm font-medium hover:bg-accent cursor-pointer"
                >
                  {isExpanded ? (
                    <ChevronDown className="h-4 w-4" />
                  ) : (
                    <ChevronRight className="h-4 w-4" />
                  )}
                  <span className="capitalize">{categoryName}</span>
                  <span className="ml-auto text-xs text-muted-foreground">
                    {items.length}
                  </span>
                </button>

                {/* Category Items - Grid Layout */}
                {isExpanded && (
                  <div className="mt-2 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2 pl-2">
                    {items.map((integration) => (
                      <IntegrationItem
                        key={integration.id}
                        integration={integration}
                        isConnecting={connectingId === integration.id}
                        onConnect={() => handleConnect(integration)}
                        onDisconnect={() => handleDisconnect(integration.id)}
                      />
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </ScrollArea>

      {/* Connect Dialog */}
      {dialogIntegration && (
        <ConnectDialog
          integration={dialogIntegration}
          onClose={() => setDialogIntegration(null)}
          onSuccess={handleConnectionComplete}
        />
      )}
    </div>
  )
}

// Individual integration item component
interface IntegrationItemProps {
  integration: Integration
  isConnecting: boolean
  onConnect: () => void
  onDisconnect: () => void
}

function IntegrationItem({ integration, isConnecting, onConnect, onDisconnect }: IntegrationItemProps) {
  const color = getIntegrationColor(integration.id)
  const isConnected = integration.connected

  return (
    <div className="flex items-center gap-2 rounded-lg border p-2 transition-colors hover:bg-accent/50">
      {/* Icon */}
      <div
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md"
        style={{ backgroundColor: `${color}15` }}
      >
        <IntegrationIcon id={integration.id} className="h-4 w-4" style={{ color }} />
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0 overflow-hidden">
        <div className="flex items-center gap-1">
          <span className="font-medium text-sm truncate">{integration.name}</span>
          {integration.beta && (
            <span className="shrink-0 rounded bg-yellow-100 px-1 py-0.5 text-[10px] font-medium text-yellow-800">
              Beta
            </span>
          )}
        </div>
        <p className="text-xs text-muted-foreground truncate">
          {integration.description}
        </p>
      </div>

      {/* Action - always visible */}
      <div className="shrink-0">
        {isConnected ? (
          <div className="flex items-center gap-1">
            <Check className="h-3 w-3 text-green-600" />
            <Button
              variant="ghost"
              size="sm"
              onClick={onDisconnect}
              disabled={isConnecting}
              className="h-7 px-2 text-xs cursor-pointer"
            >
              Disconnect
            </Button>
          </div>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={onConnect}
            disabled={isConnecting}
            className="h-7 px-3 text-xs cursor-pointer"
          >
            {isConnecting ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              'Connect'
            )}
          </Button>
        )}
      </div>
    </div>
  )
}
