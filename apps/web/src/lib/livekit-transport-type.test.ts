import { describe, expect, it } from 'vitest'
import { liveKitTransportModeLabel, transportModeFromStatsReport } from './livekit-transport-type'

function mockStats(entries: Record<string, unknown>[]): RTCStatsReport {
  const map = new Map<string, Record<string, unknown>>()
  for (const entry of entries) {
    map.set(entry.id as string, entry)
  }
  return {
    forEach(callback: (value: RTCStats) => void) {
      map.forEach((value) => callback(value as RTCStats))
    },
  } as RTCStatsReport
}

describe('livekit-transport-type', () => {
  it('detects P2P from host/srflx candidates', () => {
    const report = mockStats([
      { id: 't1', type: 'transport', selectedCandidatePairId: 'pair1' },
      {
        id: 'pair1',
        type: 'candidate-pair',
        nominated: true,
        localCandidateId: 'local1',
        remoteCandidateId: 'remote1',
      },
      { id: 'local1', type: 'local-candidate', candidateType: 'srflx' },
      { id: 'remote1', type: 'remote-candidate', candidateType: 'host' },
    ])
    expect(transportModeFromStatsReport(report)).toBe('p2p')
  })

  it('detects relay when either side uses TURN', () => {
    const report = mockStats([
      { id: 't1', type: 'transport', selectedCandidatePairId: 'pair1' },
      {
        id: 'pair1',
        type: 'candidate-pair',
        nominated: true,
        localCandidateId: 'local1',
        remoteCandidateId: 'remote1',
      },
      { id: 'local1', type: 'local-candidate', candidateType: 'relay' },
      { id: 'remote1', type: 'remote-candidate', candidateType: 'host' },
    ])
    expect(transportModeFromStatsReport(report)).toBe('relay')
  })

  it('labels transport modes for the header', () => {
    expect(liveKitTransportModeLabel('p2p')).toBe('P2P')
    expect(liveKitTransportModeLabel('relay')).toBe('Relay')
    expect(liveKitTransportModeLabel('unknown')).toBe('Connected')
  })
})