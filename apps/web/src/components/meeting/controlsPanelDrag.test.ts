import { describe, expect, it } from 'vitest'
import { expandedAfterDrag, PANEL_DRAG_THRESHOLD_PX } from './controlsPanelDrag'

describe('expandedAfterDrag', () => {
  it('should expand when a collapsed panel is dragged up past the threshold', () => {
    expect(expandedAfterDrag(false, -(PANEL_DRAG_THRESHOLD_PX + 1))).toBe(true)
  })

  it('should collapse when an expanded panel is dragged down past the threshold', () => {
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX + 1)).toBe(false)
  })

  it('should ignore a drag shorter than the threshold in either direction', () => {
    expect(expandedAfterDrag(false, -(PANEL_DRAG_THRESHOLD_PX - 1))).toBe(false)
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX - 1)).toBe(true)
  })

  it('should ignore a drag exactly at the threshold', () => {
    expect(expandedAfterDrag(false, -PANEL_DRAG_THRESHOLD_PX)).toBe(false)
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX)).toBe(true)
  })

  it('should leave the panel alone when dragged further in the direction it already sits', () => {
    expect(expandedAfterDrag(true, -(PANEL_DRAG_THRESHOLD_PX + 1))).toBe(true)
    expect(expandedAfterDrag(false, PANEL_DRAG_THRESHOLD_PX + 1)).toBe(false)
  })

  it('should ignore a drag that did not move', () => {
    expect(expandedAfterDrag(false, 0)).toBe(false)
    expect(expandedAfterDrag(true, 0)).toBe(true)
  })
})
