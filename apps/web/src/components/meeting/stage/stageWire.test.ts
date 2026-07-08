import { describe, expect, test } from 'vitest'
import { parseMeetingStage, parseStageWire, stageDescription } from './stageWire'

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
})