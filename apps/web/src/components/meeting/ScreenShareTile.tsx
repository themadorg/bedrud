import type { TrackReference, TrackReferenceOrPlaceholder } from '@livekit/components-react'
import { useParticipantInfo, VideoTrack } from '@livekit/components-react'
import { Monitor } from 'lucide-react'

interface ScreenShareTileProps {
  trackRef: TrackReferenceOrPlaceholder
}

export function ScreenShareTile({ trackRef }: ScreenShareTileProps) {
  const { name, identity } = useParticipantInfo({ participant: trackRef.participant })
  const displayName = name ?? identity ?? '?'

  // Placeholders have no publication yet — nothing to render
  if (!trackRef.publication) return null

  return (
    <div
      className="relative w-full h-full bg-[#030308] rounded-xl overflow-hidden"
      style={{ border: '1px solid color-mix(in oklab, var(--primary) 35%, transparent)' }}
    >
      <VideoTrack trackRef={trackRef as TrackReference} className="absolute inset-0 w-full h-full object-contain" />
      <div className="absolute bottom-2.5 left-2.5 flex items-center gap-1.5 bg-black/65 backdrop-blur-sm rounded-[7px] px-2.5 py-1">
        <Monitor size={12} className="shrink-0 text-teal-400" />
        <span className="text-white text-xs font-medium">{displayName} is presenting</span>
      </div>
    </div>
  )
}
