import { RoomAudioRenderer, useRoomContext } from '@livekit/components-react'
import { type RemoteParticipant, RoomEvent } from 'livekit-client'
import { useEffect } from 'react'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'

/** Renders remote room audio and keeps it silent while the local user is deafened. */
export function MeetingRoomAudioRenderer() {
  const room = useRoomContext()
  const { isSelfDeafened, isServerDeafened } = useMeetingRoomContext()
  const deafened = isSelfDeafened || isServerDeafened

  useEffect(() => {
    if (!deafened) return

    const muteParticipant = (participant: RemoteParticipant) => {
      participant.setVolume(0)
    }

    const muteAll = () => {
      room.remoteParticipants.forEach((participant) => muteParticipant(participant))
    }

    muteAll()

    const onParticipantConnected = (participant: RemoteParticipant) => {
      muteParticipant(participant)
    }

    const onTrackSubscribed = (
      _track: unknown,
      _publication: unknown,
      participant: RemoteParticipant,
    ) => {
      if (!participant.isLocal) muteParticipant(participant)
    }

    room.on(RoomEvent.ParticipantConnected, onParticipantConnected)
    room.on(RoomEvent.TrackSubscribed, onTrackSubscribed)
    return () => {
      room.off(RoomEvent.ParticipantConnected, onParticipantConnected)
      room.off(RoomEvent.TrackSubscribed, onTrackSubscribed)
    }
  }, [deafened, room])

  return <RoomAudioRenderer volume={deafened ? 0 : 1} />
}