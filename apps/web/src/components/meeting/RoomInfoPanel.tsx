import { useConnectionState, useParticipants, useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { useRoomPublishReady } from '#/lib/livekit-publish'
import { liveKitTransportModeLabel, useLiveKitTransportMode } from '#/lib/livekit-transport-type'
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
import {
  latencyColorClass,
  packetLossColorClass,
  qualityColorClass,
  useMeetingConnectionStats,
} from './meetingConnectionStats'

interface RoomInfoPanelProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  roomId: string
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

export function RoomInfoPanel({ open, onOpenChange, roomId }: RoomInfoPanelProps) {
  const room = useRoomContext()
  const participants = useParticipants()
  const connectionState = useConnectionState()
  const noiseMode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const connected = connectionState === ConnectionState.Connected
  const { stats, loading } = useMeetingConnectionStats(room.localParticipant, open)
  const transportMode = useLiveKitTransportMode(room, connected)
  const chatReady = useRoomPublishReady(room, connected)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="meet-dialog max-w-[min(92vw,360px)] gap-0 p-0 shadow-2xl">
        <DialogHeader className="border-b border-[var(--meet-border)] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-[var(--meet-fg)]">Room info</DialogTitle>
          <DialogDescription className="text-[var(--meet-fg-muted)]">
            Connection details for this meeting.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 px-4 py-4">
          <div>
            <p className="text-[10px] font-medium uppercase tracking-wider text-[var(--meet-fg-muted)]">Room ID</p>
            <p className="mt-0.5 break-all font-mono text-sm text-[var(--meet-fg-strong)]">{roomId}</p>
          </div>

          <div className="rounded-lg border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-3">
            <p className="mb-2.5 text-[10px] font-medium uppercase tracking-wider text-[var(--meet-fg-muted)]">
              Connection
            </p>
            <div className="flex flex-col gap-2">
              <InfoRow label="Status">
                <span className={connected ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}>
                  {connectionStateLabel(connectionState)}
                </span>
              </InfoRow>
              {connected && transportMode !== 'unknown' && (
                <InfoRow label="Transport">
                  <span
                    className={cn(
                      transportMode === 'p2p'
                        ? 'text-teal-600 dark:text-teal-400'
                        : 'text-sky-600 dark:text-sky-400',
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
                      chatReady
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-amber-600 dark:text-amber-400',
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
                  <span className={cn('font-mono tabular-nums', latencyColorClass(stats.jitter))}>
                    {stats.jitter} ms
                  </span>
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
        </div>

        <DialogFooter className="border-t border-[var(--meet-border)] px-4 py-3 sm:justify-end">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            className="text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control-hover)] hover:text-[var(--meet-fg)]"
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
