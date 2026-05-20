// TODO oncoming feature
import { useConnectionState } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useRef, useState } from 'react'

import { cn } from '#/lib/utils'

interface MeetingHeaderProps {
  meetId: string
  /** Epoch ms when the LiveKit session was created on the server. 0/undefined means this user is the first joiner — fall back to local connect time. */
  sessionStartedAt?: number
}

function formatElapsed(ms: number): string {
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

/** Top-of-screen header showing connection status, room name, and elapsed time. */
export function MeetingHeader({ meetId }: MeetingHeaderProps) {
  const state = useConnectionState()
  const isConnected = state === ConnectionState.Connected
  const joinTimeRef = useRef<number | null>(null)
  const [elapsed, setElapsed] = useState('00:00')

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

  return (
    <header className="absolute left-0 right-0 top-0 z-20 flex items-center justify-center px-4 pointer-events-none h-[calc(56px+env(safe-area-inset-top))] pt-[env(safe-area-inset-top)]">
      <div className="flex items-center gap-2.5 pointer-events-auto">
        {/* Connected / Connecting badge */}
        <div
          className="flex items-center gap-[5px] rounded-[7px] px-[9px] py-[3px]"
          style={{
            background: isConnected ? 'rgba(34,197,94,0.12)' : 'rgba(234,179,8,0.12)',
            border: `1px solid ${isConnected ? 'rgba(34,197,94,0.25)' : 'rgba(234,179,8,0.25)'}`,
          }}
        >
          {isConnected ? (
            <span className="inline-block w-1.5 h-1.5 rounded-full bg-green-500" />
          ) : (
            <svg
              className="meet-connecting"
              width="10"
              height="10"
              viewBox="0 0 10 10"
              role="img"
              aria-label="Connecting"
            >
              <circle cx="5" cy="5" r="4" fill="none" stroke="#eab308" strokeWidth="1.5" strokeDasharray="6 4" />
            </svg>
          )}
          <span className={cn('text-[11px] font-medium', isConnected ? 'text-green-300' : 'text-yellow-300')}>
            {isConnected ? 'Connected' : state}
          </span>
        </div>
        <span className="text-white/50 text-[13px]">·</span>
        <span className="text-white/55 text-xs font-mono">{meetId}</span>
        <span className="text-white/50 text-[13px]">·</span>
        <span className={cn('text-[11px] font-mono', isConnected ? 'text-white/55' : 'text-white/50')}>{elapsed}</span>

        {/* TODO oncoming feature — recording badge removed */}
      </div>
    </header>
  )
}
