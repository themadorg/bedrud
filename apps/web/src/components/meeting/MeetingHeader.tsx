// TODO oncoming feature
import { useConnectionState, useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useState } from 'react'

import { useRoomPublishReady } from '#/lib/livekit-publish'
import { liveKitTransportModeLabel, useLiveKitTransportMode } from '#/lib/livekit-transport-type'
import { cn } from '#/lib/utils'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import {
  formatMeetingClock,
  formatMeetingElapsed,
  getMeetingJoinedAtMs,
  noteMeetingConnected,
  noteMeetingDisconnected,
} from '@/components/meeting/meetingSessionTime'

interface MeetingHeaderProps {
  meetId: string
  /** Epoch ms when the LiveKit session was created on the server. 0/undefined means this user is the first joiner — fall back to local connect time. */
  sessionStartedAt?: number
  infoOpen?: boolean
  onToggleInfo?: () => void
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
  const [elapsed, setElapsed] = useState('00:00')
  const [showClock, setShowClock] = useState(false)
  const [clockTime, setClockTime] = useState(() => formatMeetingClock(new Date()))

  // Set shared join timestamp when first connected
  useEffect(() => {
    if (isConnected) {
      noteMeetingConnected()
    } else {
      noteMeetingDisconnected()
      setElapsed('00:00')
    }
  }, [isConnected])

  // Tick elapsed every second while connected
  useEffect(() => {
    if (!isConnected) return
    const id = setInterval(() => {
      const joined = getMeetingJoinedAtMs()
      if (joined != null) {
        setElapsed(formatMeetingElapsed(Date.now() - joined))
      }
    }, 1000)
    return () => clearInterval(id)
  }, [isConnected])

  useEffect(() => {
    if (!showClock) return
    const tick = () => setClockTime(formatMeetingClock(new Date()))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [showClock])

  return (
    <header
      className={cn(
        // Full height includes safe-area; content sits in the 56px band below the notch.
        'absolute top-0 left-0 z-20 flex flex-col justify-end pointer-events-none h-[calc(56px+env(safe-area-inset-top,0px))] transition-[right] duration-200',
        meetRightInsetClass(layout),
      )}
    >
      {/* 56px row — mobile: left; desktop: centered. */}
      <div
        className={cn(
          'flex h-14 min-w-0 items-center gap-2 pointer-events-auto leading-none',
          // Mobile: left-aligned, clear of top-right icons.
          'justify-start ps-3 max-sm:pe-[calc(100px+env(safe-area-inset-right,0px))]',
          // Desktop: centered title cluster.
          'sm:justify-center sm:px-4',
        )}
      >
        {/* Connection status — desktop only (mobile: Room info in ⋯ menu) */}
        <button
          type="button"
          onClick={onToggleInfo}
          disabled={!onToggleInfo}
          className={cn(
            'hidden sm:flex items-center gap-[5px] rounded-[7px] px-[9px] py-[3px] border transition-all duration-150',
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
              role="presentation"
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
        <span className="hidden text-[13px] leading-none text-[var(--meet-fg-muted)] sm:inline">·</span>
        <span className="truncate text-xs font-mono leading-none text-[var(--meet-fg-muted)]">{meetId}</span>
        <span className="text-[13px] leading-none text-[var(--meet-fg-muted)]">·</span>
        <button
          type="button"
          onClick={() => setShowClock((v) => !v)}
          className={cn(
            'flex h-9 shrink-0 cursor-pointer items-center border-none bg-transparent p-0 text-[11px] font-mono leading-none transition-colors hover:text-[var(--meet-fg-strong)]',
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
