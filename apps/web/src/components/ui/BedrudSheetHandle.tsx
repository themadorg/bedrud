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
 * The root is vaul's own `Drawer.Handle`, so the tap that cycles the snap points and the drag that
 * moves the sheet both stay with vaul. That makes the handle a bare `div` rather than a button, so
 * the accessible name has to be given rather than read from the element.
 *
 * `preventCycle` is for a sheet with a single height, which has nothing for a tap to cycle to. The
 * sheet is still dragged and dismissed the usual ways.
 */
export function BedrudSheetHandle({ label, preventCycle }: { label: string; preventCycle?: boolean }) {
  return (
    <Drawer.Handle
      preventCycle={preventCycle}
      aria-label={label}
      className="flex h-12 w-full shrink-0 cursor-grab items-center justify-center active:cursor-grabbing"
    >
      <span className={HANDLE_PILL_CLASS} />
    </Drawer.Handle>
  )
}
