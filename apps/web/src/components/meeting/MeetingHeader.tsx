import { useConnectionState } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useRef, useState } from 'react'

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
    <header
      className="absolute left-0 right-0 top-0 z-20 flex items-center justify-center px-4"
      style={{
        pointerEvents: 'none',
        height: 'calc(56px + env(safe-area-inset-top, 0px))',
        paddingTop: 'env(safe-area-inset-top, 0px)',
      }}
    >
      <div className="flex items-center gap-2.5" style={{ pointerEvents: 'auto' }}>
        <span style={{ color: 'rgba(255,255,255,0.55)', fontSize: 12, fontFamily: 'monospace' }}>{meetId}</span>
        <span style={{ color: 'rgba(255,255,255,0.25)', fontSize: 13 }}>·</span>
        <span style={{ color: 'rgba(255,255,255,0.25)', fontSize: 11, fontFamily: 'monospace' }}>{elapsed}</span>
        <span style={{ color: 'rgba(255,255,255,0.25)', fontSize: 13 }}>·</span>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 5,
            background: isConnected ? 'rgba(34,197,94,0.12)' : 'rgba(234,179,8,0.12)',
            border: `1px solid ${isConnected ? 'rgba(34,197,94,0.25)' : 'rgba(234,179,8,0.25)'}`,
            borderRadius: 7,
            padding: '3px 9px',
          }}
        >
          {isConnected ? (
            <span
              style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e', display: 'inline-block' }}
            />
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
          <span style={{ color: isConnected ? '#86efac' : '#fde047', fontSize: 11, fontWeight: 500 }}>
            {isConnected ? 'Connected' : state}
          </span>
        </div>
      </div>
    </header>
  )
}
