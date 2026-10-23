import { describe, expect, it } from 'vitest'
import { HANDLE_TOUCH_SLOP_PX, isHandleTap, snapPointOnHandleTap } from './sheetSnapPoints'

const SNAP_POINTS = [0.5, 1]

describe('snapPointOnHandleTap', () => {
  it('should expand to full when tapped at half', () => {
    expect(snapPointOnHandleTap(0.5, SNAP_POINTS)).toBe(1)
  })

  it('should return to half when tapped at full, rather than dismissing', () => {
    expect(snapPointOnHandleTap(1, SNAP_POINTS)).toBe(0.5)
  })

  it('should expand when the sheet is between detents mid-drag', () => {
    expect(snapPointOnHandleTap(0.73, SNAP_POINTS)).toBe(1)
  })

  it('should expand when no detent is active yet', () => {
    expect(snapPointOnHandleTap(null, SNAP_POINTS)).toBe(1)
  })

  it('should have nowhere else to go on a sheet with one height', () => {
    expect(snapPointOnHandleTap(1, [1])).toBe(1)
  })
})

describe('isHandleTap', () => {
  const press = { clientX: 100, clientY: 400 }

  it('should count a release where the press began as a tap', () => {
    expect(isHandleTap(press, press)).toBe(true)
  })

  it('should count a release just inside the touch slop as a tap', () => {
    expect(isHandleTap(press, { clientX: 100, clientY: 400 - HANDLE_TOUCH_SLOP_PX })).toBe(true)
  })

  it('should count a release just past the touch slop as a drag', () => {
    expect(isHandleTap(press, { clientX: 100, clientY: 400 - HANDLE_TOUCH_SLOP_PX - 1 })).toBe(false)
  })

  it('should measure the slop along both axes together', () => {
    const diagonal = HANDLE_TOUCH_SLOP_PX / Math.SQRT2 + 1
    expect(isHandleTap(press, { clientX: 100 + diagonal, clientY: 400 + diagonal })).toBe(false)
  })

  it('should count a click with no recorded press as a tap, so a keyboard activation still works', () => {
    expect(isHandleTap(null, press)).toBe(true)
  })
})
