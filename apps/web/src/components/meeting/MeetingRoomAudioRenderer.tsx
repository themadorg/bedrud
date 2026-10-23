import { RoomAudioRenderer, useAudioPlayback, useRoomContext } from '@livekit/components-react'
import { type RemoteParticipant, RoomEvent } from 'livekit-client'
import { VolumeX } from 'lucide-react'
import { useEffect } from 'react'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { Button } from '@/components/ui/button'

/**
 * Asks for a tap when the browser blocks remote audio. LiveKit only retries playback on its
 * own once a local mic stream starts, so a listener without a mic would otherwise stay silent.
 */
function StartAudioPrompt() {
  const room = useRoomContext()
  const { canPlayAudio, startAudio } = useAudioPlayback(room)

  if (canPlayAudio) return null

  return (
    <div
      role="alert"
      className="fixed bottom-[calc(100px+env(safe-area-inset-bottom))] left-1/2 -translate-x-1/2 z-60 flex items-center gap-3 bg-[var(--meet-bg-panel)] border border-[var(--meet-border)] rounded-xl px-4 py-3 shadow-[var(--meet-shadow)] backdrop-blur-lg max-w-[min(340px,calc(100vw-32px))]"
    >
      <VolumeX size={15} className="shrink-0 text-[var(--meet-fg-muted)]" aria-hidden="true" />
      <span className="flex-1 text-[13px] text-[var(--meet-fg)]">Your browser blocked meeting sound.</span>
      <Button type="button" size="sm" onClick={() => startAudio()}>
        Turn on sound
      </Button>
    </div>
  )
}

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
      room.remoteParticipants.forEach((participant) => {
        muteParticipant(participant)
      })
    }

    muteAll()

    const onParticipantConnected = (participant: RemoteParticipant) => {
      muteParticipant(participant)
    }

    const onTrackSubscribed = (_track: unknown, _publication: unknown, participant: RemoteParticipant) => {
      if (!participant.isLocal) muteParticipant(participant)
    }

    room.on(RoomEvent.ParticipantConnected, onParticipantConnected)
    room.on(RoomEvent.TrackSubscribed, onTrackSubscribed)
    return () => {
      room.off(RoomEvent.ParticipantConnected, onParticipantConnected)
      room.off(RoomEvent.TrackSubscribed, onTrackSubscribed)
    }
  }, [deafened, room])

  return (
    <>
      <RoomAudioRenderer volume={deafened ? 0 : 1} />
      <StartAudioPrompt />
    </>
  )
}
