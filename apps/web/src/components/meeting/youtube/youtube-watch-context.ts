import { createContext, useContext } from 'react'
import type { YoutubeSession } from './youtubeWire'

export interface YoutubeWatchContextValue {
  session: YoutubeSession | null
  isHost: boolean
  shareDialogOpen: boolean
  openShareDialog: () => void
  closeShareDialog: () => void
  shareVideo: (url: string) => string | null
  stopShare: () => void
  publishSync: (playing: boolean, currentTime: number) => void
  remoteSyncNonce: number
}

/** Isolated module so provider and hooks always share one React context instance. */
export const YoutubeWatchContext = createContext<YoutubeWatchContextValue | null>(null)

export function useYoutubeWatch(): YoutubeWatchContextValue {
  const ctx = useContext(YoutubeWatchContext)
  if (!ctx) throw new Error('useYoutubeWatch must be used inside YoutubeWatchProvider')
  return ctx
}
