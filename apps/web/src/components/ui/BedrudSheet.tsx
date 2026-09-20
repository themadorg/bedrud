import type { ReactNode } from 'react'
import { Drawer } from 'vaul'
import { BedrudSheetHandle } from '@/components/ui/BedrudSheetHandle'
import { snapPointOnHandleTap } from '@/components/ui/sheetSnapPoints'

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
 *
 * The height is set rather than capped. vaul reads its fractional snap points against the content's
 * own height, so a sheet that is only as tall as its content makes "half" half of that content and
 * leaves the rest translated off the bottom of the screen. Setting the height makes the fractions
 * mean what the spec says they mean: half and full of the visible viewport.
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
          className="fixed right-0 bottom-0 left-0 z-40 flex h-[var(--meet-sheet-max-height)] flex-col rounded-t-3xl bg-[var(--meet-sidebar)] backdrop-blur-2xl pb-[env(safe-area-inset-bottom,0px)]"
        >
          <Drawer.Title className="sr-only">{label}</Drawer.Title>
          <BedrudSheetHandle
            label={`Resize ${label}`}
            preventCycle
            onClick={() => onSnapPointChange(snapPointOnHandleTap(activeSnapPoint, SHEET_SNAP_POINTS))}
          />
          {/*
            The body is sized to the band the current snap point actually shows, not to the sheet.
            vaul keeps the sheet at its full height and slides it down, so a body that filled the
            sheet would put its last row — for chat, the composer — below the bottom of the screen at
            every snap point short of full. `--snap-point-height` is how far vaul has slid the sheet
            down, which is 0 once it is fully open, so it is subtracted rather than used as a height.
            The handle's 3rem comes off as well, because the handle sits above the body.
          */}
          <div className="flex min-h-0 flex-col px-4 h-[calc(100%-3rem-var(--snap-point-height,0px))]">{children}</div>
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
