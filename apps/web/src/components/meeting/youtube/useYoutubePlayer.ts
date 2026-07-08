import { type RefObject, useEffect, useRef, useState } from 'react'
import { type YTPlayer, isYoutubePlayerReady, loadYoutubeIframeApi } from './loadYoutubeIframeApi'

interface UseYoutubePlayerOptions {
  videoId: string | null
  onReady?: (player: YTPlayer) => void
  onStateChange?: (state: number, player: YTPlayer) => void
}

export function useYoutubePlayer(
  containerRef: RefObject<HTMLDivElement | null>,
  { videoId, onReady, onStateChange }: UseYoutubePlayerOptions,
) {
  const playerRef = useRef<YTPlayer | null>(null)
  const onReadyRef = useRef(onReady)
  const onStateChangeRef = useRef(onStateChange)
  const [readyVersion, setReadyVersion] = useState(0)

  useEffect(() => {
    onReadyRef.current = onReady
  }, [onReady])

  useEffect(() => {
    onStateChangeRef.current = onStateChange
  }, [onStateChange])

  useEffect(() => {
    if (!videoId || !containerRef.current) return

    let cancelled = false
    const mountNode = containerRef.current
    playerRef.current = null
    setReadyVersion(0)

    void loadYoutubeIframeApi().then(() => {
      if (cancelled || !mountNode) return

      if (isYoutubePlayerReady(playerRef.current)) {
        playerRef.current.destroy()
      }
      playerRef.current = null

      new window.YT!.Player(mountNode, {
        videoId,
        playerVars: {
          autoplay: 0,
          rel: 0,
          modestbranding: 1,
          playsinline: 1,
        },
        events: {
          onReady: (event) => {
            if (cancelled) return
            playerRef.current = event.target
            setReadyVersion((v) => v + 1)
            onReadyRef.current?.(event.target)
          },
          onStateChange: (event) => {
            if (!isYoutubePlayerReady(event.target)) return
            onStateChangeRef.current?.(event.data, event.target)
          },
        },
      })
    })

    return () => {
      cancelled = true
      if (isYoutubePlayerReady(playerRef.current)) {
        playerRef.current.destroy()
      }
      playerRef.current = null
      setReadyVersion(0)
    }
  }, [videoId, containerRef])

  return { playerRef, readyVersion }
}
