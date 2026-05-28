import { useParticipants } from '@livekit/components-react'
import { Info, MessageSquare, Users } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { ChatToastNotifier } from '@/components/meeting/ChatToastNotifier'
import { useMeetingChatContext, useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { MeetingControls } from '@/components/meeting/MeetingControls'
import { RoomInfoPanel } from '@/components/meeting/RoomInfoPanel'

// Lazy-loaded panels: only fetched when the user opens them.
const ChatPanel = lazy(() => import('@/components/meeting/ChatPanel').then((m) => ({ default: m.ChatPanel })))
const ParticipantsList = lazy(() =>
  import('@/components/meeting/ParticipantsList').then((m) => ({ default: m.ParticipantsList })),
)

interface MeetingPanelsProps {
  navigate: () => void
}

/** Manages side panels (chat, participants, room info), their toggle buttons, toast notifications, and controls. */
export function MeetingPanels({ navigate }: MeetingPanelsProps) {
  const [chatOpen, setChatOpen] = useState(false)
  const [participantsOpen, setParticipantsOpen] = useState(false)
  const [infoOpen, setInfoOpen] = useState(false)

  const toggleChat = () => {
    setChatOpen((o) => !o)
    setParticipantsOpen(false)
    setInfoOpen(false)
  }
  const toggleParticipants = () => {
    setParticipantsOpen((o) => !o)
    setChatOpen(false)
    setInfoOpen(false)
  }
  const toggleInfo = () => {
    setInfoOpen((o) => !o)
    setChatOpen(false)
    setParticipantsOpen(false)
  }

  const { roomId } = useMeetingRoomContext()

  return (
    <>
      {/* Top-left: Participants button */}
      <ParticipantsToggle isOpen={participantsOpen} onToggle={toggleParticipants} />

      {/* Top-right: Chat button */}
      <ChatToggle isOpen={chatOpen} onToggle={toggleChat} />

      {/* Info toggle — between chat and participants on right */}
      <InfoToggle isOpen={infoOpen} onToggle={toggleInfo} />

      {participantsOpen && !chatOpen && !infoOpen && (
        <Suspense fallback={null}>
          <ParticipantsList onClose={() => setParticipantsOpen(false)} />
        </Suspense>
      )}
      {chatOpen && (
        <Suspense fallback={null}>
          <ChatPanel onClose={() => setChatOpen(false)} />
        </Suspense>
      )}
      {infoOpen && <RoomInfoPanel roomId={roomId} onClose={() => setInfoOpen(false)} />}
      <ChatToastNotifier chatOpen={chatOpen} />
      <MeetingControls onNavigate={navigate} />
    </>
  )
}

/* ── Top-left: Participants toggle button ─────────────────────── */

function ParticipantsToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  const participants = useParticipants()
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute z-[25] flex items-center gap-1.5 rounded-xl px-3 py-[7px] text-xs font-semibold cursor-pointer backdrop-blur-lg transition-all duration-150"
      style={{
        top: 'calc(14px + env(safe-area-inset-top, 0px))',
        left: 'calc(14px + env(safe-area-inset-left, 0px))',
        background: isOpen ? 'color-mix(in oklab, var(--primary) 25%, transparent)' : 'rgba(12,12,22,0.7)',
        border: `1px solid ${isOpen ? 'color-mix(in oklab, var(--primary) 40%, transparent)' : 'rgba(255,255,255,0.08)'}`,
        color: isOpen ? 'var(--accent-400)' : 'rgba(255,255,255,0.55)',
      }}
      aria-label={isOpen ? 'Close participants' : 'Show participants'}
    >
      <Users size={14} />
      <span>{participants.length}</span>
    </button>
  )
}

/* ── Top-right: Info toggle button ────────────────────────────── */

function InfoToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute z-[25] w-[38px] h-[38px] flex items-center justify-center rounded-xl cursor-pointer backdrop-blur-lg transition-all duration-150"
      style={{
        top: 'calc(14px + env(safe-area-inset-top, 0px))',
        right: 'calc(14px + 38px + 8px + env(safe-area-inset-right, 0px))',
        background: isOpen ? 'color-mix(in oklab, var(--primary) 25%, transparent)' : 'rgba(12,12,22,0.7)',
        border: `1px solid ${isOpen ? 'color-mix(in oklab, var(--primary) 40%, transparent)' : 'rgba(255,255,255,0.08)'}`,
        color: isOpen ? 'var(--accent-400)' : 'rgba(255,255,255,0.55)',
      }}
      aria-label={isOpen ? 'Close room info' : 'Show room info'}
    >
      <Info size={16} />
    </button>
  )
}

/* ── Top-right: Chat toggle button ────────────────────────────── */

function ChatToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  const { unreadCount } = useMeetingChatContext()
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute z-[25] w-[38px] h-[38px] flex items-center justify-center rounded-xl cursor-pointer backdrop-blur-lg transition-all duration-150"
      style={{
        top: 'calc(14px + env(safe-area-inset-top, 0px))',
        right: 'calc(14px + env(safe-area-inset-right, 0px))',
        background: isOpen ? 'color-mix(in oklab, var(--primary) 25%, transparent)' : 'rgba(12,12,22,0.7)',
        border: `1px solid ${isOpen ? 'color-mix(in oklab, var(--primary) 40%, transparent)' : 'rgba(255,255,255,0.08)'}`,
        color: isOpen ? 'var(--accent-400)' : 'rgba(255,255,255,0.55)',
      }}
      aria-label={isOpen ? 'Close chat' : `Open chat${unreadCount > 0 ? ` (${unreadCount} unread)` : ''}`}
    >
      <MessageSquare size={16} />
      {unreadCount > 0 && !isOpen && (
        <span className="absolute -top-1 -right-1 min-w-4 h-4 rounded-full bg-primary text-white text-[9px] font-bold leading-4 text-center px-1 pointer-events-none">
          {unreadCount > 99 ? '99+' : unreadCount}
        </span>
      )}
    </button>
  )
}
