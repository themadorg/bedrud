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
      setTimeout(() => {
        setToasts((t) => t.filter((x) => x.id !== id))
      }, 4500)
    })
  }, [chatMessages, chatOpen])

  if (toasts.length === 0) return null

  return (
    <div className="fixed top-[calc(68px+env(safe-area-inset-top))] right-[calc(16px+env(safe-area-inset-right))] z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="chat-toast flex flex-col gap-[5px] bg-[#0f0f1c]/96 rounded-[14px] px-4 py-[13px] shadow-[0_8px_28px_rgba(0,0,0,0.5)] backdrop-blur-lg max-w-[min(340px,calc(100vw-32px))]"
          style={{ border: '1px solid color-mix(in oklab, var(--primary) 35%, transparent)' }}
        >
          <span className="text-[13px] font-semibold text-teal-400">{toast.sender}</span>
          <span className="text-sm text-white/75 overflow-hidden line-clamp-2 break-words">{toast.message}</span>
        </div>
      ))}
    </div>
  )
}
