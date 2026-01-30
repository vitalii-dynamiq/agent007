/**
 * Connect Dialog Component
 * 
 * Handles different authentication types:
 * - OAuth2 via MCP (Pipedream/Composio) - Uses Pipedream SDK with iFrame
 * - API Key: Shows input field
 * - Token: Shows input field
 * - IAM Role: Shows AWS role configuration
 * - Service Account: Shows file upload for GCP
 */

import { useState, useEffect, useRef } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { X, Loader2, ExternalLink, AlertCircle, CheckCircle2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api, type Integration, getExternalUserId } from '@/lib/api'
import { IntegrationIcon, getIntegrationColor } from './integration-icons'
import { createFrontendClient, type PipedreamClient, type ConnectResult, type ConnectError } from '@pipedream/sdk/browser'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

interface ConnectDialogProps {
  integration: Integration
  onClose: () => void
  onSuccess: () => void
}

export function ConnectDialog({ integration, onClose, onSuccess }: ConnectDialogProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [oauthStatus, setOauthStatus] = useState<'idle' | 'waiting' | 'success' | 'error'>('idle')
  
  // Form state for different auth types
  const [apiKey, setApiKey] = useState('')
  const [token, setToken] = useState('')
  const [roleArn, setRoleArn] = useState('')
  const [externalId, setExternalId] = useState('')
  const [serviceAccountJson, setServiceAccountJson] = useState('')
  const [awsAuthMode, setAwsAuthMode] = useState<'access_keys' | 'iam_role'>('access_keys')
  const [awsAccountId, setAwsAccountId] = useState('')
  const [awsAccessKeyId, setAwsAccessKeyId] = useState('')
  const [awsSecretAccessKey, setAwsSecretAccessKey] = useState('')
  const [awsRegion, setAwsRegion] = useState('')
  const [installPollActive, setInstallPollActive] = useState(false)
  const [atlassianSubdomain, setAtlassianSubdomain] = useState('')
  
  // Pipedream client ref
  const pdClientRef = useRef<PipedreamClient | null>(null)

  const color = getIntegrationColor(integration.id)
  const requiresAtlassianSubdomain = integration.id === 'confluence'

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      pdClientRef.current = null
    }
  }, [])

  useEffect(() => {
    setAtlassianSubdomain('')
  }, [integration.id])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    try {
      let data: Record<string, string> = {}

      if (integration.authType === 'github_app') {
        await handleGitHubAppConnect()
        return
      }

      if (integration.id === 'aws' || integration.authType === 'aws_access_key') {
        if (awsAuthMode === 'iam_role') {
          if (!roleArn.trim()) throw new Error('Role ARN is required')
          await api.storeAwsCredentials({
            accountId: awsAccountId.trim() || undefined,
            roleArn: roleArn.trim(),
            externalId: externalId.trim() || undefined,
            region: awsRegion.trim() || undefined,
          })
        } else {
          if (!awsAccountId.trim()) throw new Error('AWS account ID is required')
          if (!awsAccessKeyId.trim()) throw new Error('Access key ID is required')
          if (!awsSecretAccessKey.trim()) throw new Error('Secret access key is required')
          await api.storeAwsCredentials({
            accountId: awsAccountId.trim(),
            accessKeyId: awsAccessKeyId.trim(),
            secretAccessKey: awsSecretAccessKey.trim(),
            region: awsRegion.trim() || undefined,
          })
        }

        const connectData: Record<string, string> = {
          accountName: awsAccountId.trim() ? `AWS ${awsAccountId.trim()}` : 'AWS',
        }
        if (awsAccountId.trim()) {
          connectData.accountId = awsAccountId.trim()
        }
        if (awsAuthMode === 'iam_role' && roleArn.trim()) {
          connectData.roleArn = roleArn.trim()
        }
        await api.connectIntegration(integration.id, connectData)

        setSuccess(true)
        setTimeout(() => {
          onSuccess()
        }, 1000)
        return
      }

      switch (integration.authType) {
        case 'api_key':
          if (!apiKey.trim()) throw new Error('API key is required')
          data = { apiKey: apiKey.trim() }
          break
        case 'token':
          if (!token.trim()) throw new Error('Token is required')
          data = { token: token.trim() }
          break
        case 'iam_role':
          if (!roleArn.trim()) throw new Error('Role ARN is required')
          data = { roleArn: roleArn.trim() }
          if (externalId.trim()) data.externalId = externalId.trim()
          if (awsRegion.trim()) data.region = awsRegion.trim()
          break
        case 'service_account':
          if (!serviceAccountJson.trim()) throw new Error('Service account JSON is required')
          try {
            JSON.parse(serviceAccountJson.trim())
          } catch {
            throw new Error('Invalid JSON format')
          }
          data = { serviceAccountJson: serviceAccountJson.trim() }
          break
        case 'oauth2':
          if (integration.providerType === 'direct_mcp') {
            await handleDirectOAuthConnect()
          } else if (integration.mcpProvider === 'composio') {
            await handleComposioConnect()
          } else if (!integration.mcpProvider) {
            await handleDirectOAuthConnect()
          } else {
            // For OAuth, use Pipedream SDK
            await handleOAuthConnect()
          }
          return
        default:
          throw new Error(`Unsupported auth type: ${integration.authType}`)
      }

      await api.connectIntegration(integration.id, data)
      setSuccess(true)
      setTimeout(() => {
        onSuccess()
      }, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connection failed')
    } finally {
      setLoading(false)
    }
  }

  const handleOAuthConnect = async () => {
    setOauthStatus('waiting')
    setError(null)

    try {
      // Get the Pipedream app slug
      const appSlug = getAppSlug(integration.id)
      const externalUserId = getExternalUserId()
      
      console.log('Starting OAuth connect for app:', appSlug, 'userId:', externalUserId)

      // Create Pipedream frontend client with token callback
      const client = createFrontendClient({
        externalUserId,
        // tokenCallback fetches a short-lived token from our backend
        tokenCallback: async () => {
          console.log('Token callback called for user:', externalUserId)
          const response = await fetch(`${API_URL}/api/auth/connect-token`, {
            method: 'POST',
            headers: { 
              'Content-Type': 'application/json',
              'ngrok-skip-browser-warning': 'true',
              'X-User-ID': externalUserId, // IMPORTANT: Must match the user ID used for chat
            },
          })
          
          if (!response.ok) {
            const errorText = await response.text()
            throw new Error(`Failed to get connect token: ${errorText}`)
          }
          
          const data = await response.json()
          console.log('Got connect token response:', { 
            hasToken: !!data.token, 
            hasConnectLinkUrl: !!data.connectLinkUrl,
            expiresAt: data.expiresAt
          })
          
          // Return the full response expected by SDK
          return {
            token: data.token,
            connectLinkUrl: data.connectLinkUrl || '',
            expiresAt: new Date(data.expiresAt),
          }
        },
      })
      
      pdClientRef.current = client

      console.log('Calling connectAccount for app:', appSlug)
      
      // Start the OAuth flow using the SDK with callbacks
      await client.connectAccount({
        app: appSlug,
        oauthAppId: undefined, // Use Pipedream's managed OAuth app
        
        onSuccess: async (result: ConnectResult) => {
          console.log('OAuth connection successful:', result)
          // Save to our backend
          try {
            await api.connectIntegration(integration.id, { 
              oauthComplete: 'true',
              accountId: result.id
            })
            setOauthStatus('success')
            setSuccess(true)
            setTimeout(() => {
              onSuccess()
            }, 1500)
          } catch (err) {
            console.error('Failed to save connection to backend:', err)
            setOauthStatus('error')
            setError('Connected but failed to save. Please try again.')
          }
        },
        
        onError: (err: ConnectError) => {
          console.error('OAuth connection error:', err)
          setOauthStatus('error')
          setError(err.message || 'OAuth connection failed')
        },
        
        onClose: (status) => {
          console.log('OAuth dialog closed:', status)
          if (!status.successful && !status.completed) {
            // User closed without completing
            setOauthStatus('idle')
          }
        },
      })

    } catch (err) {
      console.error('OAuth connect error:', err)
      setOauthStatus('error')
      
      // Provide helpful error messages
      const errorMessage = err instanceof Error ? err.message : 'OAuth connection failed'
      
      if (errorMessage.includes('project_support_email')) {
        setError('Pipedream project configuration issue. Please check the project settings.')
      } else if (errorMessage.includes('popup') || errorMessage.includes('blocked')) {
        setError('Popup was blocked. Please allow popups for this site and try again.')
      } else {
        setError(errorMessage)
      }
    }
  }

  const handleComposioConnect = async () => {
    setOauthStatus('waiting')
    setError(null)

    let providersInfo: { providers: string[] } | null = null
    try {
      providersInfo = await api.getMCPProviders().catch(() => null)
      const providers = providersInfo?.providers || []
      if (providersInfo && !providers.includes('composio')) {
        throw new Error('Composio is not configured on the backend')
      }

      const appSlug = integration.mcpAppSlug || integration.id
      const connectionData = requiresAtlassianSubdomain
        ? { subdomain: atlassianSubdomain.trim() }
        : undefined
      if (requiresAtlassianSubdomain && !atlassianSubdomain.trim()) {
        throw new Error('Atlassian subdomain is required')
      }
      const tokenResp = await api.getConnectTokenWithProvider(
        'composio',
        appSlug,
        connectionData ? { connectionData } : undefined,
      )
      const connectLinkUrl = tokenResp.connectLinkUrl || tokenResp.token
      if (!connectLinkUrl) {
        throw new Error('Composio connect link not available')
      }

      window.open(connectLinkUrl, '_blank', 'noopener,noreferrer,width=900,height=800')
      setInstallPollActive(true)
    } catch (err) {
      setOauthStatus('error')
      setError(err instanceof Error ? err.message : 'Composio connection failed')
    } finally {
      setLoading(false)
    }
  }

  const handleDirectOAuthConnect = async () => {
    setOauthStatus('waiting')
    setError(null)

    try {
      const { authUrl } = await api.getOAuth2Url(integration.id)
      if (!authUrl) {
        throw new Error('OAuth URL not available')
      }
      window.open(authUrl, '_blank', 'noopener,noreferrer,width=900,height=800')
      setInstallPollActive(true)
    } catch (err) {
      setOauthStatus('error')
      setError(err instanceof Error ? err.message : 'OAuth connection failed')
    } finally {
      setLoading(false)
    }
  }

  const handleGitHubAppConnect = async () => {
    setOauthStatus('waiting')
    setError(null)

    try {
      const { authUrl } = await api.getGitHubInstallUrl()
      window.open(authUrl, '_blank', 'noopener,noreferrer,width=900,height=800')
      setInstallPollActive(true)
    } catch (err) {
      setOauthStatus('error')
      setError(err instanceof Error ? err.message : 'GitHub App connection failed')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!installPollActive) return

    const interval = window.setInterval(async () => {
      try {
        if (integration.authType === 'github_app') {
          const response = await api.getIntegration(integration.id)
          if ((response as { connected?: boolean }).connected) {
            setOauthStatus('success')
            setSuccess(true)
            setInstallPollActive(false)
            window.clearInterval(interval)
            setTimeout(() => {
              onSuccess()
            }, 1000)
          }
          return
        }

        if (integration.providerType === 'direct_mcp') {
          const response = await api.getIntegration(integration.id)
          if ((response as { connected?: boolean }).connected) {
            setOauthStatus('success')
            setSuccess(true)
            setInstallPollActive(false)
            window.clearInterval(interval)
            setTimeout(() => {
              onSuccess()
            }, 1000)
          }
          return
        }

        if (integration.mcpProvider === 'composio') {
          const apps = await api.listConnectedApps()
          const appSlug = integration.mcpAppSlug || integration.id
          const match = apps.find((app) => app.provider === 'composio' && app.app === appSlug)
          if (match) {
            await api.connectIntegration(integration.id, {
              oauthComplete: 'true',
              accountId: match.accountId || '',
              accountName: match.name || '',
            })
            setOauthStatus('success')
            setSuccess(true)
            setInstallPollActive(false)
            window.clearInterval(interval)
            setTimeout(() => {
              onSuccess()
            }, 1000)
          }
        }
      } catch (err) {
        console.error('Failed to poll integration status', err)
      }
    }, 2000)

    const timeout = window.setTimeout(() => {
      setInstallPollActive(false)
      window.clearInterval(interval)
    }, 120000)

    return () => {
      window.clearInterval(interval)
      window.clearTimeout(timeout)
    }
  }, [
    installPollActive,
    integration.id,
    integration.authType,
    integration.providerType,
    integration.mcpProvider,
    integration.mcpAppSlug,
    onSuccess,
  ])

  const renderForm = () => {
    switch (integration.authType) {
      case 'github_app':
        return (
          <div className="space-y-4">
            <div className="rounded-md border border-muted-foreground/20 bg-muted/30 p-3 text-xs text-muted-foreground">
              You’ll be redirected to GitHub to install our GitHub App. You can select a specific org and choose all
              or specific repositories during installation.
            </div>
          </div>
        )
      case 'aws_access_key':
        return (
          <div className="space-y-4">
            <div className="flex gap-2">
              <Button
                type="button"
                variant={awsAuthMode === 'access_keys' ? 'default' : 'outline'}
                size="sm"
                onClick={() => setAwsAuthMode('access_keys')}
              >
                Access Keys
              </Button>
              <Button
                type="button"
                variant={awsAuthMode === 'iam_role' ? 'default' : 'outline'}
                size="sm"
                onClick={() => setAwsAuthMode('iam_role')}
              >
                IAM Role (secondary)
              </Button>
            </div>

            <div>
              <label className="text-sm font-medium">AWS Account ID</label>
              <Input
                value={awsAccountId}
                onChange={(e) => setAwsAccountId(e.target.value)}
                placeholder="123456789012"
                className="mt-1"
              />
            </div>

            {awsAuthMode === 'access_keys' ? (
              <>
                <div>
                  <label className="text-sm font-medium">Access Key ID</label>
                  <Input
                    value={awsAccessKeyId}
                    onChange={(e) => setAwsAccessKeyId(e.target.value)}
                    placeholder="AKIA..."
                    className="mt-1"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Secret Access Key</label>
                  <Input
                    type="password"
                    value={awsSecretAccessKey}
                    onChange={(e) => setAwsSecretAccessKey(e.target.value)}
                    placeholder="********"
                    className="mt-1"
                  />
                </div>
              </>
            ) : (
              <div>
                <label className="text-sm font-medium">Role ARN</label>
                <Input
                  value={roleArn}
                  onChange={(e) => setRoleArn(e.target.value)}
                  placeholder="arn:aws:iam::123456789012:role/YourRole"
                  className="mt-1"
                />
              </div>
            )}

            <div>
              <label className="text-sm font-medium">Region (optional)</label>
              <Input
                value={awsRegion}
                onChange={(e) => setAwsRegion(e.target.value)}
                placeholder="us-east-1"
                className="mt-1"
              />
            </div>

            {awsAuthMode === 'iam_role' && (
              <div>
                <label className="text-sm font-medium">External ID (optional)</label>
                <Input
                  value={externalId}
                  onChange={(e) => setExternalId(e.target.value)}
                  placeholder="External ID"
                  className="mt-1"
                />
              </div>
            )}
          </div>
        )
      case 'api_key':
        return (
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">API Key</label>
              <Input
                type="password"
                placeholder="Enter your API key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="mt-1.5"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Find your API key in your {integration.name} dashboard
              </p>
            </div>
          </div>
        )

      case 'token':
        return (
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Access Token</label>
              <Input
                type="password"
                placeholder="Enter your access token"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="mt-1.5"
              />
            </div>
          </div>
        )

      case 'iam_role':
        return (
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">IAM Role ARN</label>
              <Input
                placeholder="arn:aws:iam::123456789012:role/MyRole"
                value={roleArn}
                onChange={(e) => setRoleArn(e.target.value)}
                className="mt-1.5"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                The ARN of the IAM role to assume
              </p>
            </div>
            <div>
              <label className="text-sm font-medium">External ID (optional)</label>
              <Input
                placeholder="External ID for cross-account access"
                value={externalId}
                onChange={(e) => setExternalId(e.target.value)}
                className="mt-1.5"
              />
            </div>
          </div>
        )

      case 'service_account':
        return (
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Service Account JSON</label>
              <textarea
                placeholder='{"type": "service_account", ...}'
                value={serviceAccountJson}
                onChange={(e) => setServiceAccountJson(e.target.value)}
                rows={6}
                className="mt-1.5 w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Paste your service account key JSON from Google Cloud Console
              </p>
            </div>
          </div>
        )

      case 'oauth2':
        return (
          <div className="space-y-4">
            {requiresAtlassianSubdomain && (
              <div>
                <label className="text-sm font-medium">Atlassian Subdomain</label>
                <Input
                  placeholder="your-subdomain"
                  value={atlassianSubdomain}
                  onChange={(e) => setAtlassianSubdomain(e.target.value)}
                  className="mt-1.5"
                />
                <p className="mt-1 text-xs text-muted-foreground">
                  Use the subdomain from your-site.atlassian.net
                </p>
              </div>
            )}
            {oauthStatus === 'idle' && (
              <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                <p className="text-sm text-blue-800">
                  Click the button below to connect your {integration.name} account.
                  You'll be prompted to sign in and authorize access.
                </p>
              </div>
            )}
            {oauthStatus === 'waiting' && (
              <div className="flex flex-col items-center gap-3 py-4">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                <p className="text-sm text-muted-foreground">
                  Waiting for authorization...
                </p>
                <p className="text-xs text-muted-foreground">
                  Complete the sign-in in the authorization window
                </p>
              </div>
            )}
            {oauthStatus === 'success' && (
              <div className="flex flex-col items-center gap-3 py-4">
                <CheckCircle2 className="h-8 w-8 text-green-500" />
                <p className="text-sm font-medium text-green-700">
                  Successfully connected!
                </p>
              </div>
            )}
          </div>
        )

      default:
        return (
          <div className="text-sm text-muted-foreground">
            This integration requires manual configuration.
          </div>
        )
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
          {/* Header */}
          <div className="flex items-start gap-4">
            <div
              className="flex h-12 w-12 items-center justify-center rounded-lg"
              style={{ backgroundColor: `${color}15` }}
            >
              <IntegrationIcon id={integration.id} className="h-6 w-6" style={{ color }} />
            </div>
            <div className="flex-1">
              <Dialog.Title className="text-lg font-semibold">
                Connect {integration.name}
              </Dialog.Title>
              <Dialog.Description className="text-sm text-muted-foreground">
                {integration.description}
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <Button variant="ghost" size="icon" className="cursor-pointer">
                <X className="h-4 w-4" />
              </Button>
            </Dialog.Close>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="mt-6">
            {error && (
              <div className="mb-4 flex items-start gap-2 rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                {error}
              </div>
            )}

            {success && !error && (
              <div className="mb-4 flex items-start gap-2 rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-700">
                <CheckCircle2 className="h-4 w-4 mt-0.5 shrink-0" />
                Successfully connected!
              </div>
            )}

            {renderForm()}

            {/* Provider Type Info */}
            <div className="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
              <span className="rounded bg-muted px-1.5 py-0.5 font-medium">
                {integration.providerType.replace('_', ' ')}
              </span>
              <span>•</span>
              <span>{integration.authType.replace('_', ' ')}</span>
            </div>

            {/* Actions */}
            <div className="mt-6 flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={onClose}
                disabled={loading || oauthStatus === 'waiting'}
                className="cursor-pointer"
              >
                Cancel
              </Button>
              {!success && (
                <Button 
                  type="submit" 
                  disabled={loading || oauthStatus === 'waiting' || oauthStatus === 'success'} 
                  className="cursor-pointer"
                >
                  {loading || oauthStatus === 'waiting' ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Connecting...
                    </>
                  ) : integration.authType === 'oauth2' ? (
                    <>
                      <ExternalLink className="mr-2 h-4 w-4" />
                      Connect with {integration.name}
                    </>
                  ) : (
                    'Connect'
                  )}
                </Button>
              )}
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// Map integration IDs to Pipedream app slugs
function getAppSlug(integrationId: string): string {
  const mapping: Record<string, string> = {
    // Productivity
    'gmail': 'gmail',
    'google_calendar': 'google_calendar',
    'google_drive': 'google_drive',
    'slack': 'slack',
    'notion': 'notion',
    'asana': 'asana',
    'trello': 'trello',
    'airtable': 'airtable',
    'monday': 'monday',
    'clickup': 'clickup',
    'hubspot': 'hubspot',
    // Developer Tools
    'github': 'github',
    'linear': 'linear_app',
    'jira': 'jira',
    'confluence': 'confluence',
    'discord': 'discord',
    // Communication
    'teams': 'microsoft_teams',
    'outlook': 'microsoft_outlook',
    // Cloud - these typically don't use Pipedream OAuth
    'onedrive': 'onedrive',
    'sharepoint': 'sharepoint',
  }
  return mapping[integrationId] || integrationId
}
