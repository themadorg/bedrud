import type { TrackReference, TrackReferenceOrPlaceholder } from '@livekit/components-react'
import type { LocalParticipant } from 'livekit-client'
import { Track } from 'livekit-client'

/** Local screen-share publishes are not always present in onlySubscribed track lists. */
export function resolveScreenShareTrack(
  stageOwnerIdentity: string | null,
  localParticipant: LocalParticipant,
  subscribedTracks: TrackReferenceOrPlaceholder[],
): TrackReference | undefined {
  if (stageOwnerIdentity && localParticipant.identity === stageOwnerIdentity) {
    const publication = localParticipant.getTrackPublication(Track.Source.ScreenShare)
    if (publication?.track) {
      return {
        participant: localParticipant,
        publication,
        source: Track.Source.ScreenShare,
      }
    }
  }

  const ownerTrack = stageOwnerIdentity
    ? subscribedTracks.find((t) => t.participant.identity === stageOwnerIdentity && t.publication)
    : undefined

  const fallback = subscribedTracks.find((t) => t.publication)
  const picked = ownerTrack ?? fallback
  return picked?.publication ? (picked as TrackReference) : undefined
}
