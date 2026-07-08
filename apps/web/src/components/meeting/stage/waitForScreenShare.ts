import type { LocalParticipant } from 'livekit-client'
import { ParticipantEvent, Track } from 'livekit-client'

export function waitForScreenSharePublication(participant: LocalParticipant, timeoutMs = 12_000): Promise<boolean> {
  if (participant.getTrackPublication(Track.Source.ScreenShare)?.track) {
    return Promise.resolve(true)
  }

  return new Promise((resolve) => {
    const onPublished = (publication: { source: Track.Source; track?: unknown }) => {
      if (publication.source !== Track.Source.ScreenShare || !publication.track) return
      cleanup()
      resolve(true)
    }

    const timer = window.setTimeout(() => {
      cleanup()
      resolve(Boolean(participant.getTrackPublication(Track.Source.ScreenShare)?.track))
    }, timeoutMs)

    const cleanup = () => {
      window.clearTimeout(timer)
      participant.off(ParticipantEvent.LocalTrackPublished, onPublished)
    }

    participant.on(ParticipantEvent.LocalTrackPublished, onPublished)
  })
}
