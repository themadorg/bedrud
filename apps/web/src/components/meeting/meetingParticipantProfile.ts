import { api } from '#/lib/api'

export interface MeetingParticipantProfile {
  id: string
  name: string
  avatarUrl?: string
}

export async function fetchMeetingParticipantProfile(
  roomId: string,
  identity: string,
): Promise<MeetingParticipantProfile | null> {
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
