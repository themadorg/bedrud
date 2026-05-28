import { useIsSpeaking, useParticipants, useTracks } from '@livekit/components-react'
import { type Participant, Track } from 'livekit-client'

import { ParticipantTile } from './ParticipantTile'
import { ScreenShareTile } from './ScreenShareTile'

interface FocusLayoutProps {
  pinnedIdentities: Set<string>
  onTogglePin: (identity: string) => void
}

// Strip tile dimensions (16:9)
const STRIP_W = 168
const STRIP_H = 94

// Separate component so useIsSpeaking can be called per participant
function StripTile({
  participant,
  index,
  onTogglePin,
}: {
  participant: Participant
  index: number
  onTogglePin: () => void
}) {
  const isSpeaking = useIsSpeaking(participant)

  return (
    <div
      className="shrink-0 overflow-hidden rounded-lg cursor-pointer transition-[border-color,box-shadow] duration-200"
      style={{
        width: STRIP_W,
        height: STRIP_H,
        border: isSpeaking
          ? '1.5px solid color-mix(in oklab, var(--primary) 75%, transparent)'
          : '1.5px solid rgba(255,255,255,0.07)',
        boxShadow: isSpeaking ? '0 0 14px color-mix(in oklab, var(--primary) 30%, transparent)' : 'none',
      }}
    >
      <ParticipantTile
        participant={participant}
        totalCount={6}
        index={index}
        isPinned={false}
        onTogglePin={onTogglePin}
      />
    </div>
  )
}

export function FocusLayout({ pinnedIdentities, onTogglePin }: FocusLayoutProps) {
  const screenShareTracks = useTracks([Track.Source.ScreenShare])
  const participants = useParticipants()

  const pinnedParticipants = participants.filter((p) => pinnedIdentities.has(p.identity))
  const stripParticipants = participants.filter((p) => !pinnedIdentities.has(p.identity))

  const mainCount = screenShareTracks.length + pinnedParticipants.length
  const gridCols = Math.min(mainCount, 3)
  const hasStrip = stripParticipants.length > 0

  // Strip outer height including its own padding
  const STRIP_OUTER = STRIP_H + 18

  return (
    <div
      id="meet-grid"
      className="absolute inset-0 z-0 flex flex-col pt-[calc(56px+env(safe-area-inset-top))] pb-[calc(88px+env(safe-area-inset-bottom))]"
    >
      {/* ── Main focus area ─────────────────────────────────────── */}
      <div
        className="flex-1 grid gap-[5px] p-[5px_5px_0] min-h-0"
        style={{
          gridTemplateColumns: `repeat(${gridCols}, 1fr)`,
          gridAutoRows: '1fr',
        }}
      >
        {screenShareTracks.map((track) => (
          <ScreenShareTile key={`${track.participant.identity}-screen`} trackRef={track} />
        ))}
        {pinnedParticipants.map((p, i) => (
          <ParticipantTile
            key={p.identity}
            participant={p}
            totalCount={mainCount}
            index={screenShareTracks.length + i}
            isPinned
            onTogglePin={() => onTogglePin(p.identity)}
          />
        ))}
      </div>

      {/* ── Bottom filmstrip ─────────────────────────────────────── */}
      {hasStrip && (
        <div
          className="shrink-0 relative bg-[#080812]/70 border-t border-white/[0.06] backdrop-blur-lg mt-[5px]"
          style={{ height: STRIP_OUTER }}
        >
          {/* Scrollable row */}
          <div className="h-full flex items-center gap-1.5 px-2.5 overflow-x-auto [scrollbar-width:none]">
            {stripParticipants.map((p, i) => (
              <StripTile key={p.identity} participant={p} index={i} onTogglePin={() => onTogglePin(p.identity)} />
            ))}

            {/* Right padding sentinel so last tile isn't occluded by fade */}
            <div className="w-8 shrink-0" />
          </div>

          {/* Right-edge fade — hints at horizontal scroll */}
          <div
            className="absolute right-0 top-0 w-16 h-full pointer-events-none"
            style={{
              background: 'linear-gradient(to right, transparent, rgba(8,8,18,0.85))',
            }}
          />

          {/* Participant count badge */}
          <div
            className="absolute top-2 right-3.5 rounded-md px-[7px] py-0.5 text-[11px] font-semibold pointer-events-none"
            style={{
              background: 'color-mix(in oklab, var(--primary) 18%, transparent)',
              border: '1px solid color-mix(in oklab, var(--primary) 30%, transparent)',
              color: 'color-mix(in oklab, var(--accent-400) 80%, transparent)',
            }}
          >
            {stripParticipants.length}
          </div>
        </div>
      )}
    </div>
  )
}
