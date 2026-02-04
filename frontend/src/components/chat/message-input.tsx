import { useState, useRef, useEffect, type KeyboardEvent, type ChangeEvent } from 'react'
import { Send, Loader2, Mic, Paperclip, X, FileText, FileSpreadsheet, FileImage, File } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { VoiceRecorder } from './voice-recorder'
import { ToolSelector } from './tool-selector'

export interface UploadedFile {
  name: string
  size: number
  type: string
  data: string // base64 encoded
}

export interface Tool {
  id: string
  name: string
  icon?: string
  connected: boolean
  enabled: boolean
}

interface MessageInputProps {
  onSend: (content: string, files?: UploadedFile[]) => void
  disabled?: boolean
  isLoading?: boolean
  tools?: Tool[]
  onToolToggle?: (toolId: string, enabled: boolean) => void
  onManageIntegrations?: () => void
}

// Get appropriate icon for file type
function getFileIcon(type: string) {
  if (type.startsWith('image/')) return FileImage
  if (type.includes('spreadsheet') || type.includes('csv') || type.includes('excel')) return FileSpreadsheet
  if (type.includes('text') || type.includes('json') || type.includes('pdf')) return FileText
  return File
}

// Format file size
function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function MessageInput({ 
  onSend, 
  disabled, 
  isLoading,
  tools = [],
  onToolToggle,
  onManageIntegrations,
  initialValue = '',
  onValueChange
}: MessageInputProps & { 
  initialValue?: string
  onValueChange?: (value: string) => void 
}) {
  const [value, setValue] = useState(initialValue)
  
  // Sync with initialValue when it changes
  useEffect(() => {
    if (initialValue) {
      setValue(initialValue)
    }
  }, [initialValue])
  const [files, setFiles] = useState<UploadedFile[]>([])
  const [showVoiceRecorder, setShowVoiceRecorder] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Auto-resize textarea
  useEffect(() => {
    const textarea = textareaRef.current
    if (textarea) {
      textarea.style.height = 'auto'
      textarea.style.height = `${Math.min(textarea.scrollHeight, 200)}px`
    }
  }, [value])

  const handleFileSelect = async (e: ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = e.target.files
    if (!selectedFiles) return

    const newFiles: UploadedFile[] = []
    
    for (let i = 0; i < selectedFiles.length; i++) {
      const file = selectedFiles[i]
      // Read file as base64
      const data = await new Promise<string>((resolve) => {
        const reader = new FileReader()
        reader.onload = () => {
          const result = reader.result as string
          // Remove data URL prefix (e.g., "data:text/plain;base64,")
          const base64 = result.split(',')[1] || result
          resolve(base64)
        }
        reader.readAsDataURL(file)
      })

      newFiles.push({
        name: file.name,
        size: file.size,
        type: file.type || 'application/octet-stream',
        data,
      })
    }

    setFiles(prev => [...prev, ...newFiles])
    // Reset input so same file can be selected again
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const removeFile = (index: number) => {
    setFiles(prev => prev.filter((_, i) => i !== index))
  }

  const handleSend = () => {
    if ((value.trim() || files.length > 0) && !disabled) {
      onSend(value.trim(), files.length > 0 ? files : undefined)
      setValue('')
      setFiles([])
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const canSend = value.trim() || files.length > 0

  return (
    <div className="border-t bg-background p-4">
      {/* Selected files preview */}
      {files.length > 0 && (
        <div className="mx-auto max-w-4xl mb-3">
          <div className="flex flex-wrap gap-2">
            {files.map((file, index) => {
              const FileIcon = getFileIcon(file.type)
              return (
                <div
                  key={`${file.name}-${index}`}
                  className={cn(
                    "flex items-center gap-2 px-3 py-1.5 rounded-lg",
                    "bg-muted/70 border border-border/50",
                    "text-sm text-foreground/80"
                  )}
                >
                  <FileIcon className="h-4 w-4 text-muted-foreground" />
                  <span className="truncate max-w-[150px]">{file.name}</span>
                  <span className="text-xs text-muted-foreground">
                    {formatFileSize(file.size)}
                  </span>
                  <button
                    onClick={() => removeFile(index)}
                    className="ml-1 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </div>
              )
            })}
          </div>
        </div>
      )}

      <div className="mx-auto max-w-4xl">
        {/* Text input with integrated buttons */}
        <div className="relative flex items-end rounded-xl border bg-muted/50 focus-within:ring-2 focus-within:ring-ring focus-within:bg-background transition-colors">
          {/* Left side buttons */}
          <div className="flex items-center gap-1 pl-2 pb-2">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground hover:bg-transparent cursor-pointer"
              title="Upload files"
              disabled={disabled}
              onClick={() => fileInputRef.current?.click()}
            >
              <Paperclip className="h-5 w-5" />
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={handleFileSelect}
              accept=".csv,.json,.txt,.md,.py,.js,.ts,.sql,.xlsx,.xls,.pdf,.png,.jpg,.jpeg,.gif,.webp"
            />

            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground hover:bg-transparent cursor-pointer"
              title="Voice input"
              disabled={disabled}
              onClick={() => setShowVoiceRecorder(true)}
            >
              <Mic className="h-5 w-5" />
            </Button>
          </div>
          
          {/* Text input */}
          <textarea
            ref={textareaRef}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={files.length > 0 ? "Add a message or press send..." : "Ask me anything..."}
            disabled={disabled}
            rows={1}
            className={cn(
              "flex-1 resize-none bg-transparent px-2 py-3.5 text-sm",
              "focus:outline-none",
              "disabled:opacity-50",
              "placeholder:text-muted-foreground/60"
            )}
            style={{
              minHeight: '52px',
              maxHeight: '200px',
            }}
          />
          
          {/* Send button */}
          <div className="pr-2 pb-2">
            <Button
              onClick={handleSend}
              disabled={!canSend || disabled}
              size="icon"
              className={cn(
                "h-9 w-9 rounded-lg cursor-pointer",
                "transition-all duration-200",
                canSend ? "opacity-100 scale-100" : "opacity-50 scale-95"
              )}
            >
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Send className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
      </div>
      
      {/* Tool selector row */}
      {(tools.length > 0 || onManageIntegrations) && (
        <div className="mx-auto max-w-4xl mt-3 flex items-center justify-between">
          <ToolSelector
            tools={tools}
            onToggle={onToolToggle || (() => {})}
            onManageIntegrations={onManageIntegrations || (() => {})}
          />
          <p className="text-xs text-muted-foreground/50">
            Enter to send · Shift+Enter for new line
          </p>
        </div>
      )}

      {/* Helper text (only show if no tools) */}
      {tools.length === 0 && !onManageIntegrations && !showVoiceRecorder && (
        <p className="text-center text-xs text-muted-foreground/50 mt-2">
          Press Enter to send, Shift+Enter for new line
        </p>
      )}

      {/* Inline Voice Recorder */}
      {showVoiceRecorder && (
        <div className="mx-auto max-w-4xl mt-3 flex justify-center">
          <VoiceRecorder
            onTranscription={(text) => {
              // Append transcribed text to current value
              setValue(prev => prev ? `${prev} ${text}` : text)
              // Focus textarea after transcription
              textareaRef.current?.focus()
            }}
            onClose={() => setShowVoiceRecorder(false)}
            disabled={disabled}
          />
        </div>
      )}
    </div>
  )
}
