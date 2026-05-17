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
  const secs = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  const s = secs % 60
  const pad = (n: number) => n.toString().padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`
}

export function MeetingHeader({ meetId, sessionStartedAt }: MeetingHeaderProps) {
  const state = useConnectionState()
  const isConnected = state === ConnectionState.Connected
  const connectedAtRef = useRef<number | null>(null)
  const [elapsed, setElapsed] = useState('0:00')

  useEffect(() => {
    if (isConnected && connectedAtRef.current == null) {
      connectedAtRef.current = Date.now()
    }
  }, [isConnected])

  useEffect(() => {
    const tick = () => {
      const start = sessionStartedAt && sessionStartedAt > 0 ? sessionStartedAt : connectedAtRef.current
      if (start != null) setElapsed(formatElapsed(Date.now() - start))
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [sessionStartedAt])

  return (
    <header className="absolute left-0 right-0 top-0 z-20 flex items-center justify-center px-4 pointer-events-none h-[calc(56px+env(safe-area-inset-top))] pt-[env(safe-area-inset-top)]">
      <div className="flex items-center gap-2.5 pointer-events-auto">
        <div
          className="flex items-center gap-[5px] rounded-[7px] px-[9px] py-[3px]"
          style={{
            background: 'color-mix(in oklab, var(--accent-400) 20%, transparent)',
            border: '1px solid color-mix(in oklab, var(--accent-400) 40%, transparent)',
          }}
        >
          <Radio size={11} className="text-[var(--accent-400)]" />
          <span className="text-[var(--accent-300)] text-[11px] font-bold tracking-widest">LIVE</span>
        </div>
        <span className="text-white/25 text-[13px]">·</span>
        <span className="text-white/55 text-xs font-mono">{meetId}</span>
        <span className="text-white/25 text-[13px]">·</span>
        <span className="text-white/25 text-[11px] font-mono">{time}</span>
        <span className="text-white/25 text-[13px]">·</span>
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
      </div>
    </header>
  )
}
