import { describe, expect, test } from 'vitest'
import { parseMeetingStage, parseStageWire, stageDescription, stageShareKey } from './stageWire'

describe('parseStageWire', () => {
  test('parses stage_set for whiteboard', () => {
    const wire = parseStageWire({
      type: 'stage_set',
      stage: {
        kind: 'whiteboard',
        ownerIdentity: 'user-1',
        ownerName: 'Alice',
        updatedAt: 1_700_000_000_000,
      },
    })
    expect(wire).toEqual({
      type: 'stage_set',
      stage: {
        kind: 'whiteboard',
        ownerIdentity: 'user-1',
        ownerName: 'Alice',
        updatedAt: 1_700_000_000_000,
      },
    })
  })

  test('parses stage_state response', () => {
    const wire = parseStageWire({
      type: 'stage_state',
      stage: {
        kind: 'screenshare',
        ownerIdentity: 'user-2',
        ownerName: 'Bob',
        updatedAt: 1_700_000_000_100,
      },
      ts: 1_700_000_000_100,
    })
    expect(wire?.type).toBe('stage_state')
    expect(parseMeetingStage((wire as { stage: unknown }).stage)?.kind).toBe('screenshare')
  })

  test('rejects stage_clear without ownerIdentity', () => {
    expect(parseStageWire({ type: 'stage_clear', ts: 123 })).toBeNull()
  })
})

describe('stageDescription', () => {
  test('describes youtube stage', () => {
    expect(
      stageDescription({
        kind: 'youtube',
        ownerIdentity: 'a',
        ownerName: 'Alice',
        videoId: 'abc',
        playing: false,
        currentTime: 0,
        updatedAt: 1,
      }),
    ).toContain('YouTube')
  })

  test('parses and describes webxdc stage', () => {
    const stage = parseMeetingStage({
      kind: 'webxdc',
      ownerIdentity: 'u1',
      ownerName: 'Ada',
      instanceId: 'inst-1',
      packageId: 'pkg-1',
      name: 'Chess',
      updatedAt: 99,
    })
    expect(stage?.kind).toBe('webxdc')
    expect(stageDescription(stage!)).toContain('mini-app')
    expect(stageDescription(stage!)).toContain('Chess')
  })
})

describe('stageShareKey', () => {
  test('ignores updatedAt so playhead sync does not re-key', () => {
    const a = {
      kind: 'webxdc' as const,
      ownerIdentity: 'u1',
      ownerName: 'Ada',
      instanceId: 'inst-1',
      packageId: 'pkg-1',
      name: 'Chess',
      updatedAt: 1,
    }
    const b = { ...a, updatedAt: 999 }
    expect(stageShareKey(a)).toBe(stageShareKey(b))
    expect(stageShareKey(a)).toBe('webxdc:inst-1')
  })
})
