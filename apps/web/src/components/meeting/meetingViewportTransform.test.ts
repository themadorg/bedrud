import { describe, expect, it } from 'vitest'
import {
  buildViewportTransformCss,
  clientToContentNorm,
  clampViewportZoom,
  contentNormToClient,
  contentNormToGridLocal,
  zoomAtClientPoint,
} from './meetingViewportTransform'

describe('meetingViewportTransform', () => {
  const rect = new DOMRect(100, 50, 200, 100)

  it('maps client coords into content-normalized space', () => {
    expect(clientToContentNorm(150, 100, rect, { panX: 0, panY: 0, zoom: 1 })).toEqual({
      x: 0.25,
      y: 0.5,
    })
    expect(clientToContentNorm(50, 100, rect, { panX: 0, panY: 0, zoom: 1 })).toBeNull()
  })

  it('accounts for pan when normalizing pointer coords', () => {
    expect(clientToContentNorm(200, 100, rect, { panX: 50, panY: 0, zoom: 1 })).toEqual({
      x: 0.25,
      y: 0.5,
    })
  })

  it('accounts for zoom when normalizing pointer coords', () => {
    expect(clientToContentNorm(150, 100, rect, { panX: 0, panY: 0, zoom: 2 })).toEqual({
      x: 0.125,
      y: 0.25,
    })
  })

  it('builds css transform with pan and zoom', () => {
    expect(buildViewportTransformCss({ panX: 10, panY: -4, zoom: 1.5 })).toBe(
      'translate(10px, -4px) scale(1.5)',
    )
  })

  it('clamps zoom', () => {
    expect(clampViewportZoom(0.1)).toBe(0.5)
    expect(clampViewportZoom(5)).toBe(2.5)
    expect(clampViewportZoom(1.2)).toBe(1.2)
  })

  it('anchors zoom to the cursor point', () => {
    const start = { panX: 0, panY: 0, zoom: 1 }
    const next = zoomAtClientPoint(start, rect, 200, 100, 2)
    expect(clientToContentNorm(200, 100, rect, next)).toEqual({ x: 0.5, y: 0.5 })
  })

  it('round-trips client coords through norm and back', () => {
    const transform = { panX: 24, panY: -12, zoom: 1.5 }
    const insets = { top: 50, right: 0, bottom: 0, left: 0 }
    const client = { x: 175, y: 140 }
    const norm = clientToContentNorm(client.x, client.y, rect, transform, insets)
    expect(norm).not.toBeNull()
    const back = contentNormToClient(norm!, rect, transform, insets)
    expect(back.x).toBeCloseTo(client.x, 5)
    expect(back.y).toBeCloseTo(client.y, 5)
  })

  it('maps norm coords to grid-local pixels without viewport offset', () => {
    const transform = { panX: 24, panY: -12, zoom: 1.5 }
    const insets = { top: 50, right: 0, bottom: 0, left: 0 }
    const norm = { x: 0.25, y: 0.5 }
    const gridLocal = contentNormToGridLocal(norm, rect, transform, insets)
    const client = contentNormToClient(norm, rect, transform, insets)
    expect(gridLocal.x).toBeCloseTo(client.x - rect.left, 5)
    expect(gridLocal.y).toBeCloseTo(client.y - rect.top, 5)
  })

  it('accounts for shell padding when normalizing pointer coords', () => {
    const insets = { top: 50, right: 0, bottom: 0, left: 0 }
    expect(clientToContentNorm(150, 125, rect, { panX: 0, panY: 0, zoom: 1 }, insets)).toEqual({
      x: 0.25,
      y: 0.5,
    })
    expect(clientToContentNorm(150, 90, rect, { panX: 0, panY: 0, zoom: 1 }, insets)).toBeNull()
  })
})