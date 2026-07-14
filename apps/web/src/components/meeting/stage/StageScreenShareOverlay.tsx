import type { TrackReference } from '@livekit/components-react'
import { useLocalParticipant, useTracks, VideoTrack } from '@livekit/components-react'
import { Track } from 'livekit-client'
import { Maximize2, Minimize2, Monitor, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { MeetingExpandLeftRail } from '@/components/meeting/MeetingExpandLeftRail'
import { meetStageShellClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import {
  MEETING_CHROME_STATE,
  type MeetingChromePanel,
  type MeetingChromeStateDetail,
  requestCloseElevatedChrome,
} from '@/components/meeting/meetingChromeEvents'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useMeetingStage } from './MeetingStageContext'
import { resolveScreenShareTrack } from './resolveScreenShareTrack'
import { stageOwnerLabel } from './stageWire'

const EXPANDED_Z = 200

function ScreenSharePanel({
  displayName,
  trackRef,
  expanded,
  onToggleExpand,
  showStop,
  onStop,
}: {
  displayName: string
  trackRef: TrackReference
  expanded: boolean
  onToggleExpand: () => void
  showStop: boolean
  onStop: () => void
}) {
  return (
    <div
      data-screenshare-overlay="true"
      className="meet-dialog flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] shadow-2xl backdrop-blur-md"
    >
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-border bg-background px-2 py-1.5 sm:px-3 sm:py-2">
        <div className="flex min-w-0 items-center gap-2">
          <Monitor size={16} className="shrink-0 text-[var(--meet-accent)]" />
          <div className="min-w-0 text-foreground">
            <p className="truncate text-sm font-medium">Screen share</p>
            <p className="truncate text-[11px] text-muted-foreground">{displayName} is presenting</p>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-0.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            aria-label={expanded ? 'Exit fullscreen' : 'Expand to fullscreen'}
            aria-pressed={expanded}
            onClick={onToggleExpand}
          >
            {expanded ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
          </Button>
          {showStop ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label="Stop screen share"
              onClick={onStop}
            >
              <X className="h-4 w-4" />
            </Button>
          ) : null}
        </div>
      </div>

      <div className="relative min-h-0 flex-1 bg-black">
        <VideoTrack trackRef={trackRef} className="absolute inset-0 h-full w-full object-contain" />
      </div>
    </div>
  )
}

export function StageScreenShareOverlay() {
  const layout = useMeetingUILayout()
  const { stage, isOwner, clearStage } = useMeetingStage()
  const { localParticipant } = useLocalParticipant()
  const screenShareTracks = useTracks([Track.Source.ScreenShare], { onlySubscribed: true })
  const [expanded, setExpanded] = useState(false)
  const [activePanel, setActivePanel] = useState<MeetingChromePanel>(null)
  const chromeDetail = { source: 'screenshare-expand' as const }

  const stageOwnerIdentity = stage?.kind === 'screenshare' ? stage.ownerIdentity : null
  const trackRef = useMemo(
    () => resolveScreenShareTrack(stageOwnerIdentity, localParticipant, screenShareTracks),
    [stageOwnerIdentity, localParticipant, screenShareTracks],
  )

  useEffect(() => {
    if (!expanded) {
      setActivePanel(null)
      return
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const t = e.target
      if (
        t instanceof Element &&
        (t.closest('[data-elevated-chat="true"]') ||
          t.closest('[data-elevated-settings="true"]') ||
          t.closest('[data-elevated-room-info="true"]'))
      ) {
        return
      }
      e.preventDefault()
      setExpanded(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [expanded])

  useEffect(() => {
    if (!expanded) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prev
    }
  }, [expanded])

  useEffect(() => {
    if (!expanded) return
    const onState = (e: Event) => {
      const detail = (e as CustomEvent<MeetingChromeStateDetail>).detail
      setActivePanel(detail?.panel ?? null)
    }
    window.addEventListener(MEETING_CHROME_STATE, onState)
    return () => window.removeEventListener(MEETING_CHROME_STATE, onState)
  }, [expanded])

  const wasExpandedRef = useRef(false)
  useEffect(() => {
    if (expanded) {
      wasExpandedRef.current = true
      return
    }
    if (wasExpandedRef.current) {
      wasExpandedRef.current = false
      requestCloseElevatedChrome()
      setActivePanel(null)
    }
  }, [expanded])

  if (stage?.kind !== 'screenshare' && !trackRef) return null

  if (!trackRef) {
    return (
      <div className={cn(meetStageShellClass(layout, 'p-3 max-sm:p-2'))}>
        <div className="meet-dialog flex min-h-0 flex-1 flex-col items-center justify-center overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] p-6 text-center shadow-2xl backdrop-blur-md">
          <Monitor size={28} className="mb-3 text-[var(--meet-accent)]" />
          <p className="text-sm font-medium text-[var(--meet-fg-strong)]">
            Waiting for {stage ? stageOwnerLabel(stage) : 'presenter'}&apos;s screen…
          </p>
          <p className="mt-1 text-[11px] text-[var(--meet-fg-muted)]">The presentation should appear shortly.</p>
        </div>
      </div>
    )
  }

  const displayName =
    trackRef.participant.name || trackRef.participant.identity || (stage ? stageOwnerLabel(stage) : 'Presenter')

  const stopShare = () => {
    setExpanded(false)
    clearStage()
  }

  const ownerCanStop = isOwner && stage?.kind === 'screenshare'
  const panel = (
    <ScreenSharePanel
      displayName={displayName}
      trackRef={trackRef}
      expanded={expanded}
      onToggleExpand={() => setExpanded((v) => !v)}
      showStop={ownerCanStop}
      onStop={stopShare}
    />
  )

  const expandedSurface =
    expanded &&
    typeof document !== 'undefined' &&
    createPortal(
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Screen share fullscreen"
        data-screenshare-overlay="true"
        className="meet-dialog fixed flex overflow-hidden border-0 bg-background text-foreground shadow-2xl"
        style={{
          zIndex: EXPANDED_Z,
          top: 'var(--app-offset-top, 0px)',
          left: 'var(--app-offset-left, 0px)',
          width: 'var(--app-width, 100vw)',
          height: 'var(--app-height, 100dvh)',
        }}
      >
        <MeetingExpandLeftRail
          activePanel={activePanel}
          chromeDetail={chromeDetail}
          onLeave={ownerCanStop ? stopShare : undefined}
          leaveLabel="Stop screen share"
        />
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">{panel}</div>
      </div>,
      document.body,
    )

  return (
    <>
      <div className={cn(meetStageShellClass(layout, 'p-3 max-sm:p-2'))}>
        <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">{!expanded ? panel : null}</div>
      </div>
      {expandedSurface}
    </>
  )
}
