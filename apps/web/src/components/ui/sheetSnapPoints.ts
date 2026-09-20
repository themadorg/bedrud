/**
 * Where a tap on the sheet's handle should take it.
 *
 * vaul's own handle closes the sheet when it is tapped at the last snap point rather than stepping
 * back down, which is not what a two-height sheet wants: the tap is how a reader who expanded to
 * read returns to seeing the call behind it. The sheet passes vaul's `preventCycle` to take the tap
 * over, and this decides where it lands. Drag, velocity and dismissal remain vaul's.
 *
 * Anything that is not the last detent — a half-open sheet, a position mid-drag, or no detent at all
 * — expands, so a tap never has to be aimed.
 */
export function snapPointOnHandleTap(activeSnapPoint: number | string | null, snapPoints: readonly number[]): number {
  const lastSnapPoint = snapPoints[snapPoints.length - 1]
  return activeSnapPoint === lastSnapPoint ? snapPoints[0] : lastSnapPoint
}
