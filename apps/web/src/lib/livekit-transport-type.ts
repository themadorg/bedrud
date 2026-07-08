import { ConnectionState, Room, RoomEvent } from 'livekit-client'
import { useEffect, useState } from 'react'

/** How media + chat reach LiveKit — same WebRTC peer connection either way. */
export type LiveKitTransportMode = 'p2p' | 'relay' | 'unknown'

type CandidateStats = {
  candidateType?: string
}

type EngineWithPublisherStats = {
  pcManager?: {
    publisher?: {
      getStats(): Promise<RTCStatsReport>
    }
  }
}

/** Parse nominated ICE pair from an RTC stats report. */
export function transportModeFromStatsReport(report: RTCStatsReport): LiveKitTransportMode {
  let selectedPairId = ''
  const pairs = new Map<string, RTCIceCandidatePairStats>()
  const candidates = new Map<string, CandidateStats>()

  report.forEach((entry) => {
    const stat = entry as Record<string, unknown>
    const type = stat.type as string | undefined
    if (type === 'transport' && typeof stat.selectedCandidatePairId === 'string') {
      selectedPairId = stat.selectedCandidatePairId
    }
    if (type === 'candidate-pair') {
      const pair = entry as RTCIceCandidatePairStats
      if (!selectedPairId && pair.selected) {
        selectedPairId = pair.id
      }
      pairs.set(pair.id, pair)
    }
    if (type === 'local-candidate' || type === 'remote-candidate') {
      candidates.set(entry.id, { candidateType: stat.candidateType as string | undefined })
    }
  })

  const pair = pairs.get(selectedPairId)
  if (!pair) return 'unknown'

  const local = candidates.get(pair.localCandidateId ?? '')
  const remote = candidates.get(pair.remoteCandidateId ?? '')
  if (local?.candidateType === 'relay' || remote?.candidateType === 'relay') {
    return 'relay'
  }
  if (
    local?.candidateType === 'host' ||
    local?.candidateType === 'srflx' ||
    remote?.candidateType === 'host' ||
    remote?.candidateType === 'srflx'
  ) {
    return 'p2p'
  }
  return 'unknown'
}

/** Selected ICE transport for the publisher peer connection (media + chat data channels). */
export async function getLiveKitTransportMode(room: Room): Promise<LiveKitTransportMode> {
  if (room.state !== ConnectionState.Connected) return 'unknown'
  try {
    const engine = room.engine as unknown as EngineWithPublisherStats
    const report = await engine.pcManager?.publisher?.getStats()
    if (!report) return 'unknown'
    return transportModeFromStatsReport(report)
  } catch {
    return 'unknown'
  }
}

export function liveKitTransportModeLabel(mode: LiveKitTransportMode): string {
  if (mode === 'p2p') return 'P2P'
  if (mode === 'relay') return 'Relay'
  return 'Connected'
}

export function useLiveKitTransportMode(room: Room | undefined, enabled: boolean): LiveKitTransportMode {
  const [mode, setMode] = useState<LiveKitTransportMode>('unknown')

  useEffect(() => {
    if (!enabled || !room) {
      setMode('unknown')
      return
    }

    let cancelled = false

    const refresh = async () => {
      const next = await getLiveKitTransportMode(room)
      if (!cancelled) setMode(next)
    }

    const onConnected = () => {
      void refresh()
    }

    room.on(RoomEvent.Connected, onConnected)
    room.on(RoomEvent.Reconnected, onConnected)
    if (room.state === ConnectionState.Connected) {
      void refresh()
    }

    const interval = window.setInterval(() => {
      if (room.state === ConnectionState.Connected) void refresh()
    }, 2000)

    return () => {
      cancelled = true
      room.off(RoomEvent.Connected, onConnected)
      room.off(RoomEvent.Reconnected, onConnected)
      window.clearInterval(interval)
    }
  }, [enabled, room])

  return mode
}