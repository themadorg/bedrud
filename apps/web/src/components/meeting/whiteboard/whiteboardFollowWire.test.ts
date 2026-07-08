import { describe, expect, it } from 'vitest'
import {
  decodeWhiteboardFollowPacket,
  encodeWhiteboardFollowPacket,
  type WhiteboardFollowChangePacket,
  type WhiteboardViewportPacket,
} from './whiteboardFollowWire'

describe('whiteboardFollowWire', () => {
  const followPacket: WhiteboardFollowChangePacket = {
    type: 'follow-change',
    followerId: 'user-a',
    followerName: 'Alice',
    targetId: 'user-b',
    action: 'FOLLOW',
  }

  const viewportPacket: WhiteboardViewportPacket = {
    type: 'viewport',
    identity: 'user-b',
    sceneBounds: [0, 0, 1200, 800],
  }

  it('round-trips follow-change packets', () => {
    const encoded = encodeWhiteboardFollowPacket(followPacket)
    expect(decodeWhiteboardFollowPacket(encoded)).toEqual(followPacket)
  })

  it('round-trips viewport packets', () => {
    const encoded = encodeWhiteboardFollowPacket(viewportPacket)
    expect(decodeWhiteboardFollowPacket(encoded)).toEqual(viewportPacket)
  })

  it('rejects invalid packets', () => {
    expect(decodeWhiteboardFollowPacket(new TextEncoder().encode('not json'))).toBeNull()
    expect(decodeWhiteboardFollowPacket(new TextEncoder().encode('{}'))).toBeNull()
  })
})