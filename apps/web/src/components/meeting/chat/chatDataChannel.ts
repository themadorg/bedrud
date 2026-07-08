/** LiveKit RTC data channel max payload (bytes). */
export const CHAT_DATA_MAX_BYTES = 65_535

/** Leave headroom for encoder / broker overhead. */
export const CHAT_DATA_SAFE_BYTES = 60_000

export interface ChatWirePayload {
  type: 'chat'
  id: string
  timestamp: number
  senderName: string
  senderIdentity: string
  message: string
  attachments: unknown[]
  poll?: unknown
}

export interface ChatChunkMetaWire {
  type: 'chat_chunk_meta'
  id: string
  timestamp: number
  senderName: string
  senderIdentity: string
  messageChunks: number
  attachmentChunks: number
  pollChunks: number
}

export type ChatChunkKind = 'message' | 'attachments' | 'poll'

export interface ChatChunkWire {
  type: 'chat_chunk'
  id: string
  kind: ChatChunkKind
  index: number
  part: string
}

export type ChatOutboundWire = ChatWirePayload | ChatChunkMetaWire | ChatChunkWire

export function encodeChatWire(payload: ChatOutboundWire): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(payload))
}

export function chatWireByteLength(payload: ChatOutboundWire): number {
  return encodeChatWire(payload).length
}

function splitUtf8ByMaxBytes(text: string, maxBytes: number, build: (part: string) => ChatOutboundWire): string[] {
  if (!text) return []
  const parts: string[] = []
  let remaining = text

  while (remaining.length > 0) {
    let low = 1
    let high = remaining.length
    let best = 0

    while (low <= high) {
      const mid = Math.floor((low + high) / 2)
      const candidate = remaining.slice(0, mid)
      if (chatWireByteLength(build(candidate)) <= maxBytes) {
        best = mid
        low = mid + 1
      } else {
        high = mid - 1
      }
    }

    if (best === 0) {
      throw new Error('Chat chunk overhead exceeds data channel limit')
    }

    parts.push(remaining.slice(0, best))
    remaining = remaining.slice(best)
  }

  return parts
}

function chunkSection(id: string, kind: ChatChunkKind, text: string): ChatChunkWire[] {
  const parts = splitUtf8ByMaxBytes(text, CHAT_DATA_SAFE_BYTES, (part) => ({
    type: 'chat_chunk',
    id,
    kind,
    index: 0,
    part,
  }))
  return parts.map((part, index) => ({
    type: 'chat_chunk',
    id,
    kind,
    index,
    part,
  }))
}

export function buildChatWirePackets(payload: ChatWirePayload): ChatOutboundWire[] {
  const single: ChatWirePayload = { ...payload, type: 'chat' }
  if (chatWireByteLength(single) <= CHAT_DATA_SAFE_BYTES) {
    return [single]
  }

  const { message, attachments, poll, ...rest } = payload
  const attachmentsJson = attachments.length > 0 ? JSON.stringify(attachments) : ''
  const pollJson = poll !== undefined ? JSON.stringify(poll) : ''

  const messageParts = chunkSection(rest.id, 'message', message)
  const attachmentParts = chunkSection(rest.id, 'attachments', attachmentsJson)
  const pollParts = chunkSection(rest.id, 'poll', pollJson)

  const meta: ChatChunkMetaWire = {
    type: 'chat_chunk_meta',
    id: rest.id,
    timestamp: rest.timestamp,
    senderName: rest.senderName,
    senderIdentity: rest.senderIdentity,
    messageChunks: messageParts.length,
    attachmentChunks: attachmentParts.length,
    pollChunks: pollParts.length,
  }

  if (chatWireByteLength(meta) > CHAT_DATA_SAFE_BYTES) {
    throw new Error('Chat metadata exceeds data channel limit')
  }

  if (messageParts.length + attachmentParts.length + pollParts.length === 0) {
    throw new Error('Chat message exceeds data channel limit')
  }

  return [meta, ...messageParts, ...attachmentParts, ...pollParts]
}

export interface PendingChatChunks {
  meta: ChatChunkMetaWire
  messageParts: string[]
  attachmentParts: string[]
  pollParts: string[]
  messageReceived: number
  attachmentReceived: number
  pollReceived: number
  updatedAt: number
}

export function createChunkBuffer(): Map<string, PendingChatChunks> {
  return new Map()
}

function metaMatches(a: ChatChunkMetaWire, b: ChatChunkMetaWire): boolean {
  return (
    a.messageChunks === b.messageChunks && a.attachmentChunks === b.attachmentChunks && a.pollChunks === b.pollChunks
  )
}

export function ingestChatChunk(buffers: Map<string, PendingChatChunks>, meta: ChatChunkMetaWire): PendingChatChunks {
  const existing = buffers.get(meta.id)
  if (existing && metaMatches(existing.meta, meta)) {
    existing.updatedAt = Date.now()
    return existing
  }

  const entry: PendingChatChunks = {
    meta,
    messageParts: Array.from({ length: meta.messageChunks }, () => ''),
    attachmentParts: Array.from({ length: meta.attachmentChunks }, () => ''),
    pollParts: Array.from({ length: meta.pollChunks }, () => ''),
    messageReceived: 0,
    attachmentReceived: 0,
    pollReceived: 0,
    updatedAt: Date.now(),
  }
  buffers.set(meta.id, entry)
  return entry
}

function applySectionPart(parts: string[], received: number, index: number, part: string): number | null {
  if (index < 0 || index >= parts.length) return null
  if (parts[index] !== '') return received
  parts[index] = part
  return received + 1
}

function isChunkAssemblyComplete(entry: PendingChatChunks): boolean {
  const { meta } = entry
  return (
    entry.messageReceived >= meta.messageChunks &&
    entry.attachmentReceived >= meta.attachmentChunks &&
    entry.pollReceived >= meta.pollChunks
  )
}

export function applyChatChunkPart(entry: PendingChatChunks, chunk: ChatChunkWire): PendingChatChunks | null {
  const { meta } = entry
  if (chunk.id !== meta.id) return null

  let nextReceived: number | null = null
  switch (chunk.kind) {
    case 'message':
      nextReceived = applySectionPart(entry.messageParts, entry.messageReceived, chunk.index, chunk.part)
      if (nextReceived !== null) entry.messageReceived = nextReceived
      break
    case 'attachments':
      nextReceived = applySectionPart(entry.attachmentParts, entry.attachmentReceived, chunk.index, chunk.part)
      if (nextReceived !== null) entry.attachmentReceived = nextReceived
      break
    case 'poll':
      nextReceived = applySectionPart(entry.pollParts, entry.pollReceived, chunk.index, chunk.part)
      if (nextReceived !== null) entry.pollReceived = nextReceived
      break
    default:
      return null
  }

  if (nextReceived === null) return null

  entry.updatedAt = Date.now()
  if (!isChunkAssemblyComplete(entry)) return null

  return entry
}

export function assembledChatFromChunks(entry: PendingChatChunks): ChatWirePayload {
  const message = entry.messageParts.join('')
  let attachments: unknown[] = []
  if (entry.meta.attachmentChunks > 0) {
    attachments = JSON.parse(entry.attachmentParts.join('')) as unknown[]
  }

  const result: ChatWirePayload = {
    type: 'chat',
    id: entry.meta.id,
    timestamp: entry.meta.timestamp,
    senderName: entry.meta.senderName,
    senderIdentity: entry.meta.senderIdentity,
    message,
    attachments,
  }

  if (entry.meta.pollChunks > 0) {
    result.poll = JSON.parse(entry.pollParts.join(''))
  }

  return result
}

export function pruneChunkBuffers(buffers: Map<string, PendingChatChunks>, maxAgeMs = 120_000): void {
  const cutoff = Date.now() - maxAgeMs
  for (const [id, entry] of buffers) {
    if (entry.updatedAt < cutoff) buffers.delete(id)
  }
}
