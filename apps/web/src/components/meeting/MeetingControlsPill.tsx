import { useState } from 'react'
import { MeetingCallControlsRow } from '@/components/meeting/MeetingCallControlsRow'
import { MeetingOptionsSheet } from '@/components/meeting/MeetingOptionsSheet'
import type { MeetingOptionRow, MeetingOptionRowId } from '@/components/meeting/meetingOptionRows'
import { HANDLE_PILL_CLASS } from '@/components/ui/BedrudSheetHandle'
import { cn } from '@/lib/utils'

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
 * The phone in-call controls, and the handle that opens the room options in their own sheet.
 *
 * The pill carries controls only. Scrim, drag-down and Escape for the options all come from
 * `BedrudSheet`, as they do for every other sheet in the app.
 */
export function MeetingControlsPill({ rows, onSelectOption, ...controls }: MeetingControlsPillProps) {
  const [optionsOpen, setOptionsOpen] = useState(false)

  return (
    <>
      <MeetingOptionsSheet
        open={optionsOpen}
        onClose={() => setOptionsOpen(false)}
        rows={rows}
        onSelect={onSelectOption}
      />

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
          aria-label="More options"
          aria-haspopup="dialog"
          onClick={() => setOptionsOpen(true)}
          className="flex h-12 w-full shrink-0 cursor-pointer items-center justify-center border-none bg-transparent"
        >
          <span className={HANDLE_PILL_CLASS} />
        </button>

        <MeetingCallControlsRow {...controls} />
      </div>
    </>
  )
}
