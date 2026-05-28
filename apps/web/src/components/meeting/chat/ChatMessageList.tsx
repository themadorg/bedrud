import { MessageSquare } from 'lucide-react'
import { type DragEvent, useCallback, useEffect, useRef, useState } from 'react'
import type { ChatMessage, SystemMessage } from '../MeetingContext'
import { ChatMessageCluster } from './ChatMessageCluster'
import { ChatScrollManager } from './ChatScrollManager'
import { groupMessages } from './chatGrouping'

interface Props {
  chatMessages: ChatMessage[]
  systemMessages: SystemMessage[]
  onScrollUnreadChange: (n: number) => void
  onDrop: (file: File) => void
}

export function ChatMessageList({ chatMessages, systemMessages, onScrollUnreadChange, onDrop }: Props) {
  const messagesRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const autoFollowRef = useRef(true)
  const prevCountRef = useRef(0)
  const [showScrollBtn, setShowScrollBtn] = useState(false)
  const [scrollUnread, setScrollUnread] = useState(0)

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

  const totalCount = chatMessages.length + systemMessages.length

  useEffect(() => {
    const delta = totalCount - prevCountRef.current
    if (delta <= 0) return
    prevCountRef.current = totalCount
    if (autoFollowRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    } else {
      setScrollUnread((n) => {
        const next = n + delta
        onScrollUnreadChange(next)
        return next
      })
    }
  }, [totalCount, onScrollUnreadChange])

  const scrollToBottom = useCallback(() => {
    autoFollowRef.current = true
    setShowScrollBtn(false)
    setScrollUnread(0)
    onScrollUnreadChange(0)
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [onScrollUnreadChange])

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
        className="h-full overflow-y-auto px-3.5 py-3 flex flex-col gap-2.5 meet-chat-scroll"
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
          items.map((item) => {
            if (item.kind === 'date-separator') {
              return (
                <div key={item.id} className="flex items-center gap-2.5 py-1">
                  <div className="flex-1 h-px bg-white/[0.06]" />
                  <span className="text-[11px] text-white/50 font-medium">{item.label}</span>
                  <div className="flex-1 h-px bg-white/[0.06]" />
                </div>
              )
            }

            if (item.kind === 'system') {
              const label = item.msg.event === 'kick' ? 'was kicked by' : 'was banned by'
              return (
                <div key={item.id} className="flex justify-center py-0.5">
                  <span className="text-[11px] text-white/50 bg-white/[0.05] border border-white/[0.08] rounded-full px-2.5 py-[3px] italic">
                    {item.msg.target} {label} {item.msg.actor}
                  </span>
                </div>
              )
            }

            return <ChatMessageCluster key={item.id} cluster={item} />
          })
        )}
        <div ref={bottomRef} />
      </div>

      <ChatScrollManager show={showScrollBtn} unreadCount={scrollUnread} onScrollToBottom={scrollToBottom} />
    </div>
  )
}
