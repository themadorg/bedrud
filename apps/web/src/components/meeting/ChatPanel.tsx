import { Pin, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  type ChatAttachment,
  type ChatMessage,
  type ChatPoll,
  normalizeChatAttachment,
  type SystemMessage,
} from '@/components/meeting/MeetingContext'
import { MeetingElevatedLeftDock } from '@/components/meeting/MeetingElevatedLeftDock'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { ChatInput, type ChatInputHandle } from './chat/ChatInput'
import { ChatMessageList } from './chat/ChatMessageList'
import { useFocusTrap } from './useFocusTrap'

/** Matches Tailwind `sm` and ControlsBar mobile breakpoint (640px). */
const MOBILE_MAX_WIDTH_MQ = '(max-width: 639px)'

/**
 * Unpinned overlay above stage WebXDC (body z-15) and screen-share shells (z-5).
 * Must portal to `document.body` so it is not trapped under meet-room stacking.
 */
const OVERLAY_Z = 40

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
  /** Desktop dock edge. Default right. Left used when opened from expanded WebXDC. */
  side?: 'left' | 'right'
  /** Stack above expanded WebXDC (z-200) — must portal to body. */
  elevated?: boolean
}

const headerBtnClass = (active = false) =>
  cn(
    'flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] border-none bg-transparent cursor-pointer transition-[background,color] duration-150',
    active ? 'text-accent-400' : 'text-white/50 hover:text-white/70',
  )

function useIsMobileChat() {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(MOBILE_MAX_WIDTH_MQ).matches : false,
  )
  useEffect(() => {
    const mq = window.matchMedia(MOBILE_MAX_WIDTH_MQ)
    const onChange = () => setIsMobile(mq.matches)
    onChange()
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])
  return isMobile
}

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
  side = 'right',
  elevated = false,
}: Props) {
  const inputRef = useRef<ChatInputHandle>(null)
  const noop = useCallback(() => {}, [])
  const isMobile = useIsMobileChat()
  // Mobile is always a full-screen modal — never dock/stick.
  const isDocked = stuck && !isMobile
  const fromLeft = side === 'left' && !isMobile
  // Unpinned (or mobile): overlay stage content. Elevated: expand-mode left dock.
  const isOverlay = !isDocked

  useEffect(() => {
    markRead()
    const t = setTimeout(() => inputRef.current?.focus(), 80)
    return () => clearTimeout(t)
  }, [markRead])

  // Clear pin/dock when entering mobile so desktop stick state does not leak.
  useEffect(() => {
    if (isMobile && stuck) onStuckChange?.(false)
  }, [isMobile, stuck, onStuckChange])

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

  // Modal / overlay / elevated: trap focus. Docked desktop sidebar: leave focus free.
  const trapRef = useFocusTrap({ enabled: isOverlay || elevated, onClose })

  const body = (
    <>
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-white/[0.06] px-3 sm:h-[52px] sm:px-4">
        <span className="text-base font-semibold text-white/85">Chat</span>
        <div className="flex items-center gap-1 sm:gap-2">
          {!elevated && (
            <button
              type="button"
              onClick={() => onStuckChange?.(!stuck)}
              className={cn(headerBtnClass(stuck), 'max-sm:hidden')}
              aria-label={stuck ? 'Unstick chat' : 'Stick chat open'}
              aria-pressed={stuck}
            >
              <Pin size={15} className={stuck ? 'fill-current' : ''} />
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className={cn(headerBtnClass(), 'h-11 w-11 max-sm:rounded-lg')}
            aria-label="Close chat"
          >
            <X size={18} />
          </button>
        </div>
      </div>

      <ChatMessageList
        chatMessages={chatMessages}
        systemMessages={systemMessages}
        currentIdentity={currentIdentity}
        onVotePoll={votePoll}
        onReactToMessage={reactToMessage}
        onScrollUnreadChange={noop}
        onDrop={(file) => {
          inputRef.current?.attachFile(file)
        }}
      />

      <ChatInput ref={inputRef} onSend={sendChat} onUpload={uploadAndSend} />
    </>
  )

  // Expanded WebXDC: shared left dock (same shell/size as settings + room info).
  if (elevated) {
    return (
      <MeetingElevatedLeftDock label="Chat" marker="chat" shellRef={trapRef}>
        {body}
      </MeetingElevatedLeftDock>
    )
  }

  const panel = (
    <aside
      ref={trapRef}
      role="dialog"
      aria-modal={isOverlay}
      aria-label="Chat"
      data-chat-overlay={isOverlay ? 'true' : undefined}
      style={
        isOverlay
          ? {
              // Above stage WebXDC (z-15) and share shell when unpinned.
              zIndex: OVERLAY_Z,
            }
          : undefined
      }
      className={cn(
        'flex flex-col bg-[var(--meet-sidebar)] backdrop-blur-2xl transition-[left,right,width,top,height] duration-200',
        'z-40',
        // Mobile: full-screen on *visual* viewport (iOS Safari toolbar-safe).
        'fixed left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] w-[var(--app-width,100svw)] max-h-[var(--app-height,100svh)] max-w-[var(--app-width,100svw)]',
        'pt-[env(safe-area-inset-top,0px)]',
        // Desktop: 320px sidebar — always `fixed` so overlay can sit above body-portaled stage apps.
        'sm:fixed sm:top-0 sm:h-full sm:max-h-none sm:w-[min(320px,var(--app-width,100svw))] sm:max-w-none sm:pt-[env(safe-area-inset-top,0px)] sm:pb-[env(safe-area-inset-bottom,0px)]',
        fromLeft
          ? 'sm:left-0 sm:right-auto sm:border-r sm:border-[var(--meet-border-subtle)]'
          : 'sm:left-auto sm:right-0 sm:border-l sm:border-[var(--meet-border-subtle)]',
        isOverlay && 'shadow-2xl',
      )}
    >
      {body}
    </aside>
  )

  // Portal unpinned overlay so chat stacks above body-portaled WebXDC / stage chrome.
  // Pinned dock stays in-tree (stage is inset for it).
  if (isOverlay && typeof document !== 'undefined') {
    return createPortal(panel, document.body)
  }

  return panel
}
