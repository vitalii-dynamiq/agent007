/**
 * API Client for Dynamiq Backend
 * 
 * Endpoints:
 * - /api/conversations - Conversation management
 * - /api/integrations - Integration catalog and connections
 * - /api/mcp - MCP provider operations
 */

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

// Static user ID for demo - ensures consistency across sessions and restarts
const DEMO_USER_ID = 'demo_user_dynamiq'

// Get the external user ID - static for demo purposes
// Exported so connect-dialog can use the same userID
export function getExternalUserId(): string {
  // Log user ID to console for debugging
  console.log('[Dynamiq] User ID:', DEMO_USER_ID)
  return DEMO_USER_ID
}

// Common headers for all API requests (includes ngrok header and user ID)
const getHeaders = (contentType = true): HeadersInit => {
  const headers: HeadersInit = {
    'ngrok-skip-browser-warning': 'true', // Skip ngrok interstitial page
    'X-User-ID': getExternalUserId(), // Include user ID for MCP account matching
  }
  if (contentType) {
    headers['Content-Type'] = 'application/json'
  }
  return headers
}

// =============================================================================
// Types
// =============================================================================

export interface Conversation {
  id: string
  title: string
  userId: string
  sandboxId?: string
  enabledTools?: string[] // Integration IDs enabled for this chat
  messages: Message[]
  createdAt: string
  updatedAt: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  toolCalls?: ToolCall[]
  toolCallId?: string
  createdAt: string
}

export interface ToolCall {
  id: string
  name: string
  arguments: string
  result?: string
}

export interface Integration {
  id: string
  name: string
  description: string
  category: string
  icon: string
  providerType: 'cli' | 'cloud_cli' | 'mcp' | 'direct_mcp' | 'api'
  authType: 'oauth2' | 'api_key' | 'token' | 'iam_role' | 'service_account' | 'aws_access_key' | 'github_app' | 'database'
  mcpProvider?: string
  mcpAppSlug?: string
  capabilities: string[]
  enabled: boolean
  connected?: boolean
  beta?: boolean
}

export interface ConnectedApp {
  app: string
  accountId: string
  name: string
  provider?: string
}

export interface SSEEvent {
  type: string
  data: Record<string, unknown>
}

// =============================================================================
// API Client
// =============================================================================

export const api = {
  // -------------------------------------------------------------------------
  // Conversations
  // -------------------------------------------------------------------------
  
  async listConversations(): Promise<Conversation[]> {
    const res = await fetch(`${API_URL}/api/conversations`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to list conversations')
    return res.json()
  },

  async createConversation(title?: string): Promise<Conversation> {
    const res = await fetch(`${API_URL}/api/conversations`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({ title }),
    })
    if (!res.ok) throw new Error('Failed to create conversation')
    return res.json()
  },

  async getConversation(id: string): Promise<Conversation> {
    const res = await fetch(`${API_URL}/api/conversations/${id}`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to get conversation')
    return res.json()
  },

  async deleteConversation(id: string): Promise<void> {
    const res = await fetch(`${API_URL}/api/conversations/${id}`, {
      method: 'DELETE',
      headers: getHeaders(false),
    })
    if (!res.ok) throw new Error('Failed to delete conversation')
  },

  async updateConversation(id: string, data: { title?: string; enabledTools?: string[] }): Promise<Conversation> {
    const res = await fetch(`${API_URL}/api/conversations/${id}`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify(data),
    })
    if (!res.ok) throw new Error('Failed to update conversation')
    return res.json()
  },

  async getConversationTools(id: string): Promise<string[]> {
    const res = await fetch(`${API_URL}/api/conversations/${id}/tools`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to get conversation tools')
    const data = await res.json()
    return data.enabledTools || []
  },

  async setConversationTools(id: string, tools: string[]): Promise<void> {
    const res = await fetch(`${API_URL}/api/conversations/${id}/tools`, {
      method: 'PUT',
      headers: getHeaders(),
      body: JSON.stringify({ enabledTools: tools }),
    })
    if (!res.ok) throw new Error('Failed to set conversation tools')
  },

  // -------------------------------------------------------------------------
  // Messages (SSE Streaming)
  // -------------------------------------------------------------------------
  
  sendMessage(
    conversationId: string, 
    content: string, 
    onEvent: (event: SSEEvent) => void,
    files?: Array<{ name: string; size: number; type: string; data: string }>
  ): () => void {
    const controller = new AbortController()
    
    // Build request body with optional files
    const body: Record<string, unknown> = { content }
    if (files && files.length > 0) {
      body.files = files
    }
    
    fetch(`${API_URL}/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify(body),
      signal: controller.signal,
    }).then(async (res) => {
      if (!res.ok) {
        onEvent({ type: 'error', data: { message: 'Failed to send message' } })
        return
      }

      const reader = res.body?.getReader()
      if (!reader) return

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        
        // Parse SSE events from buffer
        // SSE format: "event: <type>\ndata: <json>\n\n"
        let eventEnd = buffer.indexOf('\n\n')
        while (eventEnd !== -1) {
          const eventBlock = buffer.slice(0, eventEnd)
          buffer = buffer.slice(eventEnd + 2)
          
          // Parse the event block
          let eventType = ''
          let eventData = ''
          
          for (const line of eventBlock.split('\n')) {
            if (line.startsWith('event: ')) {
              eventType = line.slice(7)
            } else if (line.startsWith('data: ')) {
              eventData = line.slice(6)
            }
          }
          
          if (eventType && eventData) {
            try {
              const data = JSON.parse(eventData)
              console.log(`[SSE] ${eventType}:`, data)
              onEvent({ type: eventType, data })
            } catch (e) {
              console.warn('[SSE] Failed to parse data:', eventData, e)
            }
          }
          
          eventEnd = buffer.indexOf('\n\n')
        }
      }
      
      // Send done event if not already sent
      onEvent({ type: 'done', data: {} })
    }).catch((err) => {
      if (err.name !== 'AbortError') {
        onEvent({ type: 'error', data: { message: err.message } })
      }
    })

    return () => controller.abort()
  },

  // -------------------------------------------------------------------------
  // Integrations
  // -------------------------------------------------------------------------
  
  async listIntegrations(): Promise<{ integrations: Integration[] }> {
    const res = await fetch(`${API_URL}/api/integrations`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to list integrations')
    return res.json()
  },

  async getIntegration(id: string): Promise<Integration> {
    const res = await fetch(`${API_URL}/api/integrations/${id}`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to get integration')
    return res.json()
  },

  async connectIntegration(id: string, data: Record<string, string>): Promise<void> {
    const res = await fetch(`${API_URL}/api/integrations/${id}/connect`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify(data),
    })
    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: 'Connection failed' }))
      throw new Error(error.error || 'Failed to connect integration')
    }
  },

  async getOAuth2Url(id: string): Promise<{ authUrl: string; state?: string }> {
    const res = await fetch(`${API_URL}/api/integrations/${id}/connect`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify({}),
    })
    if (!res.ok) throw new Error('Failed to start OAuth flow')
    return res.json()
  },

  async disconnectIntegration(id: string): Promise<void> {
    const res = await fetch(`${API_URL}/api/integrations/${id}/disconnect`, {
      method: 'DELETE',
      headers: getHeaders(false),
    })
    if (!res.ok) throw new Error('Failed to disconnect integration')
  },

  // -------------------------------------------------------------------------
  // Cloud Credentials
  // -------------------------------------------------------------------------

  async storeAwsCredentials(data: {
    accountId?: string
    roleArn?: string
    externalId?: string
    region?: string
    accessKeyId?: string
    secretAccessKey?: string
    name?: string
  }): Promise<void> {
    const res = await fetch(`${API_URL}/api/cloud/credentials/aws`, {
      method: 'POST',
      headers: getHeaders(),
      body: JSON.stringify(data),
    })
    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: 'Failed to store AWS credentials' }))
      throw new Error(error.error || 'Failed to store AWS credentials')
    }
  },

  async getGitHubInstallUrl(): Promise<{ authUrl: string; state: string }> {
    const res = await fetch(`${API_URL}/api/github/install`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to get GitHub install URL')
    return res.json()
  },

  // -------------------------------------------------------------------------
  // MCP / Auth
  // -------------------------------------------------------------------------
  
  async getConnectToken(): Promise<string> {
    const res = await fetch(`${API_URL}/api/auth/connect-token`, {
      method: 'POST',
      headers: getHeaders(false),
    })
    if (!res.ok) {
      const errorText = await res.text().catch(() => '')
      throw new Error(errorText || 'Failed to get connect token')
    }
    const data = await res.json()
    return data.token
  },

  async getConnectTokenWithProvider(
    provider: string,
    app?: string,
    options?: { connectionData?: Record<string, string> },
  ): Promise<{ token?: string; connectLinkUrl?: string; expiresAt?: string; provider?: string }> {
    const url = new URL(`${API_URL}/api/auth/connect-token`)
    url.searchParams.set('provider', provider)
    if (app) url.searchParams.set('app', app)
    const body = options?.connectionData ? JSON.stringify({ connectionData: options.connectionData }) : undefined
    const headers = getHeaders(!!body)
    const res = await fetch(url.toString(), {
      method: 'POST',
      headers,
      body,
    })
    if (!res.ok) {
      const errorText = await res.text().catch(() => '')
      throw new Error(errorText || 'Failed to get connect token')
    }
    return res.json()
  },

  async listConnectedApps(): Promise<ConnectedApp[]> {
    const res = await fetch(`${API_URL}/api/apps`, { headers: getHeaders(false) })
    if (!res.ok) {
      console.warn('Connected apps endpoint not available')
      return []
    }
    return res.json()
  },

  async getMCPProviders(): Promise<{
    providers: string[]
    providerInfos?: Array<{ name: string; type?: string; description?: string }>
    default?: string
    defaultProvider?: string
  }> {
    const res = await fetch(`${API_URL}/api/mcp/providers`, { headers: getHeaders(false) })
    if (!res.ok) throw new Error('Failed to get MCP providers')
    return res.json()
  },

  /**
   * Transcribe audio using OpenAI's speech-to-text API
   * @param audioBlob - The audio blob to transcribe
   * @returns The transcription result
   */
  async transcribeAudio(audioBlob: Blob): Promise<{ text: string }> {
    const formData = new FormData()
    formData.append('file', audioBlob, 'recording.webm')
    
    const res = await fetch(`${API_URL}/api/transcribe`, {
      method: 'POST',
      headers: {
        'ngrok-skip-browser-warning': 'true',
        'X-User-ID': getExternalUserId(),
        // Note: Don't set Content-Type for FormData - browser sets it with boundary
      },
      body: formData,
    })
    
    if (!res.ok) {
      const errorText = await res.text().catch(() => '')
      throw new Error(errorText || 'Transcription failed')
    }
    
    return res.json()
  },

  /**
   * Pre-warm a sandbox for faster first message response
   */
  async warmSandbox(): Promise<{ status: string; sandbox_id?: string; ready: boolean; message?: string }> {
    console.log('[Dynamiq] Pre-warming sandbox...')
    try {
      const res = await fetch(`${API_URL}/api/sandbox/warm`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'ngrok-skip-browser-warning': 'true',
          'X-User-ID': getExternalUserId(),
        },
      })
      
      if (!res.ok) {
        console.warn('[Dynamiq] Warm sandbox request failed:', res.status)
        return { status: 'error', ready: false }
      }
      
      const result = await res.json()
      console.log('[Dynamiq] Warm sandbox response:', result)
      return result
    } catch (err) {
      console.warn('[Dynamiq] Warm sandbox error:', err)
      return { status: 'error', ready: false }
    }
  },

  /**
   * Check status of warm sandbox
   */
  async warmSandboxStatus(): Promise<{ status: string; sandbox_id?: string; ready: boolean; message?: string }> {
    try {
      const res = await fetch(`${API_URL}/api/sandbox/warm/status`, {
        method: 'GET',
        headers: {
          'ngrok-skip-browser-warning': 'true',
          'X-User-ID': getExternalUserId(),
        },
      })
      
      if (!res.ok) {
        return { status: 'error', ready: false }
      }
      
      return res.json()
    } catch (err) {
      return { status: 'error', ready: false }
    }
  },
}
