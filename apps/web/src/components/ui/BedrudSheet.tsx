import type { ReactNode } from 'react'
import { Drawer } from 'vaul'
import { BedrudSheetHandle } from '@/components/ui/BedrudSheetHandle'

/**
 * The two heights every sheet in the app offers, as fractions of its own content height. Half is
 * where a sheet rests when it opens, so the call above it still reads.
 */
export const SHEET_SNAP_POINTS: number[] = [0.5, 1]

interface BedrudSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Names the sheet for assistive technology, and labels the handle's tap. */
  label: string
  activeSnapPoint: number | string | null
  onSnapPointChange: (snapPoint: number | string | null) => void
  /** Marker attributes the meeting stylesheet selects on. */
  dataMarkers?: Record<string, string>
  children: ReactNode
}

/**
 * The app's bottom sheet. Every sheet in the app is one of these.
 *
 * Corner, container, handle, bottom inset and gutter are fixed rather than defaulted. A default is a
 * suggestion, and the one caller that takes the suggestion up is the one that renders wrong — the
 * Android component this mirrors carries the same note for the same reason.
 *
 * Drag, velocity, snapping and dismissal are vaul's. `snapToSequentialPoint` keeps a fast flick from
 * skipping half on the way down, so the spec's "a second drag down dismisses" holds rather than one
 * hard flick closing from full.
 */
export function BedrudSheet({
  open,
  onOpenChange,
  label,
  activeSnapPoint,
  onSnapPointChange,
  dataMarkers,
  children,
}: BedrudSheetProps) {
  return (
    <Drawer.Root
      open={open}
      onOpenChange={onOpenChange}
      snapPoints={SHEET_SNAP_POINTS}
      activeSnapPoint={activeSnapPoint}
      setActiveSnapPoint={onSnapPointChange}
      snapToSequentialPoint
      dismissible
      modal
    >
      <Drawer.Portal>
        <Drawer.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Drawer.Content
          aria-label={label}
          {...dataMarkers}
          className="fixed right-0 bottom-0 left-0 z-40 flex max-h-[var(--meet-sheet-max-height)] flex-col rounded-t-3xl bg-[var(--meet-sidebar)] backdrop-blur-2xl pb-[env(safe-area-inset-bottom,0px)]"
        >
          <Drawer.Title className="sr-only">{label}</Drawer.Title>
          <BedrudSheetHandle label={`Resize ${label}`} />
          <div className="flex min-h-0 flex-1 flex-col px-4">{children}</div>
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
