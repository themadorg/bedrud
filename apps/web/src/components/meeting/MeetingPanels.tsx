import { useParticipants, useRoomContext } from '@livekit/components-react'
import { Globe, Lock, MessageSquare, Users, Video } from 'lucide-react'
import { useState } from 'react'
import { cn } from '#/lib/utils'
import { ChatPanel } from '@/components/meeting/ChatPanel'
import { ChatToastNotifier } from '@/components/meeting/ChatToastNotifier'
import { useMeetingChatContext, useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { MeetingControls } from '@/components/meeting/MeetingControls'
import { ParticipantsList } from '@/components/meeting/ParticipantsList'
import { RoomAccessDialog } from '@/components/meeting/RoomAccessDialog'
import { RoomInfoPanel } from '@/components/meeting/RoomInfoPanel'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'

interface MeetingPanelsProps {
  navigate: () => void
  chatOpen: boolean
  setChatOpen: (open: boolean | ((prev: boolean) => boolean)) => void
  chatStuck: boolean
  setChatStuck: (stuck: boolean) => void
  videoSidebarOpen: boolean
  onToggleVideoSidebar: () => void
  infoOpen: boolean
  onCloseInfo: () => void
  participantsOpen: boolean
  onToggleParticipants: () => void
  onCloseParticipants: () => void
}

export function MeetingPanels({
  navigate,
  chatOpen,
  setChatOpen,
  chatStuck,
  setChatStuck,
  videoSidebarOpen,
  onToggleVideoSidebar,
  infoOpen,
  onCloseInfo,
  participantsOpen,
  onToggleParticipants,
  onCloseParticipants,
}: MeetingPanelsProps) {
  const { stage } = useMeetingStage()

  const closeChat = () => {
    setChatOpen(false)
    setChatStuck(false)
  }

  const toggleChat = () => {
    setChatOpen((open) => {
      if (open && chatStuck) return true
      return !open
    })
    onCloseParticipants()
    onCloseInfo()
  }

  const { roomId, adminId } = useMeetingRoomContext()
  const { chatMessages, systemMessages, sendChat, markRead, votePoll, reactToMessage } = useMeetingChatContext()
  const room = useRoomContext()
  const currentIdentity = room.localParticipant.identity

  return (
    <>
      <div
        className="absolute z-[25] flex items-center gap-2"
        style={{
          top: 'calc(14px + env(safe-area-inset-top, 0px))',
          left: 'calc(14px + env(safe-area-inset-left, 0px))',
        }}
      >
        <ParticipantsToggle isOpen={participantsOpen} onToggle={onToggleParticipants} />
        {stage && <VideoSidebarToggle isOpen={videoSidebarOpen} onToggle={onToggleVideoSidebar} />}
        <RoomAccessBadge />
      </div>

      <ChatToggle isOpen={chatOpen} onToggle={toggleChat} />

      {participantsOpen && !infoOpen && (!chatOpen || chatStuck) && (
        <ParticipantsList adminId={adminId} onClose={onCloseParticipants} />
      )}
      {chatOpen && (
        <ChatPanel
          onClose={closeChat}
          roomId={roomId}
          currentIdentity={currentIdentity}
          chatMessages={chatMessages}
          systemMessages={systemMessages}
          sendChat={sendChat}
          markRead={markRead}
          votePoll={votePoll}
          reactToMessage={reactToMessage}
          stuck={chatStuck}
          onStuckChange={setChatStuck}
        />
      )}
      <RoomInfoPanel open={infoOpen} onOpenChange={(open) => !open && onCloseInfo()} roomId={roomId} />
      <ChatToastNotifier chatOpen={chatOpen} />
      <MeetingControls onNavigate={navigate} />
    </>
  )
}

function meetChromeButtonClass(active: boolean, className?: string) {
  return cn(
    'cursor-pointer backdrop-blur-lg transition-all duration-150',
    active
      ? 'border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
      : 'border border-[var(--meet-border-subtle)] bg-[var(--meet-chrome)] text-[var(--meet-fg-muted)] hover:text-[var(--meet-fg-strong)]',
    className,
  )
}

function VideoSidebarToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className={meetChromeButtonClass(
        isOpen,
        'flex items-center gap-1.5 rounded-xl px-3 py-[7px] text-xs font-semibold',
      )}
      aria-label={isOpen ? 'Close video panel' : 'Show video panel'}
    >
      <Video size={14} />
      <span>Videos</span>
    </button>
  )
}

function ParticipantsToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  const participants = useParticipants()

  return (
    <button
      type="button"
      onClick={onToggle}
      className={meetChromeButtonClass(
        isOpen,
        'flex items-center gap-1.5 rounded-xl px-3 py-[7px] text-xs font-semibold',
      )}
      aria-label={isOpen ? 'Close participants' : 'Show participants'}
    >
      <Users size={14} />
      <span>{participants.length}</span>
    </button>
  )
}

function RoomAccessBadge() {
  const { isPublic } = useMeetingRoomContext()
  const [open, setOpen] = useState(false)
  const Icon = isPublic ? Globe : Lock

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          'flex h-8 w-8 items-center justify-center rounded-lg backdrop-blur-lg transition-all duration-150',
          isPublic
            ? 'border border-[color-mix(in_oklab,var(--accent-600)_35%,transparent)] bg-[color-mix(in_oklab,var(--accent-600)_18%,transparent)] text-accent-400'
            : meetChromeButtonClass(false),
        )}
        aria-label={isPublic ? 'Public room — change access' : 'Private room — change access'}
      >
        <Icon size={14} />
      </button>
      <RoomAccessDialog open={open} onOpenChange={setOpen} />
    </>
  )
}

function ChatToggle({ isOpen, onToggle }: { isOpen: boolean; onToggle: () => void }) {
  const { unreadCount } = useMeetingChatContext()

  return (
    <button
      type="button"
      onClick={onToggle}
      className={meetChromeButtonClass(
        isOpen,
        'absolute z-[25] flex h-[38px] w-[38px] items-center justify-center rounded-xl',
      )}
      style={{
        top: 'calc(14px + env(safe-area-inset-top, 0px))',
        right: 'calc(14px + env(safe-area-inset-right, 0px))',
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
