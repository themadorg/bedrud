import { useParticipants } from '@livekit/components-react'

import { cn } from '#/lib/utils'
import { ParticipantTile } from './ParticipantTile'

interface ParticipantGridProps {
  pinnedIdentities: Set<string>
  onTogglePin: (identity: string) => void
}

function gridCols(count: number): string {
  if (count === 1) return 'grid-cols-1'
  if (count <= 4) return 'grid-cols-1 sm:grid-cols-2'
  if (count <= 9) return 'grid-cols-2 sm:grid-cols-3'
  return 'grid-cols-2 sm:grid-cols-3 lg:grid-cols-4'
}

export function ParticipantGrid({ pinnedIdentities, onTogglePin }: ParticipantGridProps) {
  const participants = useParticipants()

  // Note: useParticipants() from LiveKit always includes the local participant once connected.
  // Therefore participants.length is effectively always >= 1 in normal usage.
  // We do not render a special "empty" state here. If length ever becomes 0 (e.g. very early mount
  // or certain error states), the grid simply renders nothing — this is intentional future-proofing
  // rather than dead code that needs removal.

  return (
    <div
      id="meet-grid"
      className="absolute inset-0 z-0 pt-[calc(56px+env(safe-area-inset-top))] pb-[calc(88px+env(safe-area-inset-bottom))]"
    >
      <div
        className={cn(
          'grid h-full w-full grid-auto-rows-[1fr]',
          gridCols(participants.length),
          participants.length === 1 ? 'gap-0 p-0' : 'gap-[3px] p-[3px]',
        )}
      >
        {participants.map((p, i) => (
          <ParticipantTile
            key={p.identity}
            participant={p}
            totalCount={participants.length}
            index={i}
            isPinned={pinnedIdentities.has(p.identity)}
            onTogglePin={() => onTogglePin(p.identity)}
          />
        ))}
      </div>
    </div>
  )
}
