import { useRoomContext } from '@livekit/components-react'
import { LogOut, PhoneOff } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import { ControlsBar, type ControlsBarMoreExtras } from '@/components/meeting/ControlsBar'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

interface MeetingControlsProps {
  onNavigate: () => void
  /** Hide the floating controls bar on mobile (e.g. full-screen chat modal). */
  hideOnMobile?: boolean
  /** Merged into the single bottom ⋯ menu on mobile. */
  moreExtras?: ControlsBarMoreExtras
}

/** Renders the bottom controls bar and the end-meeting dialog for creators. */
export function MeetingControls({ onNavigate, hideOnMobile = false, moreExtras }: MeetingControlsProps) {
  const { isCreator, roomId } = useMeetingRoomContext()
  const room = useRoomContext()
  const [endDialogOpen, setEndDialogOpen] = useState(false)
  const [isEnding, setIsEnding] = useState(false)

  function handleLeaveRequest() {
    if (isCreator) setEndDialogOpen(true)
    else onNavigate()
  }

  async function handleEndMeeting() {
    setIsEnding(true)
    try {
      await api.delete(`/api/room/${roomId}`)
    } catch {
      /* already closed */
    }
    room.disconnect()
    onNavigate()
  }

  function handleLeaveMeeting() {
    room.disconnect()
    onNavigate()
  }

  return (
    <>
      <div className={cn(hideOnMobile && 'max-lg:hidden')}>
        <ControlsBar onLeave={handleLeaveRequest} moreExtras={moreExtras} />
      </div>
      <Dialog open={endDialogOpen} onOpenChange={setEndDialogOpen}>
        <DialogContent className="meet-dialog sm:max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-white">Leave meeting?</DialogTitle>
            <DialogDescription className="text-white/50">
              You created this meeting. End it for everyone, or just slip out.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex-col gap-2 sm:flex-col">
            <Button
              className="w-full gap-2 bg-red-500/85 text-white border-none hover:bg-red-500/90"
              onClick={handleEndMeeting}
              disabled={isEnding}
            >
              <PhoneOff className="h-4 w-4" />
              End Meeting for Everyone
            </Button>
            <Button
              variant="outline"
              className="w-full gap-2 bg-white/[0.05] border-white/10 text-white/80 hover:bg-white/[0.1] hover:text-white"
              onClick={handleLeaveMeeting}
            >
              <LogOut className="h-4 w-4" />
              Leave Meeting
            </Button>
            <Button
              variant="ghost"
              className="w-full text-white/50 hover:text-white/70"
              onClick={() => setEndDialogOpen(false)}
            >
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
