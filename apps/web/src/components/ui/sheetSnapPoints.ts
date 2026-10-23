/** How far a press may travel and still be a tap: Android's `ViewConfiguration` touch slop. */
export const HANDLE_TOUCH_SLOP_PX = 8

interface PointerPosition {
  clientX: number
  clientY: number
}

/**
 * Whether a click on the handle was a tap rather than the end of a drag.
 *
 * The handle moves with the finger while the sheet is dragged, so the release lands on the handle
 * and the browser fires a click for it. Replacing vaul's tap also replaced the guard vaul keeps for
 * exactly this, so a drag that released on the handle stepped the sheet a second time. A click with
 * no recorded press did not come from a pointer and is always a tap.
 */
export function isHandleTap(press: PointerPosition | null, release: PointerPosition): boolean {
  if (press === null) return true
  return Math.hypot(release.clientX - press.clientX, release.clientY - press.clientY) <= HANDLE_TOUCH_SLOP_PX
}

/**
 * Where a tap on the sheet's handle should take it.
 *
 * vaul's own handle closes the sheet when it is tapped at the last snap point rather than stepping
 * back down, which is not what a two-height sheet wants: the tap is how a reader who expanded to
 * read returns to seeing the call behind it. The handle's own `onClick` replaces vaul's tap, and
 * this decides where it lands. Drag, velocity and dismissal remain vaul's.
 *
 * Anything that is not the last detent — a half-open sheet, a position mid-drag, or no detent at all
 * — expands, so a tap never has to be aimed.
 */
export function snapPointOnHandleTap(activeSnapPoint: number | string | null, snapPoints: readonly number[]): number {
  const lastSnapPoint = snapPoints[snapPoints.length - 1]
  return activeSnapPoint === lastSnapPoint ? snapPoints[0] : lastSnapPoint
}
