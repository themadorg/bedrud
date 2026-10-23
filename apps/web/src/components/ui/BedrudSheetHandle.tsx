import { Drawer } from 'vaul'

/** The drawn pill: Android's `Dimens.meetingHandleWidth` × `meetingHandleHeight`. */
const HANDLE_PILL_CLASS = 'h-1 w-8 rounded-full bg-[var(--meet-fg-muted)]'

/**
 * The grab bar every sheet in the app wears, and the one the call's controls bar already draws for
 * itself. One handle, one shape.
 *
 * The pill is 32 × 4 as on Android, but the target around it is a 48px full-width strip rather than
 * Android's 28dp padded box: WCAG 2.5.8 asks for 44px, and a drag target that is hard to catch is
 * worse on glass than it looks in an emulator. The strip is transparent, so it costs no visible
 * pixels.
 *
 * vaul injects its own `[data-vaul-handle]` stylesheet — a 32 × 5 grey bar — and injects it outside
 * any cascade layer, so it beats Tailwind's layered utilities whatever their specificity. The strip's
 * size and the cleared bar therefore carry Tailwind's important modifier, the documented way to win
 * over unlayered third-party CSS. vaul wraps the children in a hit-area span that it stretches over
 * the strip; that span centres the pill.
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
      className="!m-0 !h-12 !w-full shrink-0 cursor-grab !rounded-none !bg-transparent !opacity-100 active:cursor-grabbing *:flex *:items-center *:justify-center"
    >
      <span className={HANDLE_PILL_CLASS} />
    </Drawer.Handle>
  )
}
