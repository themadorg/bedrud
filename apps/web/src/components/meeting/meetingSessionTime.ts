/**
 * Shared session clock for meeting UI (header + room info).
 * Starts on first LiveKit Connected; clears on disconnect.
 */

let joinedAtMs: number | null = null

export function noteMeetingConnected(now = Date.now()): void {
  if (joinedAtMs == null) joinedAtMs = now
}

export function noteMeetingDisconnected(): void {
  joinedAtMs = null
}

export function getMeetingJoinedAtMs(): number | null {
  return joinedAtMs
}

export function formatMeetingElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function formatMeetingClock(now: Date): string {
  return now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
