import { MessageSquare, X } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import type { ChatAttachment } from '#/components/meeting/MeetingContext'
import { useMeetingChatContext, useMeetingRoomContext } from '#/components/meeting/MeetingContext'
import { api } from '#/lib/api'
import { ChatInput, type ChatInputHandle } from './chat/ChatInput'
import { ChatMessageList } from './chat/ChatMessageList'
import { useFocusTrap } from './useFocusTrap'

interface Props {
  onClose: () => void
}

export function ChatPanel({ onClose }: Props) {
  const { roomId } = useMeetingRoomContext()
  const { chatMessages, systemMessages, sendChat, markRead } = useMeetingChatContext()
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
      return api.post<ChatAttachment>(`/api/room/${roomId}/chat/upload`, form)
    },
    [roomId],
  )

  const trapRef = useFocusTrap({ enabled: true, onClose })

  return (
    <aside
      ref={trapRef}
      className="absolute right-0 top-0 bottom-0 z-30 flex flex-col bg-[#0a0a16]/94 backdrop-blur-2xl border-l border-white/[0.07] pt-[env(safe-area-inset-top)] pb-[calc(env(safe-area-inset-bottom))]"
      style={{ width: 'min(320px, 100vw)' }}
    >
      {/* Header */}
      <div className="h-[52px] shrink-0 flex items-center justify-between px-4 border-b border-white/[0.06]">
        <div className="flex items-center gap-[7px]">
          <MessageSquare size={14} className="text-[color-mix(in_oklab,var(--accent-400)_70%,transparent)]" />
          <span className="text-white/80 text-[13px] font-semibold">Chat</span>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="w-7 h-7 rounded-[7px] bg-transparent border-none flex items-center justify-center text-white/50 cursor-pointer"
          aria-label="Close chat"
        >
          <X size={15} />
        </button>
      </div>

      {/* Message list */}
      <ChatMessageList
        chatMessages={chatMessages}
        systemMessages={systemMessages}
        onScrollUnreadChange={noop}
        onDrop={(file) => {
          void uploadAndSend(file)
            .then((att) => sendChat('', [att]))
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
