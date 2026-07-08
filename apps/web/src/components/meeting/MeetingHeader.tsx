// TODO oncoming feature
import { useConnectionState, useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useRef, useState } from 'react'

import { useRoomPublishReady } from '#/lib/livekit-publish'
import { liveKitTransportModeLabel, useLiveKitTransportMode } from '#/lib/livekit-transport-type'
import { cn } from '#/lib/utils'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'

interface MeetingHeaderProps {
  meetId: string
  /** Epoch ms when the LiveKit session was created on the server. 0/undefined means this user is the first joiner — fall back to local connect time. */
  sessionStartedAt?: number
  infoOpen?: boolean
  onToggleInfo?: () => void
}

function formatElapsed(ms: number): string {
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function formatClock(now: Date): string {
  return now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

/** Top-of-screen header showing connection status, room name, and elapsed time. */
export function MeetingHeader({ meetId, infoOpen = false, onToggleInfo }: MeetingHeaderProps) {
  const room = useRoomContext()
  const state = useConnectionState()
  const layout = useMeetingUILayout()
  const isConnected = state === ConnectionState.Connected
  const transportMode = useLiveKitTransportMode(room, isConnected)
  const chatReady = useRoomPublishReady(room, isConnected)
  const isP2P = isConnected && transportMode === 'p2p'
  const isRelay = isConnected && transportMode === 'relay'
  const statusLabel = isConnected
    ? chatReady
      ? liveKitTransportModeLabel(transportMode)
      : isP2P
        ? 'P2P · chat'
        : 'Connecting chat'
    : String(state)
  const joinTimeRef = useRef<number | null>(null)
  const [elapsed, setElapsed] = useState('00:00')
  const [showClock, setShowClock] = useState(false)
  const [clockTime, setClockTime] = useState(() => formatClock(new Date()))

  // Set join timestamp when first connected
  useEffect(() => {
    if (isConnected && joinTimeRef.current === null) {
      joinTimeRef.current = Date.now()
    }
    if (!isConnected) {
      joinTimeRef.current = null
      setElapsed('00:00')
    }
  }, [isConnected])

  // Tick elapsed every second while connected
  useEffect(() => {
    if (!isConnected) return
    const id = setInterval(() => {
      if (joinTimeRef.current !== null) {
        setElapsed(formatElapsed(Date.now() - joinTimeRef.current))
      }
    }, 1000)
    return () => clearInterval(id)
  }, [isConnected])

  useEffect(() => {
    if (!showClock) return
    const tick = () => setClockTime(formatClock(new Date()))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [showClock])

  return (
    <header
      className={cn(
        'absolute top-0 left-0 z-20 flex items-center justify-center px-4 pointer-events-none h-[calc(56px+env(safe-area-inset-top))] pt-[env(safe-area-inset-top)] transition-[right] duration-200',
        meetRightInsetClass(layout),
      )}
    >
      <div className="flex items-center gap-2.5 pointer-events-auto">
        <button
          type="button"
          onClick={onToggleInfo}
          disabled={!onToggleInfo}
          className={cn(
            'flex items-center gap-[5px] rounded-[7px] px-[9px] py-[3px] border transition-all duration-150',
            onToggleInfo && 'cursor-pointer hover:brightness-110',
            !onToggleInfo && 'cursor-default',
            infoOpen
              ? 'border-[color-mix(in_oklab,var(--primary)_40%,transparent)] bg-[color-mix(in_oklab,var(--primary)_25%,transparent)]'
              : isConnected && !chatReady
                ? 'border-amber-500/30 bg-amber-500/10'
                : isP2P
                  ? 'border-teal-500/35 bg-teal-500/10'
                  : isRelay
                    ? 'border-sky-500/35 bg-sky-500/10'
                    : isConnected
                      ? 'border-emerald-500/30 bg-emerald-500/10'
                      : 'border-amber-500/30 bg-amber-500/10',
          )}
          aria-label={infoOpen ? 'Close room info' : 'Show room info'}
          aria-pressed={infoOpen}
        >
          {isConnected && chatReady ? (
            <svg
              width="10"
              height="10"
              viewBox="0 0 10 10"
              aria-hidden
              className={cn(
                'shrink-0',
                infoOpen
                  ? 'text-accent-400'
                  : isP2P
                    ? 'text-teal-600 dark:text-teal-400'
                    : isRelay
                      ? 'text-sky-600 dark:text-sky-400'
                      : 'text-emerald-600 dark:text-emerald-400',
              )}
            >
              <circle cx="5" cy="5" r="3.5" fill="currentColor" />
            </svg>
          ) : isConnected ? (
            <svg
              className="meet-connecting text-amber-600 dark:text-amber-400"
              width="10"
              height="10"
              viewBox="0 0 10 10"
              role="img"
              aria-label="Opening chat channel"
            >
              <circle cx="5" cy="5" r="4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="6 4" />
            </svg>
          ) : (
            <svg
              className="meet-connecting text-amber-600 dark:text-amber-400"
              width="10"
              height="10"
              viewBox="0 0 10 10"
              role="img"
              aria-label="Connecting"
            >
              <circle cx="5" cy="5" r="4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="6 4" />
            </svg>
          )}
          <span
            className={cn(
              'text-[11px] font-medium',
              infoOpen
                ? 'text-accent-400'
                : isConnected && !chatReady
                  ? 'text-amber-600 dark:text-amber-400'
                  : isP2P
                    ? 'text-teal-600 dark:text-teal-400'
                    : isRelay
                      ? 'text-sky-600 dark:text-sky-400'
                      : isConnected
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-amber-600 dark:text-amber-400',
            )}
            title={
              isConnected && chatReady
                ? 'Audio, video, and chat share one WebRTC connection'
                : isConnected
                  ? 'Waiting for chat data channel on the peer connection'
                  : undefined
            }
          >
            {statusLabel}
          </span>
        </button>
        <span className="text-[13px] text-[var(--meet-fg-muted)]">·</span>
        <span className="text-xs font-mono text-[var(--meet-fg-muted)]">{meetId}</span>
        <span className="text-[13px] text-[var(--meet-fg-muted)]">·</span>
        <button
          type="button"
          onClick={() => setShowClock((v) => !v)}
          className={cn(
            'cursor-pointer border-none bg-transparent p-0 text-[11px] font-mono transition-colors hover:text-[var(--meet-fg-strong)]',
            showClock ? 'text-[var(--meet-fg-strong)]' : 'text-[var(--meet-fg-muted)]',
          )}
          aria-label={showClock ? 'Show meeting duration' : 'Show current time'}
          title={showClock ? 'Meeting duration' : 'Current time'}
        >
          {showClock ? clockTime : elapsed}
        </button>

        {/* TODO oncoming feature — recording badge removed */}
      </div>
    </header>
  )
}
