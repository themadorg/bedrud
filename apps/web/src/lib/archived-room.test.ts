import { describe, expect, it } from 'vitest'
import { type ArchivedOwnedResponse, recreateRoomRequest } from './archived-room'

function archivedRoom(isPublic: boolean): ArchivedOwnedResponse {
  return {
    status: 'archived_owned',
    name: 'team-standup',
    mode: 'standard',
    isPublic,
    settings: {
      allowChat: true,
      allowVideo: true,
      allowAudio: true,
      requireApproval: false,
      e2ee: false,
      isPersistent: false,
      recordingsAllowed: false,
    },
  }
}

describe('recreateRoomRequest', () => {
  // The create handler reads a missing isPublic as false, so an ended public room came back private.
  it('should keep a public room public', () => {
    expect(recreateRoomRequest(archivedRoom(true)).isPublic).toBe(true)
  })

  it('should keep a private room private', () => {
    expect(recreateRoomRequest(archivedRoom(false)).isPublic).toBe(false)
  })

  it('should carry the name, mode and settings without the status', () => {
    const archived = archivedRoom(true)
    expect(recreateRoomRequest(archived)).toEqual({
      name: archived.name,
      mode: archived.mode,
      isPublic: true,
      settings: archived.settings,
    })
  })
})
