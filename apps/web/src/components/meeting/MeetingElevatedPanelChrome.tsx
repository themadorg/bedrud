import { useConnectionState } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import {
  formatMeetingClock,
  formatMeetingElapsed,
  getMeetingJoinedAtMs,
  noteMeetingConnected,
} from '@/components/meeting/meetingSessionTime'
import { cn } from '@/lib/utils'

export function useMeetingElapsedClock() {
  const connectionState = useConnectionState()
  const isConnected = connectionState === ConnectionState.Connected
  const [elapsed, setElapsed] = useState('00:00')
  const [clock, setClock] = useState(() => formatMeetingClock(new Date()))

  useEffect(() => {
    if (isConnected) noteMeetingConnected()
  }, [isConnected])

  useEffect(() => {
    const tick = () => {
      setClock(formatMeetingClock(new Date()))
      const joined = getMeetingJoinedAtMs()
      if (joined != null) {
        setElapsed(formatMeetingElapsed(Date.now() - joined))
      } else {
        setElapsed('00:00')
      }
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [isConnected])

  return { elapsed, clock }
}

type HeaderProps = {
  title: ReactNode
  onClose: () => void
  closeLabel: string
  leading?: ReactNode
}

export function MeetingElevatedPanelHeader({ title, onClose, closeLabel, leading }: HeaderProps) {
  return (
    <header className="flex shrink-0 items-center gap-1 border-b border-[var(--meet-border)]">
      <div className="flex h-12 w-full items-center px-1">
        {leading ?? <span className="flex-1 px-3 text-[17px] font-semibold text-[var(--meet-fg-strong)]">{title}</span>}
        <button
          type="button"
          onClick={onClose}
          className="flex h-11 w-11 shrink-0 items-center justify-center border-none bg-transparent text-[var(--meet-fg-muted)]"
          aria-label={closeLabel}
        >
          <X size={20} />
        </button>
      </div>
    </header>
  )
}

export function MeetingElevatedMeetingSubheader() {
  const { elapsed, clock } = useMeetingElapsedClock()

  return (
    <div className="flex shrink-0 items-end justify-between gap-4 border-b border-[var(--meet-border)] px-4 py-3.5">
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
  )
}

export function MeetingElevatedPanelSectionSubheader({ title, className }: { title: string; className?: string }) {
  return (
    <div className={cn('shrink-0 border-b border-[var(--meet-border)] px-4 py-2', className)}>
      <h2 className="text-[15px] font-semibold text-[var(--meet-fg-strong)]">{title}</h2>
    </div>
  )
}

export function MeetingElevatedPanelBody({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('flex min-h-0 flex-1 flex-col', className)}>{children}</div>
}
