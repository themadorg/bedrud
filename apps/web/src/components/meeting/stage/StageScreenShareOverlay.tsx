import type { TrackReference } from '@livekit/components-react'
import { useTracks, VideoTrack } from '@livekit/components-react'
import { Track } from 'livekit-client'
import { Monitor, X } from 'lucide-react'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { cn } from '@/lib/utils'
import { useMeetingStage } from './MeetingStageContext'
import { stageOwnerLabel } from './stageWire'

export function StageScreenShareOverlay() {
  const layout = useMeetingUILayout()
  const { stage, isOwner, clearStage } = useMeetingStage()
  const screenShareTracks = useTracks([Track.Source.ScreenShare], { onlySubscribed: true })
  const ownerIdentity = stage?.kind === 'screenshare' ? stage.ownerIdentity : null
  const trackRef = ownerIdentity
    ? screenShareTracks.find((t) => t.participant.identity === ownerIdentity && t.publication)
    : undefined
  if (stage?.kind !== 'screenshare') return null

  if (!trackRef?.publication) {
    return (
      <div
        className={cn(
          'absolute top-[calc(56px+env(safe-area-inset-top))] left-0 bottom-[calc(88px+env(safe-area-inset-bottom))] z-[5] flex flex-col p-3 transition-[right] duration-200',
          meetRightInsetClass(layout),
        )}
      >
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center overflow-hidden rounded-xl border border-white/[0.08] bg-[#030308]/95 p-6 text-center shadow-2xl backdrop-blur-md">
          <Monitor size={28} className="mb-3 text-teal-400" />
          <p className="text-sm font-medium text-white">Waiting for {stageOwnerLabel(stage)}&apos;s screen…</p>
          <p className="mt-1 text-[11px] text-white/45">The presentation should appear shortly.</p>
        </div>
      </div>
    )
  }

  const displayName = trackRef.participant.name || trackRef.participant.identity || stageOwnerLabel(stage)

  return (
    <div
      className={cn(
        'absolute top-[calc(56px+env(safe-area-inset-top))] left-0 bottom-[calc(88px+env(safe-area-inset-bottom))] z-[5] flex flex-col p-3 transition-[right] duration-200',
        meetRightInsetClass(layout),
      )}
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-white/[0.08] bg-[#030308]/95 shadow-2xl backdrop-blur-md">
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-white/[0.06] px-3 py-2.5">
          <div className="flex min-w-0 items-center gap-2">
            <Monitor size={16} className="shrink-0 text-teal-400" />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-white">Screen share</p>
              <p className="truncate text-[11px] text-white/45">{displayName} is presenting</p>
            </div>
          </div>

          {isOwner && (
            <button
              type="button"
              onClick={() => clearStage()}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border-none bg-white/[0.07] text-white/70 transition-colors hover:bg-white/[0.12] hover:text-white"
              aria-label="Stop screen share"
            >
              <X size={16} />
            </button>
          )}
        </div>

        <div className="relative min-h-0 flex-1 bg-black">
          <VideoTrack trackRef={trackRef as TrackReference} className="absolute inset-0 h-full w-full object-contain" />
        </div>
      </div>
    </div>
  )
}
