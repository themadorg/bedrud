import { Pin, X } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import {
  type ChatAttachment,
  type ChatMessage,
  type ChatPoll,
  normalizeChatAttachment,
  type SystemMessage,
} from '@/components/meeting/MeetingContext'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { ChatInput, type ChatInputHandle } from './chat/ChatInput'
import { ChatMessageList } from './chat/ChatMessageList'
import { useFocusTrap } from './useFocusTrap'

interface Props {
  onClose: () => void
  roomId: string
  currentIdentity: string
  chatMessages: ChatMessage[]
  systemMessages: SystemMessage[]
  sendChat: (text: string, attachments?: ChatAttachment[], poll?: ChatPoll) => void
  markRead: () => void
  votePoll: (messageId: string, optionId: string) => void
  reactToMessage: (messageId: string, emoji: string) => void
  stuck?: boolean
  onStuckChange?: (stuck: boolean) => void
}

const headerBtnClass = (active = false) =>
  cn(
    'flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] border-none bg-transparent cursor-pointer transition-[background,color] duration-150',
    active ? 'text-accent-400' : 'text-white/50 hover:text-white/70',
  )

export function ChatPanel({
  onClose,
  roomId,
  currentIdentity,
  chatMessages,
  systemMessages,
  sendChat,
  markRead,
  votePoll,
  reactToMessage,
  stuck = false,
  onStuckChange,
}: Props) {
  const inputRef = useRef<ChatInputHandle>(null)
  const noop = useCallback(() => {}, [])

  useEffect(() => {
    markRead()
    const t = setTimeout(() => inputRef.current?.focus(), 80)
    return () => clearTimeout(t)
  }, [markRead])

  const uploadAndSend = useCallback(
    async (file: File): Promise<ChatAttachment> => {
      const form = new FormData()
      form.append('file', file)
      const raw = await api.post<ChatAttachment>(`/api/room/${roomId}/chat/upload`, form)
      const attachment = normalizeChatAttachment(raw)
      if (!attachment) throw new Error('Invalid upload response')
      return attachment
    },
    [roomId],
  )

  const trapRef = useFocusTrap({ enabled: !stuck, onClose })

  return (
    <aside
      ref={trapRef}
      className={cn(
        'top-0 bottom-0 z-30 flex flex-col bg-[var(--meet-sidebar)] backdrop-blur-2xl pt-[env(safe-area-inset-top)] pb-[calc(env(safe-area-inset-bottom))] transition-[left,right] duration-200',
        stuck
          ? 'fixed right-0 border-l border-[var(--meet-border-subtle)]'
          : 'absolute right-0 border-l border-[var(--meet-border-subtle)]',
      )}
      style={{ width: 'min(320px, 100vw)' }}
    >
      <div className="h-[52px] shrink-0 flex items-center justify-between px-4 border-b border-white/[0.06]">
        <span className="text-base font-semibold text-white/85">Chat</span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => onStuckChange?.(!stuck)}
            className={headerBtnClass(stuck)}
            aria-label={stuck ? 'Unstick chat' : 'Stick chat open'}
            aria-pressed={stuck}
          >
            <Pin size={15} className={stuck ? 'fill-current' : ''} />
          </button>
          <button type="button" onClick={onClose} className={headerBtnClass()} aria-label="Close chat">
            <X size={15} />
          </button>
        </div>
      </div>

      {/* Message list */}
      <ChatMessageList
        chatMessages={chatMessages}
        systemMessages={systemMessages}
        currentIdentity={currentIdentity}
        onVotePoll={votePoll}
        onReactToMessage={reactToMessage}
        onScrollUnreadChange={noop}
        onDrop={(file) => {
          void uploadAndSend(file)
            .then((att) => {
              sendChat('', [att])
              inputRef.current?.focus()
            })
            .catch((err) => {
              const message = err instanceof Error ? err.message : 'Upload failed'
              try {
                const jsonMatch = message.match(/\{.*\}/)
                if (jsonMatch) {
                  const parsed = JSON.parse(jsonMatch[0]) as { error?: string }
                  if (parsed.error) {
                    toast.error(parsed.error)
                    return
                  }
                }
              } catch {
                // ignore parse error
              }
              toast.error(message)
            })
        }}
      />

      {/* Input */}
      <ChatInput ref={inputRef} onSend={sendChat} onUpload={uploadAndSend} />
    </aside>
  )
}
