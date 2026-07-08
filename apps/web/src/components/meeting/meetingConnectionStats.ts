import type { LocalTrack, RemoteTrack } from 'livekit-client'
import { ConnectionQuality, type Participant, Track } from 'livekit-client'
import { useEffect, useMemo, useState } from 'react'

export type ConnectionQualityLabel = 'excellent' | 'good' | 'poor' | 'unknown'

export interface MeetingConnectionStats {
  quality: ConnectionQualityLabel
  codec?: string
  ping?: number
  jitter?: number
  packetLoss?: number
  bandwidth?: number
}

const TRACK_SOURCES = [Track.Source.Microphone, Track.Source.Camera, Track.Source.ScreenShare] as const

export function connectionQualityLabel(participant: Participant): ConnectionQualityLabel {
  if (participant.connectionQuality === ConnectionQuality.Excellent) return 'excellent'
  if (participant.connectionQuality === ConnectionQuality.Good) return 'good'
  if (participant.connectionQuality === ConnectionQuality.Poor) return 'poor'
  return 'unknown'
}

async function getParticipantStatsReport(participant: Participant): Promise<RTCStatsReport | undefined> {
  for (const source of TRACK_SOURCES) {
    const pub = participant.getTrackPublication(source)
    if (!pub?.track) continue

    try {
      if (participant.isLocal) {
        const sender = (pub.track as LocalTrack).sender
        if (sender) return await sender.getStats()
      } else {
        const receiver = (pub.track as RemoteTrack).receiver
        if (receiver) return await receiver.getStats()
      }
    } catch {
      /* try next track */
    }
  }
  return undefined
}

function parseStatsReport(report: RTCStatsReport): Partial<MeetingConnectionStats> {
  let codec: string | undefined
  let ping: number | undefined
  let bandwidth: number | undefined
  let jitterMs: number | undefined
  let packetsLost = 0
  let packetsReceived = 0

  report.forEach((entry) => {
    const s = entry as Record<string, unknown>
    if (s.type === 'codec' && !codec) {
      const mime = s.mimeType as string | undefined
      codec = mime?.replace(/^(audio|video)\//i, '').toUpperCase()
    }
    if (s.type === 'candidate-pair' && s.nominated === true) {
      const rtt = s.currentRoundTripTime as number | undefined
      if (rtt != null) ping = Math.round(rtt * 1000)
      const bw = (s.availableIncomingBitrate ?? s.availableOutgoingBitrate) as number | undefined
      if (bw != null) bandwidth = Math.round(bw / 1000)
    }
    if (s.type === 'inbound-rtp') {
      const jitter = s.jitter as number | undefined
      if (jitter != null) {
        const ms = Math.round(jitter * 1000)
        jitterMs = jitterMs == null ? ms : Math.max(jitterMs, ms)
      }
      packetsLost += (s.packetsLost as number) ?? 0
      packetsReceived += (s.packetsReceived as number) ?? 0
    }
  })

  const total = packetsLost + packetsReceived
  const packetLoss = total > 0 ? Math.round((packetsLost / total) * 1000) / 10 : undefined

  return { codec, ping, bandwidth, jitter: jitterMs, packetLoss }
}

export async function collectMeetingConnectionStats(
  participant: Participant,
): Promise<Partial<MeetingConnectionStats>> {
  const report = await getParticipantStatsReport(participant)
  if (!report) return {}
  return parseStatsReport(report)
}

export function latencyColorClass(ms: number) {
  if (ms < 80) return 'text-emerald-600 dark:text-emerald-400'
  if (ms < 200) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-500'
}

export function packetLossColorClass(pct: number) {
  if (pct < 1) return 'text-emerald-600 dark:text-emerald-400'
  if (pct < 3) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-500'
}

export function qualityColorClass(quality: ConnectionQualityLabel) {
  if (quality === 'excellent') return 'text-emerald-600 dark:text-emerald-400'
  if (quality === 'good') return 'text-amber-600 dark:text-amber-400'
  if (quality === 'poor') return 'text-red-500'
  return 'text-[var(--meet-fg-muted)]'
}

export function useMeetingConnectionStats(participant: Participant | undefined, enabled: boolean) {
  const quality = useMemo(() => (participant ? connectionQualityLabel(participant) : 'unknown'), [participant])
  const [stats, setStats] = useState<MeetingConnectionStats | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!enabled || !participant) {
      setStats(null)
      setLoading(false)
      return
    }

    let cancelled = false

    async function poll() {
      setLoading(true)
      while (!cancelled) {
        const partial = await collectMeetingConnectionStats(participant)
        if (cancelled) break

        setStats((prev) => ({
          quality,
          codec: partial.codec ?? prev?.codec,
          ping: partial.ping ?? prev?.ping,
          jitter: partial.jitter ?? prev?.jitter,
          packetLoss: partial.packetLoss ?? prev?.packetLoss,
          bandwidth: partial.bandwidth ?? prev?.bandwidth,
        }))
        setLoading(false)

        await new Promise<void>((resolve) => {
          setTimeout(resolve, 1500)
        })
      }
    }

    void poll()
    return () => {
      cancelled = true
    }
  }, [enabled, participant, quality])

  useEffect(() => {
    if (enabled) {
      setStats((prev) => (prev ? { ...prev, quality } : null))
    }
  }, [enabled, quality])

  return { stats, loading, quality }
}
