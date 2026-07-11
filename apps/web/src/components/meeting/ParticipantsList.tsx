import { useParticipants } from '@livekit/components-react'
import { Mic, MicOff, Users, Video, VideoOff, X } from 'lucide-react'
import { useMemo } from 'react'
import { createPortal } from 'react-dom'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { getPalette } from '#/lib/participant-palette'
import { isPushToTalkParticipant, shouldShowMicMutedIndicator } from '#/lib/push-to-talk-participant'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { cn } from '@/lib/utils'

import { useFocusTrap } from './useFocusTrap'

/** Above stage WebXDC (body z-15) — same stacking model as unpinned ChatPanel. */
const OVERLAY_Z = 40

interface Props {
  onClose: () => void
  adminId: string
}

interface ParticipantMeta {
  accesses?: string[]
  deafened?: boolean
}

function parseMeta(raw: string | undefined): ParticipantMeta {
  try {
    return JSON.parse(raw ?? '{}')
  } catch {
    return {}
  }
}

export function ParticipantsList({ onClose, adminId }: Props) {
  const participants = useParticipants()
  const trapRef = useFocusTrap({ enabled: true, onClose })

  const panel = (
    <aside
      ref={trapRef}
      role="dialog"
      aria-modal="true"
      aria-label="Participants"
      data-participants-overlay="true"
      style={{ zIndex: OVERLAY_Z }}
      className={cn(
        'z-40 flex flex-col bg-[var(--meet-sidebar)] shadow-2xl backdrop-blur-2xl',
        // Mobile: full-screen on visual viewport (iOS Safari toolbar-safe)
        'fixed left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] w-[var(--app-width,100svw)] max-h-[var(--app-height,100svh)] max-w-[var(--app-width,100svw)]',
        'pt-[env(safe-area-inset-top,0px)] pb-[max(0.5rem,env(safe-area-inset-bottom,0px))]',
        // Desktop: left sidebar — fixed + body portal so it stacks above stage WebXDC (z-15).
        'sm:fixed sm:inset-y-0 sm:start-0 sm:bottom-0 sm:left-0 sm:top-0 sm:h-full sm:max-h-none sm:w-[min(288px,var(--app-width,100svw))] sm:max-w-none sm:border-e sm:border-[var(--meet-border-subtle)] sm:pb-[calc(88px+env(safe-area-inset-bottom,0px))]',
      )}
    >
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-[var(--meet-border-subtle)] px-3 sm:h-[52px] sm:px-4">
        <div className="flex items-center gap-[7px]">
          <Users size={14} className="text-[var(--meet-btn-muted-fg)]" />
          <span className="text-[13px] font-semibold text-[var(--meet-fg-strong)]">Participants</span>
          <span className="rounded-md border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] px-[6px] py-px text-[11px] font-semibold text-[var(--meet-btn-muted-fg)]">
            {participants.length}
          </span>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="flex h-11 w-11 cursor-pointer items-center justify-center rounded-[7px] border-none bg-transparent text-[var(--meet-fg-muted)] transition-[background,color] duration-150 hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)] sm:h-7 sm:w-7"
          aria-label="Close participants"
        >
          <X size={18} className="sm:h-[15px] sm:w-[15px]" />
        </button>
      </div>

      {/* List */}
      <div className="meet-scroll flex-1 overflow-y-auto p-2">
        {participants.map((p) => (
          <ParticipantRow key={p.identity} p={p} adminId={adminId} />
        ))}
      </div>
    </aside>
  )

  // Portal to body so the list sits above body-portaled WebXDC / stage chrome (same as unpinned chat).
  if (typeof document !== 'undefined') {
    return createPortal(panel, document.body)
  }
  return panel
}

interface RowProps {
  p: ReturnType<typeof useParticipants>[number]
  adminId: string
}

function ParticipantRow({ p, adminId }: RowProps): React.ReactElement {
  const { isParticipantDeafened, getParticipantDisplayName, getParticipantAvatarUrl } = useMeetingRoomContext()
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)
  const localPushToTalkEnabled = pushToTalkEnabled
  const displayName = getParticipantDisplayName(p)
  const avatarUrl = getParticipantAvatarUrl(p)
  const initial = displayName.charAt(0).toUpperCase()
  const palette = useMemo(() => getPalette(displayName), [displayName])
  const isDeafened = isParticipantDeafened(p)

  const meta = useMemo(() => parseMeta(p.metadata), [p.metadata])
  const participantAccesses = meta.accesses ?? []
  const isRoomAdmin = p.identity === adminId
  const isMod = !isRoomAdmin && participantAccesses.includes('moderator')
  const isGuest = !isRoomAdmin && !isMod && participantAccesses.includes('guest')

  const row = (
    <div className="group flex cursor-default items-center gap-2.5 rounded-xl px-2 py-[7px] transition-[background] duration-[0.12s] hover:bg-[var(--meet-control)]">
      {/* Avatar */}
      <ParticipantAvatar
        avatarUrl={avatarUrl}
        initials={initial}
        paletteBackground={palette.avatar}
        className="h-8 w-8 shrink-0 text-[13px]"
      />

      {/* Name + badges */}
      <div className="flex-1 min-w-0 flex flex-col gap-[3px]">
        <div className="flex items-center gap-[5px]">
          <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[13px] font-medium text-[var(--meet-fg-strong)]">
            {displayName}
          </span>
          {p.isLocal && <span className="shrink-0 text-[11px] text-[var(--meet-fg-muted)]">you</span>}
        </div>

        {(isRoomAdmin || isMod || isGuest) && (
          <div className="flex gap-1">
            {isRoomAdmin && (
              <span className="rounded border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] px-[5px] py-px text-[10px] font-semibold tracking-wide text-[var(--meet-btn-muted-fg)]">
                Admin
              </span>
            )}
            {isMod && (
              <span className="rounded border border-emerald-500/25 bg-emerald-500/10 px-[5px] py-px text-[10px] font-semibold text-emerald-600 dark:text-emerald-300">
                Mod
              </span>
            )}
            {isGuest && (
              <span className="rounded border border-[var(--meet-border)] bg-[var(--meet-control)] px-[5px] py-px text-[10px] font-medium text-[var(--meet-fg-muted)]">
                Guest
              </span>
            )}
          </div>
        )}
      </div>

      {/* Status icons */}
      <div className="flex items-center gap-1 shrink-0">
        {shouldShowMicMutedIndicator(p, localPushToTalkEnabled) ? (
          <MicOff size={13} className="shrink-0 text-red-400" />
        ) : !isPushToTalkParticipant(p, localPushToTalkEnabled) ? (
          <Mic size={13} className="shrink-0 text-[var(--meet-fg-muted)]" />
        ) : null}
        {isDeafened && <DeafenHeadphonesIcon size={13} off className="text-red-400" />}
        {p.isCameraEnabled ? (
          <Video size={13} className="shrink-0 text-[var(--meet-fg-muted)]" />
        ) : (
          <VideoOff size={13} className="shrink-0 text-[var(--meet-fg-muted)]" />
        )}
      </div>
    </div>
  )

  return row
}
