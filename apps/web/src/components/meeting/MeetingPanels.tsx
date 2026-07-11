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
  /** Remount elevated ChatPanel when opening from expanded WebXDC. */
  chatSurfaceKey?: number
  /** Reset chat dock to right when opened from meeting chrome (not WebXDC rail). */
  onChatOpenFromToggle?: () => void
  chatStuck: boolean
  setChatStuck: (stuck: boolean) => void
  chatSide?: 'left' | 'right'
  chatElevated?: boolean
  videoSidebarOpen: boolean
  onToggleVideoSidebar: () => void
  infoOpen: boolean
  infoElevated?: boolean
  onCloseInfo: () => void
  onToggleInfo: () => void
  participantsOpen: boolean
  onToggleParticipants: () => void
  onCloseParticipants: () => void
}

export function MeetingPanels({
  navigate,
  chatOpen,
  setChatOpen,
  chatSurfaceKey = 0,
  onChatOpenFromToggle,
  chatStuck,
  setChatStuck,
  chatSide = 'right',
  chatElevated = false,
  videoSidebarOpen,
  onToggleVideoSidebar,
  infoOpen,
  infoElevated = false,
  onCloseInfo,
  onToggleInfo,
  participantsOpen,
  onToggleParticipants,
  onCloseParticipants,
}: MeetingPanelsProps) {
  const { stage } = useMeetingStage()
  const [accessDialogOpen, setAccessDialogOpen] = useState(false)

  const closeChat = () => {
    setChatOpen(false)
    setChatStuck(false)
  }

  const toggleChat = () => {
    setChatOpen((open) => {
      if (open && chatStuck) return true
      const next = !open
      if (next) onChatOpenFromToggle?.()
      return next
    })
    onCloseParticipants()
    onCloseInfo()
  }

  const { roomId, adminId, isPublic } = useMeetingRoomContext()
  const { chatMessages, systemMessages, sendChat, markRead, votePoll, reactToMessage } = useMeetingChatContext()
  const room = useRoomContext()
  const currentIdentity = room.localParticipant.identity
  // Full-screen panels on mobile — hide floating chrome while either is open.
  const mobileOverlayOpen = chatOpen || participantsOpen

  return (
    <>
      {/* Desktop left chrome (participants stay left on desktop; mobile uses top-right icons). */}
      <div
        className="absolute z-[25] hidden items-center gap-2 sm:flex"
        style={{
          top: 'calc(14px + env(safe-area-inset-top, 0px))',
          left: 'calc(14px + env(safe-area-inset-left, 0px))',
        }}
      >
        <ParticipantsToggle isOpen={participantsOpen} onToggle={onToggleParticipants} variant="desktop" />
        {stage && <VideoSidebarToggle isOpen={videoSidebarOpen} onToggle={onToggleVideoSidebar} />}
        <RoomAccessBadge onOpen={() => setAccessDialogOpen(true)} />
      </div>

      {/* Mobile top-right: participants + chat — vertically centered in the 56px header band. */}
      <div
        className={cn('absolute z-[25] flex h-9 items-center gap-2 sm:hidden', mobileOverlayOpen && 'hidden')}
        style={{
          // (56px band − 38px buttons) / 2 = 9px below safe-area
          top: 'calc(env(safe-area-inset-top, 0px) + 9px)',
          right: 'calc(14px + env(safe-area-inset-right, 0px))',
        }}
      >
        <ParticipantsToggle isOpen={participantsOpen} onToggle={onToggleParticipants} variant="icon" />
        <ChatToggle isOpen={chatOpen} onToggle={toggleChat} absolute={false} />
      </div>

      {/* Desktop chat — top-right */}
      <ChatToggle isOpen={chatOpen} onToggle={toggleChat} className="hidden sm:flex" />

      <RoomAccessDialog open={accessDialogOpen} onOpenChange={setAccessDialogOpen} />

      {participantsOpen && !infoOpen && (!chatOpen || chatStuck) && (
        <ParticipantsList adminId={adminId} onClose={onCloseParticipants} />
      )}
      {chatOpen && (
        <ChatPanel
          key={chatElevated ? `chat-elevated-${chatSurfaceKey}` : 'chat-default'}
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
          side={chatSide}
          elevated={chatElevated}
        />
      )}
      <RoomInfoPanel
        open={infoOpen}
        onOpenChange={(open) => !open && onCloseInfo()}
        roomId={roomId}
        elevated={infoElevated}
      />
      <ChatToastNotifier chatOpen={chatOpen} />
      <MeetingControls
        onNavigate={navigate}
        hideOnMobile={mobileOverlayOpen}
        moreExtras={{
          onRoomAccess: () => setAccessDialogOpen(true),
          isPublic,
          roomId,
          // Desktop header still uses RoomInfoPanel dialog; mobile uses More sub-page.
          onRoomInfo: onToggleInfo,
          onToggleVideoSidebar,
          showVideoSidebarToggle: Boolean(stage),
          videoSidebarOpen,
        }}
      />
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

function ParticipantsToggle({
  isOpen,
  onToggle,
  variant = 'desktop',
}: {
  isOpen: boolean
  onToggle: () => void
  /** `icon` matches chat (38×38); `desktop` is the wider pill with count text. */
  variant?: 'desktop' | 'icon'
}) {
  const participants = useParticipants()
  const count = participants.length

  if (variant === 'icon') {
    return (
      <button
        type="button"
        onClick={onToggle}
        className={meetChromeButtonClass(isOpen, 'flex h-[38px] w-[38px] items-center justify-center rounded-xl')}
        aria-label={isOpen ? 'Close participants' : `Show participants (${count})`}
      >
        <Users size={16} />
      </button>
    )
  }

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
      <span>{count}</span>
    </button>
  )
}

function RoomAccessBadge({ onOpen }: { onOpen: () => void }) {
  const { isPublic } = useMeetingRoomContext()
  const Icon = isPublic ? Globe : Lock

  return (
    <button
      type="button"
      onClick={onOpen}
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
  )
}

function ChatToggle({
  isOpen,
  onToggle,
  className,
  absolute = true,
}: {
  isOpen: boolean
  onToggle: () => void
  className?: string
  /** When false, parent positions the button (e.g. mobile top-right cluster). */
  absolute?: boolean
}) {
  const { unreadCount } = useMeetingChatContext()

  return (
    <button
      type="button"
      onClick={onToggle}
      className={meetChromeButtonClass(
        isOpen,
        cn(
          'relative flex h-[38px] w-[38px] items-center justify-center rounded-xl',
          absolute && 'absolute z-[25]',
          className,
        ),
      )}
      style={
        absolute
          ? {
              top: 'calc(14px + env(safe-area-inset-top, 0px))',
              right: 'calc(14px + env(safe-area-inset-right, 0px))',
            }
          : undefined
      }
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
