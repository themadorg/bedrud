import { MessageSquare, MonitorOff, MonitorUp, PhoneOff, Video, VideoOff } from 'lucide-react'
import { MeetingMicPill } from '@/components/meeting/MeetingMicPill'
import { cn } from '@/lib/utils'

/** The circular side controls. 48px is the accessibility floor for a thumb. */
const SIDE_BUTTON_CLASS =
  'flex h-12 w-12 shrink-0 items-center justify-center rounded-full border-none transition-colors duration-150'

const SIDE_BUTTON_RESTING = 'bg-[var(--meet-control)] text-[var(--meet-control-fg)]'
const SIDE_BUTTON_ACTIVE = 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
const SIDE_BUTTON_ALERT = 'bg-[var(--meet-btn-alert-bg)] text-[var(--meet-btn-alert-fg)]'

interface MeetingCallControlsRowProps {
  cameraEnabled: boolean
  onToggleCamera: () => void
  screenShareEnabled: boolean
  screenShareAvailable: boolean
  onToggleScreenShare: () => void
  micPushToTalk: boolean
  micOpen: boolean
  micTransmitting: boolean
  micAvailable: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
  chatOpen: boolean
  unreadCount: number
  onToggleChat: () => void
  onLeave: () => void
}

/**
 * The five call controls: the row that sits at the foot of the pill, collapsed or expanded. The
 * side clusters carry equal weight so the mic slot stays centred under the handle however its
 * label changes.
 */
export function MeetingCallControlsRow({
  cameraEnabled,
  onToggleCamera,
  screenShareEnabled,
  screenShareAvailable,
  onToggleScreenShare,
  micPushToTalk,
  micOpen,
  micTransmitting,
  micAvailable,
  onToggleMic,
  onPushToTalkChange,
  chatOpen,
  unreadCount,
  onToggleChat,
  onLeave,
}: MeetingCallControlsRowProps) {
  return (
    <div className="flex w-full items-center gap-2 px-3 pb-3">
      {/*
        `min-w-fit` on both clusters is what keeps the hang-up button on screen. They size from a zero
        basis so the mic slot stays centred, and a zero basis lets a cluster be allotted less than its
        own buttons need — which cropped the end of the row rather than shrinking anything.
      */}
      <div className="flex min-w-fit flex-1 items-center justify-start gap-2">
        <button
          type="button"
          onClick={onToggleCamera}
          aria-label={cameraEnabled ? 'Disable camera' : 'Enable camera'}
          className={cn(SIDE_BUTTON_CLASS, cameraEnabled ? SIDE_BUTTON_RESTING : SIDE_BUTTON_ALERT)}
        >
          {cameraEnabled ? <Video size={18} /> : <VideoOff size={18} />}
        </button>
        <button
          type="button"
          onClick={screenShareAvailable ? onToggleScreenShare : undefined}
          aria-label={screenShareEnabled ? 'Stop sharing' : 'Share screen'}
          className={cn(
            SIDE_BUTTON_CLASS,
            screenShareEnabled ? SIDE_BUTTON_ALERT : SIDE_BUTTON_RESTING,
            !screenShareAvailable && 'cursor-not-allowed opacity-40',
          )}
        >
          {screenShareEnabled ? <MonitorOff size={17} /> : <MonitorUp size={17} />}
        </button>
      </div>

      <MeetingMicPill
        pushToTalk={micPushToTalk}
        micOpen={micOpen}
        transmitting={micTransmitting}
        available={micAvailable}
        onToggleMic={onToggleMic}
        onPushToTalkChange={onPushToTalkChange}
      />

      <div className="flex min-w-fit flex-1 items-center justify-end gap-2">
        <button
          type="button"
          onClick={onToggleChat}
          aria-label={chatOpen ? 'Close chat' : `Open chat${unreadCount > 0 ? ` (${unreadCount} unread)` : ''}`}
          className={cn('relative', SIDE_BUTTON_CLASS, chatOpen ? SIDE_BUTTON_ACTIVE : SIDE_BUTTON_RESTING)}
        >
          <MessageSquare size={17} />
          {unreadCount > 0 && !chatOpen && (
            <span
              className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--meet-btn-leave-bg)] px-1 font-semibold text-[10px] text-[var(--meet-btn-leave-fg)]"
              aria-hidden="true"
            >
              {unreadCount > 99 ? '99+' : unreadCount}
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={onLeave}
          aria-label="Leave meeting"
          // meet-btn-leave: the corner comes from the token, which needs !important against the
          // utilities on the same element.
          className="meet-btn-leave flex h-12 w-14 shrink-0 items-center justify-center border-none bg-[var(--meet-btn-leave-bg)] text-[var(--meet-btn-leave-fg)] transition-colors duration-150 hover:bg-[var(--meet-btn-leave-hover)]"
        >
          <PhoneOff size={18} />
        </button>
      </div>
    </div>
  )
}
