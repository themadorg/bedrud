import { describe, expect, it } from 'vitest'
import { snapPointOnHandleTap } from './sheetSnapPoints'

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
