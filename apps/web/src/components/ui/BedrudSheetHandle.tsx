import { cn } from '@/lib/utils'

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
 * `onClick` is optional, as it is on Android. A sheet with one height has nothing for a tap to do,
 * so it gets a plain strip with no role and no label — the sheet is still dragged and dismissed the
 * usual ways.
 */
export function BedrudSheetHandle({ onClick, label }: { onClick?: () => void; label?: string }) {
  const strip = 'flex h-12 w-full shrink-0 cursor-grab items-center justify-center active:cursor-grabbing'

  if (!onClick) {
    return (
      <div className={strip} aria-hidden="true">
        <span className={HANDLE_PILL_CLASS} />
      </div>
    )
  }

  return (
    <button type="button" onClick={onClick} className={cn(strip, 'border-none bg-transparent')} aria-label={label}>
      <span className={HANDLE_PILL_CLASS} />
    </button>
  )
}
