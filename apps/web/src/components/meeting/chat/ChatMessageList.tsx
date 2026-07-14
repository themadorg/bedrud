import { MessageSquare } from 'lucide-react'
import { type DragEvent, useCallback, useEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import type { ChatMessage, SystemMessage } from '../MeetingContext'
import { ChatImageLightbox } from './ChatImageLightbox'
import { ChatMessageCluster } from './ChatMessageCluster'
import { ChatScrollManager } from './ChatScrollManager'
import { groupMessages } from './chatGrouping'
import { systemMessageLabel } from './systemMessageLabel'

interface Props {
  chatMessages: ChatMessage[]
  systemMessages: SystemMessage[]
  currentIdentity: string
  onVotePoll: (messageId: string, optionId: string) => void
  onReactToMessage: (messageId: string, emoji: string) => void
  onScrollUnreadChange: (n: number) => void
  onDrop: (file: File) => void
  elevated?: boolean
}

export function ChatMessageList({
  chatMessages,
  systemMessages,
  currentIdentity,
  onVotePoll,
  onReactToMessage,
  onScrollUnreadChange,
  onDrop,
  elevated = false,
}: Props) {
  const messagesRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const autoFollowRef = useRef(true)
  const prevCountRef = useRef(0)
  const [showScrollBtn, setShowScrollBtn] = useState(false)
  const [scrollUnread, setScrollUnread] = useState(0)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  const handleScroll = useCallback(() => {
    const el = messagesRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
    autoFollowRef.current = atBottom
    setShowScrollBtn(!atBottom)
    if (atBottom) {
      setScrollUnread(0)
      onScrollUnreadChange(0)
    }
  }, [onScrollUnreadChange])

  /** Instant jump to bottom (no smooth animation). */
  const scrollMessagesToBottom = useCallback(() => {
    const el = messagesRef.current
    if (!el) return
    // Double-rAF so layout/images settle before measuring scrollHeight.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const node = messagesRef.current
        if (!node) return
        node.scrollTop = node.scrollHeight
      })
    })
  }, [])

  const totalCount = chatMessages.length + systemMessages.length

  useEffect(() => {
    const delta = totalCount - prevCountRef.current
    if (delta <= 0) return
    prevCountRef.current = totalCount
    if (autoFollowRef.current) {
      scrollMessagesToBottom()
    } else {
      setScrollUnread((n) => {
        const next = n + delta
        onScrollUnreadChange(next)
        return next
      })
    }
  }, [totalCount, onScrollUnreadChange, scrollMessagesToBottom])

  const scrollToBottom = useCallback(() => {
    autoFollowRef.current = true
    setShowScrollBtn(false)
    setScrollUnread(0)
    onScrollUnreadChange(0)
    scrollMessagesToBottom()
  }, [onScrollUnreadChange, scrollMessagesToBottom])

  const handleDrop = useCallback(
    (e: DragEvent<HTMLDivElement>) => {
      e.preventDefault()
      const file = Array.from(e.dataTransfer.files).find((f) => f.type.startsWith('image/'))
      if (file) onDrop(file)
    },
    [onDrop],
  )

  const items = groupMessages(chatMessages, systemMessages)

  return (
    <div className="flex-1 relative overflow-hidden">
      <div
        role="log"
        ref={messagesRef}
        onScroll={handleScroll}
        onDrop={handleDrop}
        onDragOver={(e) => e.preventDefault()}
        className={cn('meet-scroll flex h-full flex-col overflow-y-auto py-2 pl-2.5 pr-1', showScrollBtn && 'pb-14')}
      >
        {items.length === 0 ? (
          <div className="flex-1 flex flex-col items-center justify-center gap-2.5">
            <div
              className="w-11 h-11 rounded-full flex items-center justify-center"
              style={{
                background: 'color-mix(in oklab, var(--primary) 10%, transparent)',
                border: '1px solid color-mix(in oklab, var(--primary) 20%, transparent)',
              }}
            >
              <MessageSquare size={18} className="text-[color-mix(in_oklab,var(--primary)_50%,transparent)]" />
            </div>
            <p className="text-white/50 text-xs text-center">
              No messages yet.
              <br />
              Say hello!
            </p>
          </div>
        ) : (
          items.map((item, index) => {
            const stackGap = index > 0 ? 'mt-3' : undefined

            if (item.kind === 'date-separator') {
              return (
                <div key={item.id} className={cn('flex items-center gap-2.5 py-1', stackGap)}>
                  <div className="flex-1 h-px bg-white/[0.06]" />
                  <span className="text-[11px] text-white/50 font-medium">{item.label}</span>
                  <div className="flex-1 h-px bg-white/[0.06]" />
                </div>
              )
            }

            if (item.kind === 'system') {
              return (
                <div key={item.id} className={cn('flex justify-center py-0.5', stackGap)}>
                  <span className="max-w-[90%] text-center text-[11px] text-white/50 bg-white/[0.05] border border-white/[0.08] rounded-full px-2.5 py-[3px] italic">
                    {systemMessageLabel(item.msg)}
                  </span>
                </div>
              )
            }

            const isSelfCluster = item.isLocal || (!!currentIdentity && item.identity === currentIdentity)

            return (
              <div
                key={item.id}
                className={cn(
                  'w-full',
                  index > 0 && (isSelfCluster ? 'mt-1.5' : 'mt-3'),
                  isSelfCluster && 'flex justify-end',
                )}
              >
                <ChatMessageCluster
                  cluster={item}
                  currentIdentity={currentIdentity}
                  onImageOpen={setPreviewUrl}
                  onVotePoll={onVotePoll}
                  onReactToMessage={onReactToMessage}
                  elevated={elevated}
                />
              </div>
            )
          })
        )}
        <div ref={bottomRef} />
      </div>

      <ChatScrollManager show={showScrollBtn} unreadCount={scrollUnread} onScrollToBottom={scrollToBottom} />

      <ChatImageLightbox url={previewUrl} onClose={() => setPreviewUrl(null)} />
    </div>
  )
}
