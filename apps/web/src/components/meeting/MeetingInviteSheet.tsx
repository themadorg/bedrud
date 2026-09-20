import { useParticipants } from '@livekit/components-react'
import { Users } from 'lucide-react'
import { useCallback, useState } from 'react'
import { BedrudSheet, SHEET_SNAP_POINTS } from '@/components/ui/BedrudSheet'
import { MeetingInviteGrid } from './MeetingInviteGrid'
import { MeetingInviteTargets } from './MeetingInviteTargets'
import { meetingLink } from './meetingLink'

interface MeetingInviteSheetProps {
  open: boolean
  onClose: () => void
  adminId: string
}

/**
 * The phone's roster and invite in one bottom sheet over the live call, mirroring Android's
 * `MeetingInviteSheet`. It is a `BedrudSheet` and takes nothing from it but its children: the snap
 * points, corner, handle, backdrop and gutter are the sheet's, and restating any of them here would
 * be a second opinion about what a sheet looks like.
 */
export function MeetingInviteSheet({ open, onClose, adminId }: MeetingInviteSheetProps) {
  const participants = useParticipants()
  const [snapPoint, setSnapPoint] = useState<number | string | null>(SHEET_SNAP_POINTS[0])
  const expandSheet = useCallback(() => setSnapPoint(SHEET_SNAP_POINTS[SHEET_SNAP_POINTS.length - 1]), [])

  return (
    <BedrudSheet
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose()
      }}
      label="Participants and invite"
      activeSnapPoint={snapPoint}
      onSnapPointChange={setSnapPoint}
      dataMarkers={{ 'data-invite-sheet': 'true' }}
    >
      {/*
        The body scrolls. The grid is capped, but the QR and a full roster together outgrow the half
        height, and a sheet whose lower half is simply unreachable is worse than one that scrolls.
      */}
      <div className="meet-scroll flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-4">
        <div className="flex shrink-0 items-center gap-[7px]">
          <Users size={14} className="text-[var(--meet-btn-muted-fg)]" />
          <span className="text-[13px] font-semibold text-[var(--meet-fg-strong)]">Participants</span>
          <span className="rounded-md border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] px-[6px] py-px text-[11px] font-semibold text-[var(--meet-btn-muted-fg)]">
            {participants.length}
          </span>
        </div>

        <MeetingInviteGrid participants={participants} adminId={adminId} />

        <div className="h-px shrink-0 bg-[var(--meet-border-subtle)]" />

        <MeetingInviteTargets roomLink={meetingLink()} onExpandSheet={expandSheet} />
      </div>
    </BedrudSheet>
  )
}
