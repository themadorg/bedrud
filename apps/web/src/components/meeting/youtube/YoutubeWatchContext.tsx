import { useRoomContext } from '@livekit/components-react'
import { type ReactNode, useCallback, useMemo, useState } from 'react'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import type { MeetingStage } from '@/components/meeting/stage/stageWire'
import { parseYoutubeVideoId } from './parseYoutubeVideoId'
import { YoutubeWatchContext, type YoutubeWatchContextValue } from './youtube-watch-context'
import type { YoutubeSession } from './youtubeWire'

export { useYoutubeWatch } from './youtube-watch-context'
export type { YoutubeWatchContextValue }

function sessionFromStage(stage: Extract<MeetingStage, { kind: 'youtube' }>): YoutubeSession {
  return {
    videoId: stage.videoId,
    hostIdentity: stage.ownerIdentity,
    hostName: stage.ownerName,
    playing: stage.playing,
    currentTime: stage.currentTime,
    updatedAt: stage.updatedAt,
  }
}

export function YoutubeWatchProvider({ children }: { children: ReactNode }) {
  const room = useRoomContext()
  const { stage, isOwner, claimStage, clearStage, updateYoutubeStage, youtubeSyncNonce } = useMeetingStage()
  const [shareDialogOpen, setShareDialogOpen] = useState(false)
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)

  const session = useMemo(() => {
    if (stage?.kind !== 'youtube') return null
    return sessionFromStage(stage)
  }, [stage])

  const isHost = stage?.kind === 'youtube' && isOwner

  const shareVideo = useCallback(
    (url: string) => {
      if (!youtubeEnabled) {
        return 'Enable YouTube in Settings → Experimental'
      }
      const videoId = parseYoutubeVideoId(url)
      if (!videoId) return 'Enter a valid YouTube URL or video ID'

      const err = claimStage('youtube', { videoId, playing: false, currentTime: 0 })
      if (err) return err

      setShareDialogOpen(false)
      return null
    },
    [claimStage, youtubeEnabled],
  )

  const stopShare = useCallback(() => {
    if (stage?.kind !== 'youtube' || stage.ownerIdentity !== room.localParticipant.identity) return
    clearStage()
  }, [clearStage, room.localParticipant.identity, stage])

  const publishSync = useCallback(
    (playing: boolean, currentTime: number) => {
      updateYoutubeStage(playing, currentTime)
    },
    [updateYoutubeStage],
  )

  const value = useMemo(
    () => ({
      session,
      isHost,
      shareDialogOpen,
      openShareDialog: () => {
        if (!youtubeEnabled) return
        setShareDialogOpen(true)
      },
      closeShareDialog: () => setShareDialogOpen(false),
      shareVideo,
      stopShare,
      publishSync,
      remoteSyncNonce: youtubeSyncNonce,
    }),
    [session, isHost, shareDialogOpen, shareVideo, stopShare, publishSync, youtubeSyncNonce, youtubeEnabled],
  )

  return <YoutubeWatchContext.Provider value={value}>{children}</YoutubeWatchContext.Provider>
}
