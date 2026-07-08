import { describe, expect, it } from 'vitest'
import { clientToSurfaceNorm, decodeMeetingPointerPacket, encodeMeetingPointerPacket } from './meetingPointerWire'

describe('meetingPointerWire', () => {
  it('maps client coords into surface-normalized space', () => {
    const rect = new DOMRect(100, 50, 200, 100)
    expect(clientToSurfaceNorm(150, 100, rect)).toEqual({ x: 0.25, y: 0.5 })
    expect(clientToSurfaceNorm(50, 100, rect)).toBeNull()
    expect(clientToSurfaceNorm(350, 100, rect)).toBeNull()
  })

  it('accounts for pan and zoom when mapping pointer coords', () => {
    const rect = new DOMRect(100, 50, 200, 100)
    expect(clientToSurfaceNorm(200, 100, rect, { panX: 50, panY: 0, zoom: 1 })).toEqual({
      x: 0.25,
      y: 0.5,
    })
    expect(clientToSurfaceNorm(150, 100, rect, { panX: 0, panY: 0, zoom: 2 })).toEqual({
      x: 0.125,
      y: 0.25,
    })
  })

  it('subtracts meet-grid padding before normalizing', () => {
    const rect = new DOMRect(100, 50, 200, 100)
    const insets = { top: 50, right: 0, bottom: 0, left: 0 }
    expect(clientToSurfaceNorm(150, 125, rect, { panX: 0, panY: 0, zoom: 1 }, insets)).toEqual({
      x: 0.25,
      y: 0.5,
    })
  })

  it('round-trips pointer packets', () => {
    const packet = {
      identity: 'guest-abc',
      username: 'Alex',
      x: 0.42,
      y: 0.77,
      visible: true,
    }
    const decoded = decodeMeetingPointerPacket(encodeMeetingPointerPacket(packet))
    expect(decoded).toEqual(packet)
  })

  it('clamps coordinates', () => {
    const decoded = decodeMeetingPointerPacket(
      encodeMeetingPointerPacket({
        identity: 'u1',
        username: 'U',
        x: 2,
        y: -1,
        visible: true,
      }),
    )
    expect(decoded?.x).toBe(1)
    expect(decoded?.y).toBe(0)
  })
})
