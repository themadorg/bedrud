import type { Participant } from 'livekit-client'
import { ParticipantEvent, Track, type TrackPublication } from 'livekit-client'
import { useEffect, useState } from 'react'

const CAMERA_TRACK_EVENTS = [
  ParticipantEvent.TrackPublished,
  ParticipantEvent.TrackUnpublished,
  ParticipantEvent.TrackMuted,
  ParticipantEvent.TrackUnmuted,
  ParticipantEvent.TrackSubscribed,
  ParticipantEvent.TrackUnsubscribed,
  ParticipantEvent.LocalTrackPublished,
  ParticipantEvent.LocalTrackUnpublished,
] as const

export function useCameraTrackPublication(participant: Participant): TrackPublication | undefined {
  const [cameraTrack, setCameraTrack] = useState(
    () => participant.getTrackPublication(Track.Source.Camera),
  )

  useEffect(() => {
    const refresh = () => setCameraTrack(participant.getTrackPublication(Track.Source.Camera))
    for (const event of CAMERA_TRACK_EVENTS) {
      participant.on(event, refresh)
    }
    return () => {
      for (const event of CAMERA_TRACK_EVENTS) {
        participant.off(event, refresh)
      }
    }
  }, [participant])

  return cameraTrack
}

export function hasCameraVideo(publication: TrackPublication | undefined): boolean {
  return Boolean(publication?.track && !publication.isMuted)
}