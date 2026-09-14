import { Mic, MicOff } from 'lucide-react'
import { cn } from '@/lib/utils'

/** Every label the pill can show, longest first so the invisible stack is easy to read. */
const MIC_PILL_LABELS = ['Push to Talk', 'Talking…', 'Muted', 'Speak'] as const

/** Android's Dimens.meetingMicPillMaxWidth: the sides never squeeze at this width. */
const MIC_PILL_MAX_WIDTH_CLASS = 'max-w-[136px]'

interface MeetingMicPillProps {
  pushToTalk: boolean
  micOpen: boolean
  transmitting: boolean
  available: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
}

/**
 * The centre slot, one pill for both input modes so the bar's rhythm never changes. Voice activity
 * renders filled and toggles on tap; push to talk renders as an outline until held, when it fills.
 */
export function MeetingMicPill({
  pushToTalk,
  micOpen,
  transmitting,
  available,
  onToggleMic,
  onPushToTalkChange,
}: MeetingMicPillProps) {
  const label = transmitting ? 'Talking…' : pushToTalk ? 'Push to Talk' : micOpen ? 'Speak' : 'Muted'

  const container = transmitting
    ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
    : pushToTalk
      ? 'bg-transparent border border-[var(--meet-border)] text-[var(--meet-fg-muted)]'
      : micOpen
        ? 'bg-[var(--meet-control)] text-[var(--meet-control-fg)]'
        : 'bg-[var(--meet-btn-alert-bg)] text-[var(--meet-btn-alert-fg)]'

  // Push to talk is held, voice activity is tapped, so the two modes bind different handlers to
  // the same element rather than rendering two pills.
  const gestureProps = pushToTalk
    ? {
        onPointerDown: (event: React.PointerEvent<HTMLButtonElement>) => {
          if (!available) return
          event.currentTarget.setPointerCapture(event.pointerId)
          onPushToTalkChange(true)
        },
        onPointerUp: (event: React.PointerEvent<HTMLButtonElement>) => {
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId)
          }
          onPushToTalkChange(false)
        },
        onPointerLeave: () => onPushToTalkChange(false),
        onPointerCancel: () => onPushToTalkChange(false),
      }
    : { onClick: onToggleMic }

  return (
    <button
      type="button"
      aria-label={pushToTalk ? 'Push to talk' : micOpen ? 'Mute microphone' : 'Unmute microphone'}
      className={cn(
        // The pill is the row's release valve. The five controls plus this pill's reserved label are
        // wider than a 375px phone, and something has to give: a truncated word is recoverable, a
        // hang-up button cropped off the end of the row is not. So this shrinks and the rest does not.
        'flex h-12 min-w-0 items-center justify-center gap-2 overflow-hidden rounded-full px-3 transition-[background,color,transform] duration-150 active:scale-[0.96]',
        MIC_PILL_MAX_WIDTH_CLASS,
        container,
        !available && 'cursor-not-allowed opacity-40',
      )}
      {...gestureProps}
    >
      {micOpen || transmitting ? <Mic size={18} className="shrink-0" /> : <MicOff size={18} className="shrink-0" />}
      {/* Every label occupies the same cell so the pill is as wide as its longest one, always. */}
      <span className="grid">
        {MIC_PILL_LABELS.map((candidate) => (
          <span
            key={candidate}
            aria-hidden={candidate !== label}
            className={cn(
              'col-start-1 row-start-1 whitespace-nowrap text-[13px] font-medium',
              candidate !== label && 'invisible',
            )}
          >
            {candidate}
          </span>
        ))}
      </span>
    </button>
  )
}
