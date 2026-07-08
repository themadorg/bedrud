import { describe, expect, it } from 'vitest'
import {
  applyChatChunkPart,
  assembledChatFromChunks,
  buildChatWirePackets,
  chatWireByteLength,
  CHAT_DATA_SAFE_BYTES,
  createChunkBuffer,
  encodeChatWire,
  ingestChatChunk,
} from './chatDataChannel'

const basePayload = {
  type: 'chat' as const,
  id: 'msg-1',
  timestamp: 1_700_000_000_000,
  senderName: 'Ada',
  senderIdentity: 'ada',
  message: '',
  attachments: [] as unknown[],
}

function reassemble(packets: ReturnType<typeof buildChatWirePackets>) {
  const meta = packets[0]
  if (meta.type !== 'chat_chunk_meta') throw new Error('expected meta')

  const buffers = createChunkBuffer()
  const entry = ingestChatChunk(buffers, meta)
  for (const packet of packets.slice(1)) {
    if (packet.type !== 'chat_chunk') continue
    const done = applyChatChunkPart(entry, packet)
    if (done) return assembledChatFromChunks(done)
  }
  throw new Error('message was not reassembled')
}

describe('buildChatWirePackets', () => {
  it('sends a single packet for small messages', () => {
    const packets = buildChatWirePackets({ ...basePayload, message: 'hello' })
    expect(packets).toHaveLength(1)
    expect(packets[0].type).toBe('chat')
  })

  it('splits oversized messages into meta + chunks', () => {
    const message = 'x'.repeat(CHAT_DATA_SAFE_BYTES)
    const packets = buildChatWirePackets({ ...basePayload, message })
    expect(packets[0].type).toBe('chat_chunk_meta')
    expect(packets.length).toBeGreaterThan(2)
    for (const packet of packets) {
      expect(encodeChatWire(packet).length).toBeLessThanOrEqual(CHAT_DATA_SAFE_BYTES)
    }
  })

  it('reassembles chunked messages', () => {
    const message = 'abcdefghijklmnopqrstuvwxyz'.repeat(4000)
    const assembled = reassemble(buildChatWirePackets({ ...basePayload, message }))
    expect(assembled.message).toBe(message)
  })

  it('chunks oversized inline image attachments', () => {
    const dataUrl = `data:image/png;base64,${'A'.repeat(CHAT_DATA_SAFE_BYTES)}`
    const attachments = [{ kind: 'image', url: dataUrl, mime: 'image/png', w: 100, h: 100, size: 50_000 }]
    const packets = buildChatWirePackets({ ...basePayload, attachments })
    expect(packets[0].type).toBe('chat_chunk_meta')
    if (packets[0].type !== 'chat_chunk_meta') throw new Error('expected meta')
    expect(packets[0].attachmentChunks).toBeGreaterThan(1)
    expect(packets[0].messageChunks).toBe(0)

    for (const packet of packets) {
      expect(encodeChatWire(packet).length).toBeLessThanOrEqual(CHAT_DATA_SAFE_BYTES)
    }

    const assembled = reassemble(packets)
    expect(assembled.attachments).toEqual(attachments)
    expect(assembled.message).toBe('')
  })

  it('chunks message text and attachments together', () => {
    const message = 'y'.repeat(CHAT_DATA_SAFE_BYTES)
    const dataUrl = `data:image/jpeg;base64,${'B'.repeat(80_000)}`
    const attachments = [{ kind: 'image', url: dataUrl, mime: 'image/jpeg', w: 200, h: 150, size: 60_000 }]
    const packets = buildChatWirePackets({ ...basePayload, message, attachments })
    if (packets[0].type !== 'chat_chunk_meta') throw new Error('expected meta')
    expect(packets[0].messageChunks).toBeGreaterThan(1)
    expect(packets[0].attachmentChunks).toBeGreaterThan(1)

    const assembled = reassemble(packets)
    expect(assembled.message).toBe(message)
    expect(assembled.attachments).toEqual(attachments)
  })

  it('reports wire size for utf-8 payloads', () => {
    expect(chatWireByteLength({ ...basePayload, message: '😀' })).toBeGreaterThan(basePayload.id.length)
  })
})