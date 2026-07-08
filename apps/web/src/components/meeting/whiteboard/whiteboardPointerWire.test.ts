import { describe, expect, it } from 'vitest'
import {
  decodeWhiteboardPointerPacket,
  encodeWhiteboardPointerPacket,
  type WhiteboardPointerPacket,
} from './whiteboardPointerWire'

describe('whiteboardPointerWire', () => {
  const laserSample: WhiteboardPointerPacket = {
    identity: 'user-1',
    username: 'Alice',
    pointer: { x: 120, y: 340, tool: 'laser' },
    button: 'down',
  }

  const pointerSample: WhiteboardPointerPacket = {
    identity: 'user-2',
    username: 'Bob',
    pointer: { x: 44, y: 88, tool: 'pointer', renderCursor: true },
    button: 'up',
  }

  it('round-trips laser pointer packets', () => {
    const encoded = encodeWhiteboardPointerPacket(laserSample)
    expect(decodeWhiteboardPointerPacket(encoded)).toEqual(laserSample)
  })

  it('round-trips selection pointer packets', () => {
    const encoded = encodeWhiteboardPointerPacket(pointerSample)
    expect(decodeWhiteboardPointerPacket(encoded)).toEqual(pointerSample)
  })

  it('rejects invalid packets', () => {
    expect(decodeWhiteboardPointerPacket(new TextEncoder().encode('not json'))).toBeNull()
    expect(decodeWhiteboardPointerPacket(new TextEncoder().encode('{}'))).toBeNull()
  })
})