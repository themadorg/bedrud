// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { type EndedOwnRoom, restartRoomRequest } from './room-restart'

const ENDED: EndedOwnRoom = {
  name: 'weekly-sync',
  mode: 'meeting',
  settings: {
    allowChat: false,
    allowVideo: true,
    allowAudio: true,
    requireApproval: false,
    e2ee: false,
    isPersistent: false,
    recordingsAllowed: false,
  },
}

describe('restartRoomRequest', () => {
  it('should carry the public setting and participant limit over from the owner room list', () => {
    const request = restartRoomRequest(ENDED, [{ name: 'weekly-sync', isPublic: true, maxParticipants: 12 }])
    expect(request).toEqual({
      name: 'weekly-sync',
      mode: 'meeting',
      settings: ENDED.settings,
      isPublic: true,
      maxParticipants: 12,
    })
  })

  it('should skip a deleted room sharing the name', () => {
    const request = restartRoomRequest(ENDED, [
      { name: 'weekly-sync', isPublic: false, maxParticipants: 4, deletedAt: '2026-10-01T00:00:00Z' },
      { name: 'weekly-sync', isPublic: true, maxParticipants: 12 },
    ])
    expect(request.isPublic).toBe(true)
    expect(request.maxParticipants).toBe(12)
  })

  it('should leave both to the server when the room is not in the list', () => {
    const request = restartRoomRequest(ENDED, [{ name: 'other', isPublic: true, maxParticipants: 3 }])
    expect(request).not.toHaveProperty('isPublic')
    expect(request).not.toHaveProperty('maxParticipants')
  })
})
