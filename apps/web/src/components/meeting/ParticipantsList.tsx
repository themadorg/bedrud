import { useParticipants } from '@livekit/components-react'
import { Mic, MicOff, Users, Video, VideoOff, VolumeX, X } from 'lucide-react'
import { useMemo } from 'react'
import { getPalette } from '#/lib/participant-palette'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { ParticipantContextMenu, ParticipantMenuButton } from '@/components/meeting/ParticipantContextMenu'
import { useFocusTrap } from './useFocusTrap'

interface Props {
  onClose: () => void
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

export function ParticipantsList({ onClose }: Props) {
  const participants = useParticipants()
  const { adminId } = useMeetingRoomContext()
  const trapRef = useFocusTrap({ enabled: true, onClose })

  return (
    <aside
      ref={trapRef}
      className="absolute end-0 top-0 bottom-0 z-30 flex flex-col bg-[#0a0a16]/94 backdrop-blur-2xl border-s border-white/[0.07] pt-[env(safe-area-inset-top)] pb-[calc(88px+env(safe-area-inset-bottom))]"
      style={{ width: 'min(288px, 100vw)' }}
    >
      {/* Header */}
      <div className="h-[52px] shrink-0 flex items-center justify-between px-4 border-b border-white/[0.06]">
        <div className="flex items-center gap-[7px]">
          <Users size={14} className="text-[color-mix(in_oklab,var(--accent-400)_70%,transparent)]" />
          <span className="text-white/80 text-[13px] font-semibold">Participants</span>
          <span
            className="rounded-md px-[6px] py-px text-[11px] font-semibold"
            style={{
              background: 'color-mix(in oklab, var(--primary) 18%, transparent)',
              border: '1px solid color-mix(in oklab, var(--primary) 25%, transparent)',
              color: 'color-mix(in oklab, var(--accent-400) 80%, transparent)',
            }}
          >
            {participants.length}
          </span>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="w-7 h-7 rounded-[7px] bg-transparent border-none flex items-center justify-center text-white/50 cursor-pointer transition-[background,color] duration-150"
          aria-label="Close participants"
        >
          <X size={15} />
        </button>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto p-2">
        {participants.map((p) => (
          <ParticipantRow key={p.identity} p={p} adminId={adminId} />
        ))}
      </div>
    </aside>
  )
}

interface RowProps {
  p: ReturnType<typeof useParticipants>[number]
  adminId: string
}

function ParticipantRow({ p, adminId }: RowProps): React.ReactElement {
  const displayName = p.name ?? p.identity
  const initial = displayName.charAt(0).toUpperCase()
  const palette = useMemo(() => getPalette(displayName), [displayName])

  const meta = useMemo(() => parseMeta(p.metadata), [p.metadata])
  const participantAccesses = meta.accesses ?? []
  const isRoomAdmin = p.identity === adminId
  const isMod = !isRoomAdmin && participantAccesses.includes('moderator')
  const isGuest = !isRoomAdmin && !isMod && participantAccesses.includes('guest')

  const row = (
    // biome-ignore lint/a11y/noStaticElementInteractions: hover highlight is visual only, not interactive
    <div
      className="group flex items-center gap-2.5 px-2 py-[7px] rounded-xl transition-[background] duration-[0.12s] cursor-default"
      onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(255,255,255,0.05)')}
      onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
    >
      {/* Avatar */}
      <div
        className="w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-[13px] font-bold text-white"
        style={{ background: palette.avatar }}
      >
        {initial}
      </div>

      {/* Name + badges */}
      <div className="flex-1 min-w-0 flex flex-col gap-[3px]">
        <div className="flex items-center gap-[5px]">
          <span className="text-white/[0.82] text-[13px] font-medium overflow-hidden text-ellipsis whitespace-nowrap">
            {displayName}
          </span>
          {p.isLocal && <span className="text-white/50 text-[11px] shrink-0">you</span>}
        </div>

        {(isRoomAdmin || isMod || isGuest) && (
          <div className="flex gap-1">
            {isRoomAdmin && (
              <span
                className="text-[10px] font-semibold tracking-wide rounded px-[5px] py-px"
                style={{
                  color: 'var(--accent-400)',
                  background: 'color-mix(in oklab, var(--primary) 20%, transparent)',
                  border: '1px solid color-mix(in oklab, var(--primary) 30%, transparent)',
                }}
              >
                Admin
              </span>
            )}
            {isMod && (
              <span className="text-[10px] font-semibold text-emerald-300 bg-emerald-500/15 border border-emerald-500/25 rounded px-[5px] py-px">
                Mod
              </span>
            )}
            {isGuest && (
              <span className="text-[10px] font-medium text-white/50 bg-white/[0.05] border border-white/10 rounded px-[5px] py-px">
                Guest
              </span>
            )}
          </div>
        )}
      </div>

      {/* Status icons */}
      <div className="flex items-center gap-1 shrink-0">
        {meta.deafened && <VolumeX size={13} className="shrink-0 text-red-400" />}
        {p.isMicrophoneEnabled ? (
          <Mic size={13} className="shrink-0 text-white/50" />
        ) : (
          <MicOff size={13} className="shrink-0 text-red-400" />
        )}
        {p.isCameraEnabled ? (
          <Video size={13} className="shrink-0 text-white/50" />
        ) : (
          <VideoOff size={13} className="shrink-0 text-white/50" />
        )}

        <div className="opacity-0 group-hover:opacity-100 transition-opacity duration-150">
          <ParticipantMenuButton participant={p} />
        </div>
      </div>
    </div>
  )

  return <ParticipantContextMenu participant={p}>{row}</ParticipantContextMenu>
}
