/** What `POST /api/room/join` returns when the caller owns a room that has ended. */
export interface ArchivedOwnedResponse {
  status: 'archived_owned'
  name: string
  mode: string
  isPublic: boolean
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

/** Builds the `POST /api/room/create` body that restarts an ended room as it was configured. */
export function recreateRoomRequest(archived: ArchivedOwnedResponse) {
  return { name: archived.name, mode: archived.mode, isPublic: archived.isPublic, settings: archived.settings }
}
