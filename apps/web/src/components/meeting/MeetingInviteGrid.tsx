import { useIsSpeaking, useParticipants } from '@livekit/components-react'
import { MicOff } from 'lucide-react'
import { useMemo } from 'react'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { getPalette } from '#/lib/participant-palette'
import { shouldShowMicMutedIndicator } from '#/lib/push-to-talk-participant'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { cn } from '@/lib/utils'

type InviteParticipant = ReturnType<typeof useParticipants>[number]

interface ParticipantMeta {
  accesses?: string[]
}

/**
 * The badge that carries a cell's mic or deafen state. It sits on the avatar's corner on a plate in
 * the sheet's own colour, so it reads against a photo as well as against initials.
 */
const STATUS_BADGE_CLASS =
  'absolute -bottom-0.5 -end-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-[var(--meet-sidebar)] text-red-400'

/**
 * The size of the avatar itself. The cell around it is this plus the ring band, because the ring is a
 * border rather than an outline: an outset ring paints beyond the circle without taking any layout
 * space, which makes a ringed avatar read as a larger avatar and makes a cell grow and shrink as
 * somebody starts and stops speaking. Reserving the band on every cell keeps them all one size.
 */
const AVATAR_SIZE_STYLE = {
  width: 'var(--meet-invite-avatar-size)',
  height: 'var(--meet-invite-avatar-size)',
}

function parseMeta(raw: string | undefined): ParticipantMeta {
  try {
    return JSON.parse(raw ?? '{}')
  } catch {
    return {}
  }
}

interface InviteGridCellProps {
  participant: InviteParticipant
  adminId: string
}

function InviteGridCell({ participant, adminId }: InviteGridCellProps) {
  const { isParticipantDeafened, getParticipantDisplayName, getParticipantAvatarUrl } = useMeetingRoomContext()
  const pushToTalkEnabled = useAudioPreferencesStore((state) => state.pushToTalkEnabled)
  const isSpeaking = useIsSpeaking(participant)

  const displayName = getParticipantDisplayName(participant)
  const palette = useMemo(() => getPalette(displayName), [displayName])
  const meta = useMemo(() => parseMeta(participant.metadata), [participant.metadata])

  const isRoomAdmin = participant.identity === adminId
  const isModerator = !isRoomAdmin && (meta.accesses ?? []).includes('moderator')
  const isDeafened = isParticipantDeafened(participant)
  const isMicMuted = shouldShowMicMutedIndicator(participant, pushToTalkEnabled)

  // One avatar has one ring. Speaking is the transient signal and the one that answers who is
  // talking, so while it lasts it replaces the ring the participant's role would otherwise draw. A
  // cell with nothing to say still reserves the band, in transparent, so no cell is larger than
  // another and none changes size when somebody speaks.
  const ringColorClass = isSpeaking
    ? 'border-[var(--meet-speaking-ring)]'
    : isRoomAdmin
      ? 'border-[var(--accent-500)]'
      : isModerator
        ? 'border-emerald-500'
        : 'border-transparent'

  return (
    <li className="flex min-w-0 flex-col items-center gap-1.5">
      <div className="relative">
        <div className={cn('rounded-full border-2', ringColorClass)}>
          <ParticipantAvatar
            avatarUrl={getParticipantAvatarUrl(participant)}
            initials={displayName.charAt(0).toUpperCase()}
            paletteBackground={palette.avatar}
            className="text-sm"
            style={AVATAR_SIZE_STYLE}
          />
        </div>
        {isDeafened ? (
          <span className={STATUS_BADGE_CLASS}>
            <DeafenHeadphonesIcon size={10} off />
          </span>
        ) : isMicMuted ? (
          <span className={STATUS_BADGE_CLASS}>
            <MicOff size={10} />
          </span>
        ) : null}
      </div>
      <span className="w-full overflow-hidden text-ellipsis whitespace-nowrap text-center text-[11px] font-medium text-[var(--meet-fg-strong)]">
        {participant.isLocal ? 'You' : displayName}
      </span>
    </li>
  )
}

interface MeetingInviteGridProps {
  participants: InviteParticipant[]
  adminId: string
}

/**
 * Everyone in the room as a four-column avatar grid, capped and scrolling past the cap so the invite
 * block under it survives a busy room.
 */
export function MeetingInviteGrid({ participants, adminId }: MeetingInviteGridProps) {
  return (
    <ul
      className="meet-scroll grid shrink-0 grid-cols-4 gap-x-2 gap-y-3 overflow-y-auto"
      style={{ maxHeight: 'var(--meet-invite-grid-max-height)' }}
    >
      {participants.map((participant) => (
        <InviteGridCell key={participant.identity} participant={participant} adminId={adminId} />
      ))}
    </ul>
  )
}
