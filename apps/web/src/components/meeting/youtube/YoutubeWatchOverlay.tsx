import { X } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
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
    <div
      className={cn(
        'absolute top-[calc(56px+env(safe-area-inset-top))] left-0 bottom-[calc(88px+env(safe-area-inset-bottom))] z-[5] flex flex-col p-2 transition-[right] duration-200',
        meetRightInsetClass(layout),
      )}
    >
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-white/[0.06] bg-black shadow-xl">
        <div ref={containerRef} className="absolute inset-0 h-full w-full" />
        {isHost && (
          <button
            type="button"
            onClick={() => stopShare()}
            className="absolute top-2 right-2 z-10 flex h-8 w-8 items-center justify-center rounded-lg border-none bg-black/60 text-white/80 backdrop-blur-sm transition-colors hover:bg-black/80 hover:text-white"
            aria-label="Stop YouTube share"
          >
            <X size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
