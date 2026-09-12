import { useCallback, useEffect, useRef, useState } from 'react'
import { expandedAfterDrag } from '@/components/meeting/controlsPanelDrag'
import { MeetingCallControlsRow } from '@/components/meeting/MeetingCallControlsRow'
import { MeetingOptionsPanel } from '@/components/meeting/MeetingOptionsPanel'
import type { MeetingOptionRow, MeetingOptionRowId } from '@/components/meeting/meetingOptionRows'
import { cn } from '@/lib/utils'

/** The grab bar: Android's 32 × 4dp, inside a 48px tap target spanning the surface. */
const HANDLE_PILL_CLASS = 'h-1 w-8 rounded-full bg-[var(--meet-fg-muted)] opacity-55'

interface MeetingControlsPillProps {
  rows: MeetingOptionRow[]
  onSelectOption: (id: MeetingOptionRowId) => void
  cameraEnabled: boolean
  onToggleCamera: () => void
  screenShareEnabled: boolean
  screenShareAvailable: boolean
  onToggleScreenShare: () => void
  micPushToTalk: boolean
  micOpen: boolean
  micTransmitting: boolean
  micAvailable: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
  chatOpen: boolean
  unreadCount: number
  onToggleChat: () => void
  onLeave: () => void
}

/**
 * The phone in-call controls, and the room options that grow out of them.
 *
 * One surface, anchored to the bottom, so the options unfold *above* the controls and the row your
 * thumb is resting on never moves — the pill simply becomes taller. This mirrors Android's
 * `MeetingControlsPanel`, which is deliberately not built on the app's sheet primitive for the
 * same reason: as a sheet, the options arrived as a second surface carrying a duplicate copy of
 * the same controls at a different height.
 */
export function MeetingControlsPill({
  rows,
  onSelectOption,
  chatOpen,
  onToggleChat,
  onLeave,
  ...controls
}: MeetingControlsPillProps) {
  const [expanded, setExpanded] = useState(false)
  const dragStartRef = useRef<number | null>(null)

  const collapse = useCallback(() => setExpanded(false), [])

  // Escape is the web's answer to Android's BackHandler.
  useEffect(() => {
    if (!expanded) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') collapse()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [expanded, collapse])

  const handlePointerDown = (event: React.PointerEvent<HTMLButtonElement>) => {
    dragStartRef.current = event.clientY
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const handlePointerUp = (event: React.PointerEvent<HTMLButtonElement>) => {
    const start = dragStartRef.current
    dragStartRef.current = null
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    if (start === null) return
    const next = expandedAfterDrag(expanded, event.clientY - start)
    // A drag the rule declined to act on was a tap on the handle, which toggles.
    setExpanded(next === expanded ? !expanded : next)
  }

  // Rows that lead somewhere else close the panel on the way; toggles leave it open so the flip is
  // visible. The row list says which is which.
  const selectRow = (id: MeetingOptionRowId) => {
    const row = rows.find((candidate) => candidate.id === id)
    if (row && row.kind === 'action') collapse()
    onSelectOption(id)
  }

  return (
    <>
      {expanded && (
        <button
          type="button"
          aria-label="Close room options"
          onClick={collapse}
          className="fixed inset-0 z-20 border-none bg-[var(--meet-scrim)] transition-opacity duration-[320ms] ease-[cubic-bezier(0.4,0,0.2,1)]"
        />
      )}

      <div
        id="meet-controls"
        // meet-controls-bar: the corner comes from the token, which needs !important against the
        // utilities on the same element.
        className={cn(
          'meet-controls-bar fixed inset-x-2 z-30 flex flex-col items-center overflow-hidden rounded-3xl',
          'border border-[var(--meet-border-subtle)] bg-[var(--meet-chrome)] backdrop-blur-xl',
          'shadow-[var(--meet-shadow),var(--meet-shadow-inset)]',
        )}
        style={{ bottom: 'calc(12px + env(safe-area-inset-bottom, 0px))' }}
      >
        <button
          type="button"
          aria-label="Room options"
          aria-expanded={expanded}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          className="flex h-12 w-full shrink-0 cursor-grab items-center justify-center border-none bg-transparent active:cursor-grabbing"
        >
          <span className={HANDLE_PILL_CLASS} />
        </button>

        <MeetingOptionsPanel rows={rows} expanded={expanded} onSelect={selectRow} />

        <MeetingCallControlsRow
          {...controls}
          chatOpen={chatOpen}
          onToggleChat={() => {
            collapse()
            onToggleChat()
          }}
          onLeave={() => {
            collapse()
            onLeave()
          }}
        />
      </div>
    </>
  )
}
