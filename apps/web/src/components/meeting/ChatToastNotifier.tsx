import { useEffect, useRef, useState } from 'react'
import { useMeetingChatContext } from '@/components/meeting/MeetingContext'

interface ChatToast {
  id: number
  sender: string
  message: string
}

interface ChatToastNotifierProps {
  chatOpen: boolean
}

/** Shows floating toast notifications for new chat messages when the chat panel is closed. */
export function ChatToastNotifier({ chatOpen }: ChatToastNotifierProps) {
  const { chatMessages } = useMeetingChatContext()
  const seenRef = useRef(chatMessages.length)
  const [toasts, setToasts] = useState<ChatToast[]>([])
  const nextId = useRef(0)
  const timeoutRefs = useRef<Set<ReturnType<typeof setTimeout>>>(new Set())

  useEffect(() => {
    // On first mount, mark all existing messages as seen without toasting
    seenRef.current = chatMessages.length
  }, [chatMessages.length]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (chatMessages.length <= seenRef.current) return
    const newMsgs = chatMessages.slice(seenRef.current)
    seenRef.current = chatMessages.length
    if (chatOpen) return // panel is open — user can see the messages

    newMsgs.forEach((msg) => {
      const id = nextId.current++
      const sender = msg.senderName || 'Someone'
      setToasts((t) => [...t.slice(-3), { id, sender, message: msg.message }])
      const timer = setTimeout(() => {
        setToasts((t) => t.filter((x) => x.id !== id))
        timeoutRefs.current.delete(timer)
      }, 4500)
      timeoutRefs.current.add(timer)
    })

    return () => {
      // Clear any pending auto-dismiss timeouts on unmount or before re-run
      for (const t of timeoutRefs.current) clearTimeout(t)
      timeoutRefs.current.clear()
    }
  }, [chatMessages, chatOpen])

  if (toasts.length === 0) return null

  return (
    <div
      className="pointer-events-none fixed z-50 flex flex-col gap-2"
      style={{
        top: 'calc(var(--app-offset-top, 0px) + 68px + env(safe-area-inset-top, 0px))',
        // Pin to the right edge of the *visual* viewport (not layout 100vw).
        left: 'calc(var(--app-offset-left, 0px) + var(--app-width, 100svw) - 16px)',
        transform: 'translateX(-100%)',
      }}
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="chat-toast flex max-w-[min(340px,calc(var(--app-width,100svw)-32px))] flex-col gap-[5px] rounded-[14px] bg-[#0f0f1c]/96 px-4 py-[13px] shadow-[0_8px_28px_rgba(0,0,0,0.5)] backdrop-blur-lg"
          style={{ border: '1px solid color-mix(in oklab, var(--primary) 35%, transparent)' }}
        >
          <span className="text-[13px] font-semibold text-teal-400">{toast.sender}</span>
          <span className="text-sm text-white/75 overflow-hidden line-clamp-2 break-words">{toast.message}</span>
        </div>
      ))}
    </div>
  )
}
