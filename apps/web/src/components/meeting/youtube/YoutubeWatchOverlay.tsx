import { X } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { meetStageShellClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { isYoutubePlayerReady } from './loadYoutubeIframeApi'
import { useYoutubePlayer } from './useYoutubePlayer'
import { useYoutubeWatch } from './youtube-watch-context'

const SYNC_DRIFT_SECONDS = 1.5
const HOST_HEARTBEAT_MS = 2500

export function YoutubeWatchOverlay() {
  const layout = useMeetingUILayout()
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)
  const { session, isHost, stopShare, publishSync, remoteSyncNonce } = useYoutubeWatch()
  const containerRef = useRef<HTMLDivElement>(null)
  const applyingRemoteRef = useRef(false)
  const sessionRef = useRef(session)

  useEffect(() => {
    sessionRef.current = session
  }, [session])

  const { playerRef, readyVersion } = useYoutubePlayer(containerRef, {
    videoId: session?.videoId ?? null,
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

    const remote = session
    if (!remote) return

    applyingRemoteRef.current = true
    const localTime = player.getCurrentTime()
    if (Math.abs(localTime - remote.currentTime) > SYNC_DRIFT_SECONDS) {
      player.seekTo(remote.currentTime, true)
    }

    const state = player.getPlayerState()
    const isPlaying = state === window.YT!.PlayerState.PLAYING || state === window.YT!.PlayerState.BUFFERING
    if (remote.playing && !isPlaying) player.playVideo()
    if (!remote.playing && isPlaying) player.pauseVideo()

    window.setTimeout(() => {
      applyingRemoteRef.current = false
    }, 400)
  }, [isHost, playerRef, session])

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional trigger counters to force re-sync
  useEffect(() => {
    applyRemote()
  }, [applyRemote, remoteSyncNonce, readyVersion])

  useEffect(() => {
    if (!isHost || !session) return
    const timer = window.setInterval(() => {
      const player = playerRef.current
      if (!isYoutubePlayerReady(player)) return
      const state = player.getPlayerState()
      const playing = state === window.YT!.PlayerState.PLAYING || state === window.YT!.PlayerState.BUFFERING
      publishSync(playing, player.getCurrentTime())
    }, HOST_HEARTBEAT_MS)
    return () => window.clearInterval(timer)
  }, [isHost, session, publishSync, playerRef])

  if (!youtubeEnabled || !session) return null

  return (
    <div className={cn(meetStageShellClass(layout, 'p-2 max-sm:p-1.5'))}>
      <div className="meet-dialog flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] shadow-[var(--meet-shadow)]">
        {isHost && (
          <div className="flex shrink-0 items-center justify-end border-b border-[var(--meet-border-subtle)] bg-[var(--meet-chrome)] px-2 py-1.5 sm:px-3 sm:py-2">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label="Stop YouTube share"
              onClick={() => stopShare()}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        )}
        <div className="relative min-h-0 flex-1 bg-black">
          <div ref={containerRef} className="absolute inset-0 h-full w-full" />
        </div>
      </div>
    </div>
  )
}
