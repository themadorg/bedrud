import { describe, expect, it } from 'vitest'
import { decodeWebxdcWire, encodeWebxdcWire, parseWebxdcWire } from './webxdcWire'

describe('parseWebxdcWire', () => {
  it('parses status wire', () => {
    const wire = parseWebxdcWire({
      v: 1,
      kind: 'status',
      appId: 'app-1',
      serial: 3,
      ts: 1_700_000_000_000,
      update: { payload: { n: 1 } },
    })
    expect(wire?.kind).toBe('status')
    if (wire?.kind === 'status') {
      expect(wire.serial).toBe(3)
      expect(wire.update.payload).toEqual({ n: 1 })
    }
  })

  it('rejects serial < 1', () => {
    expect(
      parseWebxdcWire({
        v: 1,
        kind: 'status',
        appId: 'a',
        serial: 0,
        ts: 1,
        update: { payload: 1 },
      }),
    ).toBeNull()
  })

  it('parses control close', () => {
    const wire = parseWebxdcWire({
      v: 1,
      kind: 'control',
      appId: 'app-1',
      action: 'close',
    })
    expect(wire).toEqual({
      v: 1,
      kind: 'control',
      appId: 'app-1',
      action: 'close',
    })
  })

  it('round-trips encode/decode', () => {
    const original = {
      v: 1 as const,
      kind: 'status' as const,
      appId: 'x',
      serial: 2,
      ts: 99,
      update: { payload: 'hello' },
    }
    const bytes = encodeWebxdcWire(original)
    const back = decodeWebxdcWire(bytes)
    expect(back).toEqual(original)
  })

  it('rejects unknown version', () => {
    expect(
      parseWebxdcWire({
        v: 2,
        kind: 'status',
        appId: 'a',
        serial: 1,
        ts: 1,
        update: { payload: 1 },
      }),
    ).toBeNull()
  })
})
