import { useConnectionState, useParticipants, useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { meetingPanelScopeClass, settingsDialogScrollClass } from '#/components/settings/settingsPanelTone'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useRoomPublishReady } from '#/lib/livekit-publish'
import { liveKitTransportModeLabel, useLiveKitTransportMode } from '#/lib/livekit-transport-type'
import { MeetingElevatedLeftDock } from '@/components/meeting/MeetingElevatedLeftDock'
import {
  MeetingElevatedMeetingSubheader,
  MeetingElevatedPanelHeader,
  useMeetingElapsedClock,
} from '@/components/meeting/MeetingElevatedPanelChrome'
import { useMeetingExpandChromeHandlers } from '@/components/meeting/meeting-expand-chrome-context'
import { WebxdcPanel } from '@/components/meeting/webxdc/WebxdcPanel'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import {
  latencyColorClass,
  packetLossColorClass,
  qualityColorClass,
  useMeetingConnectionStats,
} from './meetingConnectionStats'

interface RoomInfoContentProps {
  roomId: string
  /** When false, pause stats polling (e.g. panel not visible). */
  active?: boolean
}

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="shrink-0 text-[11px] text-[var(--meet-fg-muted)]">{label}</span>
      <span className="min-w-0 text-end text-[11px] font-medium text-[var(--meet-fg-strong)]">{children}</span>
    </div>
  )
}

function connectionStateLabel(state: ConnectionState) {
  if (state === ConnectionState.Connected) return 'Connected'
  if (state === ConnectionState.Connecting) return 'Connecting'
  if (state === ConnectionState.Reconnecting) return 'Reconnecting'
  if (state === ConnectionState.Disconnected) return 'Disconnected'
  return state
}

function RoomInfoSheetHeader({ onClose }: { onClose: () => void }) {
  return (
    <>
      <MeetingElevatedPanelHeader title="Room info" onClose={onClose} closeLabel="Close room info" />
      <MeetingElevatedMeetingSubheader />
    </>
  )
}

/** Desktop dialog header with the same elapsed / clock strip. */
function RoomInfoDesktopHeader() {
  const { elapsed, clock } = useMeetingElapsedClock()

  return (
    <DialogHeader className="shrink-0 space-y-0 border-b border-[var(--meet-border)] px-4 py-3">
      <div className="flex items-start justify-between gap-3 pe-8">
        <div className="min-w-0">
          <DialogTitle className="text-[15px] font-semibold text-[var(--meet-fg)]">Room info</DialogTitle>
          <DialogDescription className="sr-only">Connection details for this meeting.</DialogDescription>
        </div>
      </div>
      <div className="mt-3 flex items-end justify-between gap-4">
        <div className="min-w-0">
          <p className="text-[11px] font-medium uppercase tracking-wider text-[var(--meet-fg-muted)]">In meeting</p>
          <p className="mt-1 text-4xl font-bold leading-none tabular-nums tracking-tight text-[var(--meet-fg-strong)]">
            <span className="sr-only">Meeting duration </span>
            {elapsed}
          </p>
        </div>
        <div className="shrink-0 self-end pb-0.5 text-end">
          <p className="text-[10px] font-medium uppercase tracking-wider text-[var(--meet-fg-muted)]">Local time</p>
          <p className="mt-1 text-[13px] tabular-nums text-[var(--meet-fg-muted)]">
            <span className="sr-only">Current time </span>
            {clock}
          </p>
        </div>
      </div>
    </DialogHeader>
  )
}

/** Room connection details body — used by dialog, elevated sheet, and mobile More. */
export function RoomInfoContent({ roomId, active = true }: RoomInfoContentProps) {
  const room = useRoomContext()
  const participants = useParticipants()
  const connectionState = useConnectionState()
  const noiseMode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const connected = connectionState === ConnectionState.Connected
  const { stats, loading } = useMeetingConnectionStats(room.localParticipant, active)
  const transportMode = useLiveKitTransportMode(room, connected)
  const chatReady = useRoomPublishReady(room, connected)
  const webxdcUserEnabled = useExperimentalPreferencesStore((s) => s.webxdcEnabled)

  return (
    <div className={cn('flex min-h-0 flex-1 flex-col', meetingPanelScopeClass)}>
      <div className={cn('min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-4', settingsDialogScrollClass)}>
        <div className="rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-3.5">
          <p className="mb-2.5 text-[10px] font-medium uppercase tracking-wider text-[var(--meet-fg-muted)]">
            Connection
          </p>
          <div className="flex flex-col gap-2">
            <InfoRow label="Status">
              <span
                className={connected ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}
              >
                {connectionStateLabel(connectionState)}
              </span>
            </InfoRow>
            {connected && transportMode !== 'unknown' && (
              <InfoRow label="Transport">
                <span
                  className={cn(
                    transportMode === 'p2p' ? 'text-teal-600 dark:text-teal-400' : 'text-sky-600 dark:text-sky-400',
                  )}
                >
                  {liveKitTransportModeLabel(transportMode)}
                </span>
              </InfoRow>
            )}
            {connected && (
              <InfoRow label="Chat channel">
                <span
                  className={cn(
                    chatReady ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400',
                  )}
                >
                  {chatReady ? 'Ready (send + receive)' : 'Opening send/receive channels…'}
                </span>
              </InfoRow>
            )}
            <InfoRow label="Participants">{participants.length}</InfoRow>
            <InfoRow label="Quality">
              <span className={cn('capitalize', qualityColorClass(stats?.quality ?? 'unknown'))}>
                {stats?.quality ?? '—'}
              </span>
            </InfoRow>
            {stats?.codec && <InfoRow label="Codec">{stats.codec}</InfoRow>}
            {stats?.ping != null && (
              <InfoRow label="RTT">
                <span className={cn('font-mono tabular-nums', latencyColorClass(stats.ping))}>{stats.ping} ms</span>
              </InfoRow>
            )}
            {stats?.jitter != null && (
              <InfoRow label="Jitter">
                <span className={cn('font-mono tabular-nums', latencyColorClass(stats.jitter))}>{stats.jitter} ms</span>
              </InfoRow>
            )}
            {stats?.packetLoss != null && (
              <InfoRow label="Packet loss">
                <span className={cn('font-mono tabular-nums', packetLossColorClass(stats.packetLoss))}>
                  {stats.packetLoss}%
                </span>
              </InfoRow>
            )}
            {stats?.bandwidth != null && (
              <InfoRow label="Bandwidth">
                <span className="font-mono tabular-nums">{stats.bandwidth} kbps</span>
              </InfoRow>
            )}
            <InfoRow label="Noise suppression">
              <span className="capitalize">{noiseMode}</span>
            </InfoRow>
            {loading && !stats && (
              <div className="flex items-center gap-1.5 pt-1 text-[10px] text-[var(--meet-fg-muted)]">
                <Loader2 className="h-3 w-3 animate-spin" />
                Collecting stats…
              </div>
            )}
            {loading && stats && (
              <div className="flex items-center gap-1.5 pt-1 text-[10px] text-[var(--meet-fg-subtle)]">
                <Loader2 className="h-3 w-3 animate-spin" />
                Updating…
              </div>
            )}
            {!loading && !stats && connected && (
              <p className="pt-1 text-[10px] text-[var(--meet-fg-muted)]">
                Enable microphone or camera to collect network stats.
              </p>
            )}
          </div>
        </div>

        {webxdcUserEnabled && (
          <div className="overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)]">
            <WebxdcPanel
              roomId={roomId}
              selfName={room.localParticipant.name || room.localParticipant.identity}
              userId={room.localParticipant.identity}
            />
          </div>
        )}
      </div>
    </div>
  )
}

interface RoomInfoPanelProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  roomId: string
  /**
   * When true (WebXDC expand rail): left dock like settings —
   * does not collapse the mini-app; clears the left rail.
   */
  elevated?: boolean
}

export function RoomInfoPanel({ open, onOpenChange, roomId, elevated = false }: RoomInfoPanelProps) {
  const { closeInfo: closeElevatedInfo } = useMeetingExpandChromeHandlers()
  const close = () => onOpenChange(false)
  const handleClose = elevated ? closeElevatedInfo : close

  const sheetBody = (
    <>
      <RoomInfoSheetHeader onClose={handleClose} />
      <RoomInfoContent roomId={roomId} active={open} />
    </>
  )

  // Elevated: shared left dock (same shell/size as chat + settings).
  if (elevated) {
    if (!open) return null
    return (
      <MeetingElevatedLeftDock label="Room info" marker="info">
        {sheetBody}
      </MeetingElevatedLeftDock>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          'meet-dialog flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
          // Desktop: compact card (settings uses larger for tabs)
          'sm:max-h-[min(90vh,720px)] sm:w-[min(360px,calc(var(--app-width,100svw)-2rem))] sm:max-w-[min(360px,calc(var(--app-width,100svw)-2rem))]',
          // Mobile full-screen — same shell as settings
          'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[var(--app-height,100svh)] max-sm:max-h-[var(--app-height,100svh)] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0',
          // Hide default Dialog X on mobile (we render nav chrome ourselves).
          'max-sm:[&>button.absolute]:hidden',
        )}
      >
        {/* Mobile: settings-style sheet chrome */}
        <div className="flex min-h-0 flex-1 flex-col sm:hidden">{sheetBody}</div>

        {/* Desktop: title header + body */}
        <div className="hidden min-h-0 flex-1 flex-col sm:flex">
          <RoomInfoDesktopHeader />
          <RoomInfoContent roomId={roomId} active={open} />
        </div>
      </DialogContent>
    </Dialog>
  )
}
