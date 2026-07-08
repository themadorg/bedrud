import type { LocalParticipant, Participant } from 'livekit-client'
import { Track } from 'livekit-client'

export function readPushToTalkFromMetadata(metadata?: string): boolean {
  try {
    const parsed = JSON.parse(metadata ?? '{}') as { pushToTalk?: unknown }
    return parsed.pushToTalk === true
  } catch {
    return false
  }
}

export function readPttMicHardMutedFromMetadata(metadata?: string): boolean {
  try {
    const parsed = JSON.parse(metadata ?? '{}') as { pttMicHardMuted?: unknown }
    return parsed.pttMicHardMuted === true
  } catch {
    return false
  }
}

export function isPushToTalkParticipant(participant: Participant, localPushToTalkEnabled = false): boolean {
  if (participant.isLocal) return localPushToTalkEnabled
  return readPushToTalkFromMetadata(participant.metadata)
}

export function shouldShowMicMutedIndicator(
  participant: Participant,
  localPushToTalkEnabled = false,
): boolean {
  if (isPushToTalkParticipant(participant, localPushToTalkEnabled)) {
    return readPttMicHardMutedFromMetadata(participant.metadata)
  }
  return !participant.isMicrophoneEnabled
}

export function patchParticipantMetadata(
  participant: { metadata?: string; setMetadata: (data: string) => Promise<void> },
  patch: Record<string, unknown>,
) {
  let base: Record<string, unknown> = {}
  try {
    base = JSON.parse(participant.metadata ?? '{}') as Record<string, unknown>
  } catch {
    base = {}
  }
  const next = JSON.stringify({ ...base, ...patch })
  if (participant.metadata === next) return
  void participant.setMetadata(next).catch(() => {})
}

export async function setLocalMicTrackMuted(localParticipant: LocalParticipant, muted: boolean) {
  if (!localParticipant.isMicrophoneEnabled) return

  const publication = localParticipant.getTrackPublication(Track.Source.Microphone)
  const track = publication?.track
  if (!track) return

  if (muted) {
    if (!track.isMuted) await track.mute()
  } else if (track.isMuted) {
    await track.unmute()
  }
}