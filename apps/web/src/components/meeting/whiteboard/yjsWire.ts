export const WHITEBOARD_YJS_TOPIC = 'whiteboard-yjs'

/** LiveKit reliable data channel safe size (binary frame, no JSON/base64 overhead). */
export const YJS_DATA_SAFE_BYTES = 56_000

const TYPE_RAW = 1
const TYPE_META = 2
const TYPE_CHUNK = 3
const ID_LEN = 8
const META_HEADER = 1 + ID_LEN + 2
const CHUNK_HEADER = 1 + ID_LEN + 2

export type YjsChunkMeta = {
  id: string
  chunkCount: number
}

export type YjsChunkPart = {
  id: string
  index: number
  data: Uint8Array
}

function randomChunkId(): string {
  const bytes = new Uint8Array(ID_LEN)
  crypto.getRandomValues(bytes)
  return bytesToHex(bytes)
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16)
  }
  return bytes
}

function writeId(view: DataView, offset: number, idHex: string) {
  const idBytes = hexToBytes(idHex)
  for (let i = 0; i < ID_LEN; i++) {
    view.setUint8(offset + i, idBytes[i] ?? 0)
  }
}

function readId(view: DataView, offset: number): string {
  const bytes = new Uint8Array(ID_LEN)
  for (let i = 0; i < ID_LEN; i++) bytes[i] = view.getUint8(offset + i)
  return bytesToHex(bytes)
}

export function buildYjsWirePackets(data: Uint8Array): Uint8Array[] {
  if (data.length + 1 <= YJS_DATA_SAFE_BYTES) {
    const packet = new Uint8Array(1 + data.length)
    packet[0] = TYPE_RAW
    packet.set(data, 1)
    return [packet]
  }

  const id = randomChunkId()
  const maxChunkPayload = YJS_DATA_SAFE_BYTES - CHUNK_HEADER
  const chunkCount = Math.ceil(data.length / maxChunkPayload)
  const packets: Uint8Array[] = []

  const meta = new Uint8Array(META_HEADER)
  meta[0] = TYPE_META
  const metaView = new DataView(meta.buffer)
  writeId(metaView, 1, id)
  metaView.setUint16(1 + ID_LEN, chunkCount, false)
  packets.push(meta)

  for (let index = 0; index < chunkCount; index++) {
    const start = index * maxChunkPayload
    const part = data.subarray(start, start + maxChunkPayload)
    const packet = new Uint8Array(CHUNK_HEADER + part.length)
    packet[0] = TYPE_CHUNK
    const view = new DataView(packet.buffer)
    writeId(view, 1, id)
    view.setUint16(1 + ID_LEN, index, false)
    packet.set(part, CHUNK_HEADER)
    packets.push(packet)
  }

  return packets
}

export interface PendingYjsChunks {
  meta: YjsChunkMeta
  parts: Uint8Array[]
  received: number
  updatedAt: number
}

export function createYjsChunkBuffer(): Map<string, PendingYjsChunks> {
  return new Map()
}

export function ingestYjsChunkMeta(buffers: Map<string, PendingYjsChunks>, meta: YjsChunkMeta): PendingYjsChunks {
  const existing = buffers.get(meta.id)
  if (existing && existing.meta.chunkCount === meta.chunkCount) {
    existing.updatedAt = Date.now()
    return existing
  }

  const entry: PendingYjsChunks = {
    meta,
    parts: Array.from({ length: meta.chunkCount }, () => new Uint8Array(0)),
    received: 0,
    updatedAt: Date.now(),
  }
  buffers.set(meta.id, entry)
  return entry
}

export function applyYjsChunkPart(entry: PendingYjsChunks, chunk: YjsChunkPart): PendingYjsChunks | null {
  if (chunk.id !== entry.meta.id) return null
  if (chunk.index < 0 || chunk.index >= entry.parts.length) return null
  if (entry.parts[chunk.index]?.length > 0) return null

  entry.parts[chunk.index] = chunk.data
  entry.received += 1
  entry.updatedAt = Date.now()

  if (entry.received < entry.meta.chunkCount) return null
  return entry
}

export function assembledYjsFromChunks(entry: PendingYjsChunks): Uint8Array {
  const total = entry.parts.reduce((sum, part) => sum + part.length, 0)
  const merged = new Uint8Array(total)
  let offset = 0
  for (const part of entry.parts) {
    merged.set(part, offset)
    offset += part.length
  }
  return merged
}

export function pruneYjsChunkBuffers(buffers: Map<string, PendingYjsChunks>, maxAgeMs = 120_000): void {
  const cutoff = Date.now() - maxAgeMs
  for (const [id, entry] of buffers) {
    if (entry.updatedAt < cutoff) buffers.delete(id)
  }
}

export type ParsedYjsWire =
  | { kind: 'raw'; data: Uint8Array }
  | { kind: 'meta'; meta: YjsChunkMeta }
  | { kind: 'chunk'; chunk: YjsChunkPart }

export function parseYjsWirePacket(payload: Uint8Array): ParsedYjsWire | null {
  if (payload.length < 1) return null
  const type = payload[0]

  if (type === TYPE_RAW) {
    return { kind: 'raw', data: payload.subarray(1) }
  }

  if (type === TYPE_META && payload.length >= META_HEADER) {
    const view = new DataView(payload.buffer, payload.byteOffset, payload.byteLength)
    return {
      kind: 'meta',
      meta: {
        id: readId(view, 1),
        chunkCount: view.getUint16(1 + ID_LEN, false),
      },
    }
  }

  if (type === TYPE_CHUNK && payload.length > CHUNK_HEADER) {
    const view = new DataView(payload.buffer, payload.byteOffset, payload.byteLength)
    return {
      kind: 'chunk',
      chunk: {
        id: readId(view, 1),
        index: view.getUint16(1 + ID_LEN, false),
        data: payload.subarray(CHUNK_HEADER),
      },
    }
  }

  return null
}
