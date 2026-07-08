interface YTPlayer {
  playVideo(): void
  pauseVideo(): void
  seekTo(seconds: number, allowSeekAhead: boolean): void
  getCurrentTime(): number
  getPlayerState(): number
  destroy(): void
}

interface YTPlayerConfig {
  videoId?: string
  playerVars?: Record<string, number | string>
  events?: {
    onReady?: (event: { target: YTPlayer }) => void
    onStateChange?: (event: { data: number; target: YTPlayer }) => void
  }
}

declare global {
  interface Window {
    YT?: {
      Player: new (element: HTMLElement | string, config: YTPlayerConfig) => YTPlayer
      PlayerState: {
        UNSTARTED: -1
        ENDED: 0
        PLAYING: 1
        PAUSED: 2
        BUFFERING: 3
        CUED: 5
      }
    }
    onYouTubeIframeAPIReady?: () => void
  }
}

export type { YTPlayer }

export function isYoutubePlayerReady(player: YTPlayer | null | undefined): player is YTPlayer {
  return typeof player?.getCurrentTime === 'function' && typeof player.getPlayerState === 'function'
}

let loadPromise: Promise<void> | null = null

export function loadYoutubeIframeApi(): Promise<void> {
  if (window.YT?.Player) return Promise.resolve()
  if (loadPromise) return loadPromise

  loadPromise = new Promise((resolve) => {
    if (window.YT?.Player) {
      resolve()
      return
    }

    const previous = window.onYouTubeIframeAPIReady
    window.onYouTubeIframeAPIReady = () => {
      previous?.()
      resolve()
    }

    const existing = document.querySelector('script[data-youtube-iframe-api]')
    if (existing) return

    const tag = document.createElement('script')
    tag.src = 'https://www.youtube.com/iframe_api'
    tag.async = true
    tag.dataset.youtubeIframeApi = 'true'
    document.head.appendChild(tag)
  })

  return loadPromise
}
