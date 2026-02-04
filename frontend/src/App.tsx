/**
 * Dynamiq AI Agent Application
 * 
 * Main application component with responsive layout:
 * - Desktop: Sidebar + Chat + Optional Integrations Panel
 * - Mobile: Collapsible sidebar with hamburger menu
 */

import { useState, useEffect, useCallback } from 'react'
import { Menu, X, Settings } from 'lucide-react'
import { ConversationList } from '@/components/sidebar/conversation-list'
import { ChatView } from '@/components/chat/chat-view'
import { type UploadedFile } from '@/components/chat/message-input'
import { SettingsPanel } from '@/components/settings-panel'
import { Button } from '@/components/ui/button'
import { DynamiqLogo } from '@/components/ui/logo'
import { api, type Conversation } from '@/lib/api'

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [selectedId, setSelectedId] = useState<string | undefined>()
  const [showIntegrations, setShowIntegrations] = useState(false)
  const [showMobileSidebar, setShowMobileSidebar] = useState(false)

  const loadConversations = useCallback(async () => {
    try {
      const convs = await api.listConversations()
      // Handle null/undefined response
      if (convs && Array.isArray(convs)) {
        setConversations(convs.sort((a, b) => 
          new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
        ))
      } else {
        setConversations([])
      }
    } catch (err) {
      console.error('Failed to load conversations:', err)
    }
  }, [])

  useEffect(() => {
    // Load conversations
    loadConversations()
    
    // Pre-warm sandbox for faster first message
    // This runs in the background and is best-effort
    api.warmSandbox().then((result) => {
      if (result.ready) {
        console.log('[App] Sandbox already warm:', result.sandbox_id)
      } else {
        console.log('[App] Sandbox warming started')
      }
    }).catch((err) => {
      console.warn('[App] Failed to warm sandbox:', err)
    })
  }, [loadConversations])

  // Close mobile sidebar when selecting a conversation
  const handleSelectConversation = (id: string) => {
    setSelectedId(id)
    setShowMobileSidebar(false)
  }

  const handleCreate = async () => {
    try {
      const conv = await api.createConversation()
      setConversations((prev) => [conv, ...prev])
      setSelectedId(conv.id)
      setShowMobileSidebar(false)
    } catch (err) {
      console.error('Failed to create conversation:', err)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteConversation(id)
      setConversations((prev) => prev.filter((c) => c.id !== id))
      if (selectedId === id) {
        setSelectedId(undefined)
      }
    } catch (err) {
      console.error('Failed to delete conversation:', err)
    }
  }

  const handleConversationUpdate = useCallback(async () => {
    if (selectedId) {
      try {
        const conv = await api.getConversation(selectedId)
        setConversations((prev) =>
          prev.map((c) => (c.id === selectedId ? conv : c))
        )
      } catch (err) {
        console.error('Failed to refresh conversation:', err)
      }
    }
  }, [selectedId])

  // Create conversation and return it - ChatView will handle sending the message
  const handleCreateAndSend = useCallback(async (_content: string, _files?: UploadedFile[]): Promise<Conversation> => {
    const conv = await api.createConversation()
    setConversations((prev) => [conv, ...prev])
    setSelectedId(conv.id)
    setShowMobileSidebar(false)
    return conv
  }, [])

  // Reset to new chat state
  const handleNewChat = useCallback(() => {
    setSelectedId(undefined)
    setShowMobileSidebar(false)
  }, [])

  const selectedConversation = conversations.find((c) => c.id === selectedId)

  return (
    <div className="flex h-screen bg-background">
      {/* Mobile Header */}
      <div className="fixed top-0 left-0 right-0 z-40 flex h-14 items-center justify-between border-b bg-background px-4 md:hidden">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setShowMobileSidebar(!showMobileSidebar)}
          className="cursor-pointer"
        >
          {showMobileSidebar ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </Button>
        <button 
          onClick={handleNewChat}
          className="flex items-center gap-2 cursor-pointer hover:opacity-80 transition-opacity"
        >
          <DynamiqLogo size="sm" />
          <span className="font-bold">Dynamiq</span>
        </button>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setShowIntegrations(!showIntegrations)}
          className="cursor-pointer"
        >
          <Settings className="h-5 w-5" />
        </Button>
      </div>

      {/* Mobile Sidebar Overlay */}
      {showMobileSidebar && (
        <div
          className="fixed inset-0 z-30 bg-black/50 md:hidden"
          onClick={() => setShowMobileSidebar(false)}
        />
      )}

      {/* Sidebar */}
      <div
        className={`
          fixed inset-y-0 left-0 z-30 w-64 transform border-r bg-background transition-transform duration-200 ease-in-out
          md:relative md:translate-x-0
          ${showMobileSidebar ? 'translate-x-0' : '-translate-x-full'}
          pt-14 md:pt-0
        `}
      >
        <div className="flex h-full flex-col">
          {/* Desktop Header */}
          <div className="hidden border-b p-4 md:block">
            <button 
              onClick={handleNewChat}
              className="flex items-center gap-2.5 cursor-pointer hover:opacity-80 transition-opacity"
            >
              <DynamiqLogo size="md" />
              <h1 className="font-bold text-lg">Dynamiq</h1>
            </button>
            <p className="text-xs text-muted-foreground mt-1">Your AI-Powered Data Analyst</p>
          </div>
          
          {/* Conversation List */}
          <div className="flex-1 overflow-hidden">
            <ConversationList
              conversations={conversations}
              selectedId={selectedId}
              onSelect={handleSelectConversation}
              onCreate={handleCreate}
              onDelete={handleDelete}
            />
          </div>

          {/* Sidebar Footer */}
          <div className="border-t p-2">
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start cursor-pointer"
              onClick={() => {
                setShowIntegrations(!showIntegrations)
                setShowMobileSidebar(false)
              }}
            >
              <Settings className="mr-2 h-4 w-4" />
              Settings
              <span className="ml-auto text-xs text-muted-foreground/50">
                Integrations & Skills
              </span>
            </Button>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex flex-1 flex-col pt-14 md:pt-0 min-w-0">
        <div className="flex flex-1 overflow-hidden min-w-0">
          {/* Chat View */}
          <div className="flex-1 min-w-0 overflow-hidden">
            <ChatView
              conversation={selectedConversation || null}
              onConversationUpdate={handleConversationUpdate}
              onManageIntegrations={() => setShowIntegrations(true)}
              onCreateAndSend={handleCreateAndSend}
            />
          </div>

        </div>
      </div>

      {/* Settings Panel - Full Screen Modal for all devices */}
      {showIntegrations && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-4xl h-[90vh] bg-background rounded-lg shadow-xl overflow-hidden m-4">
            <SettingsPanel onClose={() => setShowIntegrations(false)} />
          </div>
        </div>
      )}
    </div>
  )
}

export default App
