import { api } from '#/lib/api'

export interface MeetingParticipantProfile {
  id: string
  name: string
  avatarUrl?: string
  /** Empty while welcome sounds are off on the server, or when the participant has none. */
  welcomeSoundUrl?: string
}

// The meeting context and the join sound both ask for a profile on the same join event.
const inFlight = new Map<string, Promise<MeetingParticipantProfile | null>>()

async function requestProfile(roomId: string, identity: string): Promise<MeetingParticipantProfile | null> {
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      return await api.get<MeetingParticipantProfile>(
        `/api/room/${encodeURIComponent(roomId)}/participant/${encodeURIComponent(identity)}/profile`,
      )
    } catch {
      if (attempt === 0) {
        await new Promise((resolve) => setTimeout(resolve, 400))
        continue
      }
      return null
    }
  }
  return null
}

export function fetchMeetingParticipantProfile(
  roomId: string,
  identity: string,
): Promise<MeetingParticipantProfile | null> {
  const key = `${roomId}/${identity}`
  const pending = inFlight.get(key)
  if (pending) return pending
  const request = requestProfile(roomId, identity).finally(() => inFlight.delete(key))
  inFlight.set(key, request)
  return request
}
