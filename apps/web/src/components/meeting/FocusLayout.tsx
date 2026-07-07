import { useIsSpeaking, useParticipants } from '@livekit/components-react'
import type { Participant } from 'livekit-client'
import { MeetingViewportGrid } from '@/components/meeting/MeetingViewportPan'
import { ParticipantTile } from './ParticipantTile'

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
        border: isSpeaking ? '1.5px solid var(--meet-strip-border-active)' : '1.5px solid var(--meet-strip-border)',
        boxShadow: isSpeaking ? '0 0 14px var(--meet-strip-glow)' : 'none',
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
  const participants = useParticipants()

  const pinnedParticipants = participants.filter((p) => pinnedIdentities.has(p.identity))
  const stripParticipants = participants.filter((p) => !pinnedIdentities.has(p.identity))

  const mainCount = pinnedParticipants.length
  const gridCols = Math.min(Math.max(mainCount, 1), 3)
  const hasStrip = stripParticipants.length > 0

  // Strip outer height including its own padding
  const STRIP_OUTER = STRIP_H + 18

  return (
    <MeetingViewportGrid>
      <div className="flex h-full w-full min-h-0 flex-col">
      {/* ── Main focus area ─────────────────────────────────────── */}
      <div
        className="flex-1 grid gap-[5px] p-[5px_5px_0] min-h-0"
        style={{
          gridTemplateColumns: `repeat(${gridCols}, 1fr)`,
          gridAutoRows: '1fr',
        }}
      >
        {pinnedParticipants.map((p, i) => (
          <ParticipantTile
            key={p.identity}
            participant={p}
            totalCount={mainCount}
            index={i}
            isPinned
            onTogglePin={() => onTogglePin(p.identity)}
          />
        ))}
      </div>

      {/* ── Bottom filmstrip ─────────────────────────────────────── */}
      {hasStrip && (
        <div
          className="relative mt-[5px] shrink-0 border-t border-[var(--meet-border-subtle)] bg-[var(--meet-filmstrip-bg)] backdrop-blur-lg"
          style={{ height: STRIP_OUTER }}
        >
          {/* Scrollable row */}
          <div className="meet-scroll-none h-full flex items-center gap-1.5 overflow-x-auto px-2.5 [scrollbar-width:none]">
            {stripParticipants.map((p, i) => (
              <StripTile key={p.identity} participant={p} index={i} onTogglePin={() => onTogglePin(p.identity)} />
            ))}

            {/* Right padding sentinel so last tile isn't occluded by fade */}
            <div className="w-8 shrink-0" />
          </div>

          {/* Right-edge fade — hints at horizontal scroll */}
          <div className="meet-filmstrip-fade pointer-events-none absolute right-0 top-0 h-full w-16" />

          <div className="pointer-events-none absolute right-3.5 top-2 rounded-md border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] px-[7px] py-0.5 text-[11px] font-semibold text-[var(--meet-btn-muted-fg)]">
            {stripParticipants.length}
          </div>
        </div>
      )}
      </div>
    </MeetingViewportGrid>
  )
}
