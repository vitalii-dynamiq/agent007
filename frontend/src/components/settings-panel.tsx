/**
 * Settings Panel Component
 * 
 * Tabbed panel containing Integrations and Skills management.
 */

import { useState } from 'react'
import { X, Plug, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { IntegrationsPanel } from './integrations/integrations-panel'
import { SkillsPanel } from './skills/skills-panel'
import { cn } from '@/lib/utils'

interface SettingsPanelProps {
  onClose: () => void
  initialTab?: 'integrations' | 'skills'
}

export function SettingsPanel({ onClose, initialTab = 'integrations' }: SettingsPanelProps) {
  const [activeTab, setActiveTab] = useState<'integrations' | 'skills'>(initialTab)

  return (
    <div className="flex h-full flex-col">
      {/* Header with Tabs */}
      <div className="border-b">
        <div className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-4">
            {/* Tab buttons */}
            <div className="flex">
              <button
                onClick={() => setActiveTab('integrations')}
                className={cn(
                  "flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-[13px] transition-colors cursor-pointer",
                  activeTab === 'integrations'
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
              >
                <Plug className="h-4 w-4" />
                Integrations
              </button>
              <button
                onClick={() => setActiveTab('skills')}
                className={cn(
                  "flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 -mb-[13px] transition-colors cursor-pointer",
                  activeTab === 'skills'
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
              >
                <Sparkles className="h-4 w-4" />
                Skills
              </button>
            </div>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="cursor-pointer">
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-hidden relative">
        {activeTab === 'integrations' ? (
          <IntegrationsPanel />
        ) : (
          <SkillsPanel />
        )}
      </div>
    </div>
  )
}
