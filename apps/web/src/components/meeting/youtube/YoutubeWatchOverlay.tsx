import { Film, Maximize2, Minimize2, X } from 'lucide-react'
import { type ReactNode, useCallback, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { MeetingExpandLeftRail } from '@/components/meeting/MeetingExpandLeftRail'
import { meetStageShellClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { useMeetingExpandChrome } from '@/components/meeting/useMeetingExpandChrome'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { isYoutubePlayerReady } from './loadYoutubeIframeApi'
import { useYoutubePlayer } from './useYoutubePlayer'
import { useYoutubeWatch } from './youtube-watch-context'
import type { YoutubeSession } from './youtubeWire'

const SYNC_DRIFT_SECONDS = 1.5
const HOST_HEARTBEAT_MS = 2500

function YoutubeWatchPanel({
  hostName,
  expanded,
  onToggleExpand,
  showStop,
  onStop,
  children,
}: {
  hostName: string
  expanded: boolean
  onToggleExpand: () => void
  showStop: boolean
  onStop: () => void
  children: ReactNode
}) {
  return (
    <div className="meet-dialog flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] shadow-[var(--meet-shadow)]">
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-[var(--meet-border-subtle)] bg-[var(--meet-chrome)] px-2 py-1.5 lg:px-3 lg:py-2">
        <div className="flex min-w-0 items-center gap-2">
          <Film size={16} className="shrink-0 text-red-400" />
          <div className="min-w-0 text-foreground">
            <p className="truncate text-sm font-medium">YouTube</p>
            <p className="truncate text-[11px] text-muted-foreground">{hostName} is sharing</p>
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
              aria-label="Stop YouTube share"
              onClick={onStop}
            >
              <X className="h-4 w-4" />
            </Button>
          ) : null}
        </div>
      </div>
      {children}
    </div>
  )
}

function YoutubeWatchPlayer({
  session,
  isHost,
  publishSync,
  remoteSyncNonce,
}: {
  session: YoutubeSession
  isHost: boolean
  publishSync: (playing: boolean, currentTime: number) => void
  remoteSyncNonce: number
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const applyingRemoteRef = useRef(false)
  const sessionRef = useRef(session)

  useEffect(() => {
    sessionRef.current = session
  }, [session])

  const { playerRef, readyVersion } = useYoutubePlayer(containerRef, {
    videoId: session.videoId,
    onReady: (player) => {
      const active = sessionRef.current
      if (!active) return
      if (active.currentTime > 0) player.seekTo(active.currentTime, true)
      if (active.playing) player.playVideo()
    },
    onStateChange: (state, player) => {
      if (!isHost || applyingRemoteRef.current || !isYoutubePlayerReady(player)) return
      const playing = state === window.YT!.PlayerState.PLAYING || state === window.YT!.PlayerState.BUFFERING
      const paused = state === window.YT!.PlayerState.PAUSED
      if (playing) publishSync(true, player.getCurrentTime())
      else if (paused) publishSync(false, player.getCurrentTime())
    },
  })

  const applyRemote = useCallback(() => {
    const player = playerRef.current
    if (!player || isHost || !isYoutubePlayerReady(player)) return

    applyingRemoteRef.current = true
    const localTime = player.getCurrentTime()
    if (Math.abs(localTime - session.currentTime) > SYNC_DRIFT_SECONDS) {
      player.seekTo(session.currentTime, true)
    }

    const state = player.getPlayerState()
    const isPlaying = state === window.YT!.PlayerState.PLAYING || state === window.YT!.PlayerState.BUFFERING
    if (session.playing && !isPlaying) player.playVideo()
    if (!session.playing && isPlaying) player.pauseVideo()

    window.setTimeout(() => {
      applyingRemoteRef.current = false
    }, 400)
  }, [isHost, playerRef, session])

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional trigger counters to force re-sync
  useEffect(() => {
    applyRemote()
  }, [applyRemote, remoteSyncNonce, readyVersion])

  useEffect(() => {
    if (!isHost) return
    const timer = window.setInterval(() => {
      const player = playerRef.current
      if (!isYoutubePlayerReady(player)) return
      const state = player.getPlayerState()
      const playing = state === window.YT!.PlayerState.PLAYING || state === window.YT!.PlayerState.BUFFERING
      publishSync(playing, player.getCurrentTime())
    }, HOST_HEARTBEAT_MS)
    return () => window.clearInterval(timer)
  }, [isHost, publishSync, playerRef])

  return (
    <div className="relative min-h-0 flex-1 bg-black">
      <div ref={containerRef} className="absolute inset-0 h-full w-full" />
    </div>
  )
}

export function YoutubeWatchOverlay() {
  const layout = useMeetingUILayout()
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)
  const { session, isHost, stopShare, publishSync, remoteSyncNonce } = useYoutubeWatch()
  const shellRef = useRef<HTMLDivElement>(null)
  const expand = useMeetingExpandChrome(shellRef, 'youtube-expand', { collapseMode: 'inline' })

  if (!youtubeEnabled || !session) return null

  const handleStop = () => {
    expand.collapse()
    stopShare()
  }

  const panel = (
    <YoutubeWatchPanel
      hostName={session.hostName}
      expanded={expand.expanded}
      onToggleExpand={expand.toggleExpanded}
      showStop={isHost}
      onStop={handleStop}
    >
      <YoutubeWatchPlayer
        session={session}
        isHost={isHost}
        publishSync={publishSync}
        remoteSyncNonce={remoteSyncNonce}
      />
    </YoutubeWatchPanel>
  )

  const expandedSurface =
    expand.expanded &&
    expand.shouldPortal &&
    expand.shellStyle &&
    createPortal(
      <div
        role="dialog"
        aria-modal="true"
        aria-label="YouTube fullscreen"
        data-youtube-overlay="true"
        className="meet-dialog fixed flex overflow-hidden border-0 bg-background text-foreground shadow-2xl"
        style={expand.shellStyle}
      >
        <MeetingExpandLeftRail
          activePanel={expand.activePanel}
          chromeDetail={expand.chromeDetail}
          onLeave={isHost ? handleStop : undefined}
          leaveLabel="Stop YouTube share"
        />
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">{panel}</div>
      </div>,
      document.body,
    )

  return (
    <>
      <div ref={shellRef} className={cn(meetStageShellClass(layout, 'p-2 max-lg:p-1.5'))}>
        <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">{!expand.expanded ? panel : null}</div>
      </div>
      {expandedSurface}
    </>
  )
}
