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
import { MeetingElevatedPanelBody, MeetingElevatedPanelHeader } from '@/components/meeting/MeetingElevatedPanelChrome'
import { useMeetingExpandChromeHandlers } from '@/components/meeting/meeting-expand-chrome-context'
import { BedrudSheet, SHEET_SNAP_POINTS } from '@/components/ui/BedrudSheet'
import { shouldExpandForKeyboard } from '@/components/ui/sheetKeyboard'
import { api } from '@/lib/api'
import { useIsMobile } from '@/lib/use-is-mobile'
import { cn } from '@/lib/utils'
import { ChatInput, type ChatInputHandle } from './chat/ChatInput'
import { ChatMessageList } from './chat/ChatMessageList'
import { chatSurfaceFor } from './chatSurface'
import { useFocusTrap } from './useFocusTrap'

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

/**
 * Takes the sheet to full when the keyboard opens.
 *
 * A keyboard over a half-open sheet leaves a sliver of conversation, so focusing the composer
 * expands it. The Visual Viewport `resize` event is what an on-screen keyboard fires, and
 * `lib/visual-viewport.ts` already listens to it for `--app-height`; this reads the same signal
 * rather than guessing from focus events, which also fire for a hardware keyboard where nothing
 * needs to move.
 */
function useKeyboardExpandsSheet(enabled: boolean, onExpand: () => void): void {
  useEffect(() => {
    const viewport = window.visualViewport
    if (!enabled || !viewport) return

    let previousHeight = viewport.height
    const handleResize = () => {
      const nextHeight = viewport.height
      if (shouldExpandForKeyboard(previousHeight, nextHeight)) onExpand()
      previousHeight = nextHeight
    }

    viewport.addEventListener('resize', handleResize)
    return () => viewport.removeEventListener('resize', handleResize)
  }, [enabled, onExpand])
}

const headerBtnClass = (active = false) =>
  cn(
    'flex h-7 w-7 shrink-0 items-center justify-center rounded-sm border-none bg-transparent cursor-pointer transition-[background,color] duration-150',
    active
      ? 'text-[var(--meet-accent)]'
      : 'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)]',
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
  side = 'right',
  elevated = false,
}: Props) {
  const inputRef = useRef<ChatInputHandle>(null)
  const noop = useCallback(() => {}, [])
  const isMobile = useIsMobile()
  const { closeChat: closeElevatedChat } = useMeetingExpandChromeHandlers()
  const handleClose = elevated ? closeElevatedChat : onClose
  const surface = chatSurfaceFor({ isMobile, stuck, elevated })
  const fromLeft = side === 'left' && surface !== 'sheet'
  const isOverlay = surface === 'overlay'

  const [snapPoint, setSnapPoint] = useState<number | string | null>(SHEET_SNAP_POINTS[0])
  const expandSheet = useCallback(() => setSnapPoint(SHEET_SNAP_POINTS[SHEET_SNAP_POINTS.length - 1]), [])
  useKeyboardExpandsSheet(surface === 'sheet', expandSheet)

  useEffect(() => {
    markRead()
    const t = setTimeout(() => inputRef.current?.focus(), 80)
    return () => clearTimeout(t)
  }, [markRead])

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

  const trapRef = useFocusTrap({ enabled: isOverlay || elevated, onClose: handleClose })

  const chatBody = (
    <>
      <ChatMessageList
        chatMessages={chatMessages}
        systemMessages={systemMessages}
        currentIdentity={currentIdentity}
        onVotePoll={votePoll}
        onReactToMessage={reactToMessage}
        onScrollUnreadChange={noop}
        elevated={elevated}
        onDrop={(file) => {
          inputRef.current?.attachFile(file)
        }}
      />
      <ChatInput ref={inputRef} onSend={sendChat} onUpload={uploadAndSend} elevated={elevated} />
    </>
  )

  const body = (
    <>
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-[var(--meet-border-subtle)] px-3 lg:h-[52px] lg:px-4">
        <span className="text-base font-semibold text-[var(--meet-fg-strong)]">Chat</span>
        <div className="flex items-center gap-1 lg:gap-2">
          {!elevated && (
            <button
              type="button"
              onClick={() => onStuckChange?.(!stuck)}
              className={cn(headerBtnClass(stuck), 'max-lg:hidden')}
              aria-label={stuck ? 'Unstick chat' : 'Stick chat open'}
              aria-pressed={stuck}
            >
              <Pin size={15} className={stuck ? 'fill-current' : ''} />
            </button>
          )}
          <button
            type="button"
            onClick={handleClose}
            className={cn(headerBtnClass(), 'h-11 w-11 max-lg:rounded-lg')}
            aria-label="Close chat"
          >
            <X size={18} />
          </button>
        </div>
      </div>
      {chatBody}
    </>
  )

  if (elevated) {
    return (
      <MeetingElevatedLeftDock label="Chat" marker="chat" shellRef={trapRef}>
        <MeetingElevatedPanelHeader title="Chat" onClose={handleClose} closeLabel="Close chat" />
        <MeetingElevatedPanelBody>{chatBody}</MeetingElevatedPanelBody>
      </MeetingElevatedLeftDock>
    )
  }

  // Phones open chat as a sheet over the live call rather than as a screen that replaces it, so the
  // call stays visible above it. The sheet renders the full panel body, close button included.
  if (surface === 'sheet') {
    return (
      <BedrudSheet
        open
        onOpenChange={(nextOpen) => {
          if (!nextOpen) handleClose()
        }}
        label="Chat"
        activeSnapPoint={snapPoint}
        onSnapPointChange={setSnapPoint}
        dataMarkers={{ 'data-chat-overlay': 'true' }}
      >
        {body}
      </BedrudSheet>
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
        'meet-dialog flex flex-col bg-[var(--meet-sidebar)] backdrop-blur-2xl transition-[left,right,width,top,height] duration-200',
        'z-40',
        // Mobile: full-screen on *visual* viewport (iOS Safari toolbar-safe).
        'fixed left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] w-[var(--app-width,100svw)] max-h-[var(--app-height,100svh)] max-w-[var(--app-width,100svw)]',
        'pt-[env(safe-area-inset-top,0px)]',
        // Desktop: 320px sidebar — always `fixed` so overlay can sit above body-portaled stage apps.
        'lg:fixed lg:top-0 lg:h-full lg:max-h-none lg:w-[min(320px,var(--app-width,100svw))] lg:max-w-none lg:pt-[env(safe-area-inset-top,0px)] lg:pb-[env(safe-area-inset-bottom,0px)]',
        fromLeft
          ? 'lg:left-0 lg:right-auto lg:border-r lg:border-[var(--meet-border-subtle)]'
          : 'lg:left-auto lg:right-0 lg:border-l lg:border-[var(--meet-border-subtle)]',
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
