import { useLocalParticipant } from '@livekit/components-react'
import { Info, LogOut, MessageSquare, Mic, MicOff, Settings, Video, VideoOff } from 'lucide-react'
import { useState } from 'react'
import { RailDeafenIcon } from '@/components/meeting/DeafenHeadphonesIcon'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { useMeetingExpandChromeHandlers } from '@/components/meeting/meeting-expand-chrome-context'
import {
  type MeetingChromeExpandSource,
  type MeetingChromeOpenDetail,
  type MeetingChromePanel,
  requestOpenMeetingSettings,
} from '@/components/meeting/meetingChromeEvents'
import { RoomAccessBadge } from '@/components/meeting/RoomAccessBadge'
import { RoomAccessDialog } from '@/components/meeting/RoomAccessDialog'
import { useMeetingMicKeyboard } from '@/components/meeting/useMeetingMicKeyboard'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type Props = {
  activePanel: MeetingChromePanel
  chromeDetail: MeetingChromeOpenDetail & { source: MeetingChromeExpandSource }
  onLeave?: () => void
  leaveLabel?: string
}

const iconBtn = 'h-9 w-9 shrink-0'
const railActiveClass =
  'bg-primary/15 text-primary shadow-[0_0_0_1px_color-mix(in_oklab,var(--primary)_45%,transparent),0_0_12px_color-mix(in_oklab,var(--primary)_35%,transparent)] hover:bg-primary/20 hover:text-primary'

/** Left rail for expanded stage chrome (WebXDC, screen share, etc.). */
export function MeetingExpandLeftRail({ activePanel, chromeDetail, onLeave, leaveLabel = 'Leave' }: Props) {
  const { localParticipant, isMicrophoneEnabled: micEnabled, isCameraEnabled: camEnabled } = useLocalParticipant()
  const { isSelfDeafened, toggleSelfDeafen } = useMeetingRoomContext()
  const { micUiEnabled, micTip, toggleMic } = useMeetingMicKeyboard(localParticipant, isSelfDeafened, micEnabled)
  const micOff = isSelfDeafened || !micUiEnabled
  const [accessDialogOpen, setAccessDialogOpen] = useState(false)
  const { openChat, openInfo } = useMeetingExpandChromeHandlers()

  return (
    <>
      <aside
        className="flex w-12 min-h-0 shrink-0 flex-col items-center border-r border-border bg-background px-1 py-2"
        aria-label="Meeting actions"
      >
        <div className="flex w-full shrink-0 flex-col items-center gap-1.5">
          <div
            className="mb-1 flex h-9 w-9 items-center justify-center overflow-hidden rounded-md border border-border bg-muted/40"
            title="Bedrud"
            aria-hidden
          >
            <img src="/favicon.svg" alt="" className="h-5 w-5 object-contain" />
          </div>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(iconBtn, activePanel === 'chat' && railActiveClass)}
            aria-label={activePanel === 'chat' ? 'Close chat' : 'Open chat'}
            aria-pressed={activePanel === 'chat'}
            onClick={() => openChat()}
          >
            <MessageSquare className="h-4 w-4" />
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(iconBtn, activePanel === 'settings' && railActiveClass)}
            aria-label={activePanel === 'settings' ? 'Close settings' : 'Open settings'}
            aria-pressed={activePanel === 'settings'}
            onClick={() => requestOpenMeetingSettings(chromeDetail)}
          >
            <Settings className="h-4 w-4" />
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(iconBtn, activePanel === 'info' && railActiveClass)}
            aria-label={activePanel === 'info' ? 'Close room info' : 'Room info'}
            aria-pressed={activePanel === 'info'}
            onClick={() => openInfo()}
          >
            <Info className="h-4 w-4" />
          </Button>
        </div>

        <div className="min-h-2 flex-1" aria-hidden />

        <RoomAccessBadge variant="rail" onOpen={() => setAccessDialogOpen(true)} />

        <div className="mx-1.5 mt-1.5 h-px shrink-0 self-stretch bg-border" aria-hidden />

        <div className="flex w-full shrink-0 flex-col items-center gap-1.5 pt-2">
          <div className="flex w-full justify-center">
            <DeviceSelector
              kind="audioinput"
              menuSide="right"
              chevronDirection="right"
              elevated
              triggerClassName="h-5 w-6 text-muted-foreground hover:text-foreground"
            />
          </div>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              iconBtn,
              micOff && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
            )}
            aria-label={micOff ? (isSelfDeafened ? 'Undeafen' : 'Unmute microphone') : micTip || 'Mute microphone'}
            aria-pressed={micOff}
            title={micOff ? (isSelfDeafened ? 'Undeafen' : 'Unmute') : 'Mute'}
            onClick={() => {
              if (isSelfDeafened) {
                toggleSelfDeafen()
                return
              }
              toggleMic()
            }}
          >
            {micOff ? <MicOff className="h-4 w-4" /> : <Mic className="h-4 w-4" />}
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              iconBtn,
              isSelfDeafened && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
            )}
            aria-label={isSelfDeafened ? 'Undeafen' : 'Deafen'}
            aria-pressed={isSelfDeafened}
            title={isSelfDeafened ? 'Undeafen' : 'Deafen'}
            onClick={() => toggleSelfDeafen()}
          >
            <RailDeafenIcon off={isSelfDeafened} />
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              iconBtn,
              !camEnabled && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
            )}
            aria-label={camEnabled ? 'Disable camera' : 'Enable camera'}
            aria-pressed={!camEnabled}
            title={camEnabled ? 'Camera off' : 'Camera on'}
            onClick={() => {
              void localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})
            }}
          >
            {camEnabled ? <Video className="h-4 w-4" /> : <VideoOff className="h-4 w-4" />}
          </Button>

          {onLeave ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className={cn(iconBtn, 'text-destructive hover:bg-destructive/10 hover:text-destructive')}
              aria-label={leaveLabel}
              title={leaveLabel}
              onClick={onLeave}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          ) : null}
        </div>
      </aside>
      <RoomAccessDialog open={accessDialogOpen} onOpenChange={setAccessDialogOpen} aboveElevatedDock />
    </>
  )
}
