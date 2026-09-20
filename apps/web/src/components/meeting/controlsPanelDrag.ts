/**
 * How far a vertical drag must travel before it moves the panel. Android's
 * `Dimens.meetingHandleSwipeThreshold`.
 */
export const PANEL_DRAG_THRESHOLD_PX = 24

/**
 * Resolves a finished vertical drag against the panel's current state. `deltaY` is the pointer's
 * total travel, negative upward. Returns the state the panel should be in, which equals the state
 * it was already in when the drag was too short, or pushed it further in the direction it already
 * sits.
 */
export function expandedAfterDrag(expanded: boolean, deltaY: number): boolean {
  if (deltaY < -PANEL_DRAG_THRESHOLD_PX) return true
  if (deltaY > PANEL_DRAG_THRESHOLD_PX) return false
  return expanded
}
