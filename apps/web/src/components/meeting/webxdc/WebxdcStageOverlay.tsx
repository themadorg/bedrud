import { useRoomContext } from '@livekit/components-react'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { meetStageShellClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { cn } from '@/lib/utils'
import { WebxdcFrame } from './WebxdcFrame'
import { useOptionalWebxdcWatch } from './webxdc-watch-context'

/** Full-stage iframe when a WebXDC app is shared with the room. */
export function WebxdcStageOverlay() {
  const layout = useMeetingUILayout()
  const { roomId, getParticipantDisplayName, getParticipantAvatarUrl } = useMeetingRoomContext()
  const room = useRoomContext()
  const watch = useOptionalWebxdcWatch()
  if (!watch) return null
  const { session, isHost, busy, stopShare, leaveLocal } = watch

  if (!session) {
    if (busy) {
      return (
        <div className={cn(meetStageShellClass(layout, 'p-3 max-sm:p-2'))}>
          <div className="flex min-h-0 flex-1 items-center justify-center rounded-xl border border-white/[0.08] bg-[#030308]/95 text-sm text-white/60 shadow-2xl backdrop-blur-md">
            Opening mini-app…
          </div>
        </div>
      )
    }
    return null
  }

  const lp = room.localParticipant
  const selfName =
    session.selfName?.trim() ||
    getParticipantDisplayName(lp) ||
    lp.name ||
    lp.identity ||
    'You'
  const selfAvatarUrl = getParticipantAvatarUrl(lp)

  return (
    <div className={cn(meetStageShellClass(layout, 'p-3 max-sm:p-1.5'))}>
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-white/[0.08] bg-[#12121f] shadow-2xl">
        <WebxdcFrame
          roomId={roomId}
          instanceId={session.instanceId}
          iframeUrl={session.iframeUrl}
          iframeOrigin={session.iframeOrigin}
          appName={session.name}
          selfName={selfName}
          selfAddr={session.selfAddr}
          selfAvatarUrl={selfAvatarUrl}
          userId={lp.identity}
          onClose={() => {
            if (isHost) void stopShare()
            else leaveLocal()
          }}
        />
      </div>
    </div>
  )
}
