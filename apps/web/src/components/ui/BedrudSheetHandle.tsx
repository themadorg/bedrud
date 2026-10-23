import { Drawer } from 'vaul'

/**
 * The drawn pill: Android's `Dimens.meetingHandleWidth` × `meetingHandleHeight`, at the 0.55 alpha
 * both Android handles draw. Exported for the controls pill, which is no drawer and so cannot host a
 * `Drawer.Handle`.
 */
export const HANDLE_PILL_CLASS = 'h-1 w-8 rounded-full bg-[var(--meet-fg-muted)] opacity-55'

/**
 * The grab bar every sheet in the app wears, sharing its pill with the call's controls bar. One
 * handle, one shape.
 *
 * The pill is 32 × 4 as on Android, but the target around it is a 48px full-width strip rather than
 * Android's 28dp padded box: WCAG 2.5.8 asks for 44px, and a drag target that is hard to catch is
 * worse on glass than it looks in an emulator. The strip is transparent, so it costs no visible
 * pixels.
 *
 * The root is vaul's own `Drawer.Handle`, so the drag that moves the sheet stays with vaul. That
 * makes the handle a bare `div` rather than a button, so the accessible name has to be given rather
 * than read from the element.
 *
 * `preventCycle` turns off vaul's own tap handling, which closes the sheet at the last snap point
 * instead of stepping back down. A sheet that wants the tap to move between its own heights passes
 * it along with an `onClick`.
 */
export function BedrudSheetHandle({
  label,
  preventCycle,
  onClick,
}: {
  label: string
  preventCycle?: boolean
  onClick?: () => void
}) {
  return (
    <Drawer.Handle
      preventCycle={preventCycle}
      onClick={onClick}
      aria-label={label}
      className="flex h-12 w-full shrink-0 cursor-grab items-center justify-center active:cursor-grabbing"
    >
      <span className={HANDLE_PILL_CLASS} />
    </Drawer.Handle>
  )
}
