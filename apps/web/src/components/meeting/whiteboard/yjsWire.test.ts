import { describe, expect, test } from 'vitest'
import {
  applyYjsChunkPart,
  assembledYjsFromChunks,
  buildYjsWirePackets,
  createYjsChunkBuffer,
  ingestYjsChunkMeta,
  parseYjsWirePacket,
} from './yjsWire'

describe('yjsWire', () => {
  test('round-trips a small yjs packet', () => {
    const original = new Uint8Array([1, 2, 3, 4, 5])
    const packets = buildYjsWirePackets(original)
    expect(packets).toHaveLength(1)

    const parsed = parseYjsWirePacket(packets[0])
    expect(parsed?.kind).toBe('raw')
    if (parsed?.kind !== 'raw') return

    expect(parsed.data).toEqual(original)
  })

  test('chunks oversized packets and reassembles', () => {
    const original = new Uint8Array(80_000)
    original.fill(7)
    const packets = buildYjsWirePackets(original)
    expect(packets.length).toBeGreaterThan(2)

    const buffers = createYjsChunkBuffer()
    let pending = null as ReturnType<typeof ingestYjsChunkMeta> | null

    for (const packet of packets) {
      const parsed = parseYjsWirePacket(packet)
      if (!parsed) continue

      if (parsed.kind === 'meta') {
        pending = ingestYjsChunkMeta(buffers, parsed.meta)
        continue
      }

      if (parsed.kind === 'chunk' && pending) {
        const done = applyYjsChunkPart(pending, parsed.chunk)
        if (done) {
          expect(assembledYjsFromChunks(done)).toEqual(original)
        }
      }
    }
  })
})
