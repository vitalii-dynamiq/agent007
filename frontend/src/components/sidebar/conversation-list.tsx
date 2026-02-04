import { Plus, MessageSquare, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import type { Conversation } from '@/lib/api'

interface ConversationListProps {
  conversations: Conversation[]
  selectedId?: string
  onSelect: (id: string) => void
  onCreate: () => void
  onDelete: (id: string) => void
}

export function ConversationList({
  conversations,
  selectedId,
  onSelect,
  onCreate,
  onDelete,
}: ConversationListProps) {
  return (
    <div className="flex h-full flex-col bg-muted/50">
      <div className="p-4">
        <Button onClick={onCreate} className="w-full" size="sm">
          <Plus className="mr-2 h-4 w-4" />
          New Chat
        </Button>
      </div>
      
      <ScrollArea className="flex-1 px-2">
        <div className="space-y-1 pb-4 w-full">
          {conversations.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-muted-foreground">
              No conversations yet
            </p>
          ) : (
            conversations.map((conv) => (
              <div
                key={conv.id}
                className={cn(
                  "flex items-center gap-2 rounded-lg px-3 py-2 text-sm",
                  "transition-colors hover:bg-accent cursor-pointer",
                  "w-full",
                  selectedId === conv.id && "bg-accent"
                )}
                onClick={() => onSelect(conv.id)}
              >
                <MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="flex-1 truncate min-w-0">
                  {conv.title || 'New conversation'}
                </span>
                <button
                  className="shrink-0 p-1 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive cursor-pointer"
                  onClick={(e) => {
                    e.stopPropagation()
                    onDelete(conv.id)
                  }}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
