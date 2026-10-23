/** The join answer for a room that has ended and is the joiner's own to start again. */
export interface EndedOwnRoom {
  name: string
  mode: string
  settings: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee: boolean
    isPersistent: boolean
    recordingsAllowed: boolean
  }
}

/** The fields of the owner's room list that the ended-room answer leaves out. */
export interface OwnRoomListing {
  name: string
  isPublic: boolean
  maxParticipants: number
  deletedAt?: string | null
}

export interface RestartRoomRequest extends EndedOwnRoom {
  isPublic?: boolean
  maxParticipants?: number
}

/**
 * Builds the request that starts an ended room again under the same name, mode and settings.
 *
 * The ended-room answer carries no public setting or participant limit, and the server makes a room
 * private when they are left out, so both come from the owner's room list. A room missing from the
 * list leaves them to the server rather than guessing; a deleted room sharing the name is skipped.
 */
export function restartRoomRequest(ended: EndedOwnRoom, ownRooms: OwnRoomListing[]): RestartRoomRequest {
  const request: RestartRoomRequest = { name: ended.name, mode: ended.mode, settings: ended.settings }
  const listed = ownRooms.find((room) => room.name === ended.name && !room.deletedAt)
  if (!listed) return request
  return { ...request, isPublic: listed.isPublic, maxParticipants: listed.maxParticipants }
}
