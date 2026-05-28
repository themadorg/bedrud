import { Mic, Video, X } from 'lucide-react'
import { useState } from 'react'
import { useMeetingChatContext, useMeetingRoomContext } from '@/components/meeting/MeetingContext'

/** Shows a banner when a moderator asks the user to unmute or enable camera. */
export function AskActionBanner() {
  const { systemMessages } = useMeetingChatContext()
  const { currentUserId } = useMeetingRoomContext()
  const [dismissed, setDismissed] = useState<number>(0)

  const lastAsk = [...systemMessages]
    .reverse()
    .find((m) => (m.event === 'ask_unmute' || m.event === 'ask_camera') && m.target === currentUserId)

  if (!lastAsk || lastAsk.ts <= dismissed) return null

  const isUnmute = lastAsk.event === 'ask_unmute'

  return (
    <div
      role="alert"
      className="fixed bottom-[calc(100px+env(safe-area-inset-bottom))] left-1/2 -translate-x-1/2 z-60 flex items-center gap-3 bg-[#0f0f1e]/95 rounded-xl px-4 py-3 shadow-[0_8px_32px_rgba(0,0,0,0.4)] backdrop-blur-lg max-w-[min(340px,calc(100vw-32px))]"
      style={{ border: '1px solid color-mix(in oklab, var(--primary) 40%, transparent)' }}
    >
      <div
        className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
        style={{
          background: 'color-mix(in oklab, var(--primary) 15%, transparent)',
          border: '1px solid color-mix(in oklab, var(--primary) 30%, transparent)',
        }}
      >
        {isUnmute ? <Mic size={15} className="text-teal-400" /> : <Video size={15} className="text-teal-400" />}
      </div>
      <span className="text-white/80 text-[13px] flex-1">
        {isUnmute ? 'A moderator is asking you to unmute.' : 'A moderator is asking you to turn on your camera.'}
      </span>
      <button
        type="button"
        onClick={() => setDismissed(lastAsk.ts)}
        className="bg-none border-none p-1 cursor-pointer text-white/50 shrink-0 flex items-center"
        aria-label="Dismiss"
      >
        <X size={14} />
      </button>
    </div>
  )
}
