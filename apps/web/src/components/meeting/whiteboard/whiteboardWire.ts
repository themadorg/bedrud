import { restoreElements } from '@excalidraw/excalidraw'
import type { ExcalidrawTextElement, OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { AppState, BinaryFiles } from '@excalidraw/excalidraw/types'

export const WHITEBOARD_DATA_TOPIC = 'whiteboard'
export const WHITEBOARD_DATA_SAFE_BYTES = 60_000

export interface WhiteboardSession {
  hostIdentity: string
  hostName: string
  updatedAt: number
}

export interface WhiteboardScenePayload {
  elements: readonly OrderedExcalidrawElement[]
  appState?: Partial<AppState>
  files?: BinaryFiles
}

export type WhiteboardWire =
  | {
      type: 'whiteboard_start'
      hostIdentity: string
      hostName: string
      ts: number
    }
  | {
      type: 'whiteboard_update'
      senderIdentity: string
      elements: readonly OrderedExcalidrawElement[]
      appState?: Partial<AppState>
      files?: BinaryFiles
      ts: number
    }
  | { type: 'whiteboard_stop'; hostIdentity: string; ts: number }
  | { type: 'whiteboard_state_request'; ts: number }
  | {
      type: 'whiteboard_state'
      hostIdentity: string
      hostName: string
      elements: readonly OrderedExcalidrawElement[]
      appState?: Partial<AppState>
      files?: BinaryFiles
      ts: number
    }

export type WhiteboardChunkMetaWire = {
  type: 'whiteboard_chunk_meta'
  id: string
  chunkCount: number
}

export type WhiteboardChunkWire = {
  type: 'whiteboard_chunk'
  id: string
  index: number
  part: string
}

export type WhiteboardOutboundWire = WhiteboardWire | WhiteboardChunkMetaWire | WhiteboardChunkWire

export function encodeWhiteboardWire(payload: WhiteboardOutboundWire): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(payload))
}

function wireByteLength(payload: WhiteboardOutboundWire): number {
  return encodeWhiteboardWire(payload).length
}

function isElementArray(value: unknown): value is readonly OrderedExcalidrawElement[] {
  return Array.isArray(value)
}

export function pickReferencedFiles(elements: readonly OrderedExcalidrawElement[], files: BinaryFiles): BinaryFiles {
  const fileIds = new Set<string>()
  for (const el of elements) {
    if (el.type === 'image' && 'fileId' in el && typeof el.fileId === 'string' && el.fileId) {
      fileIds.add(el.fileId)
    }
  }
  const picked: BinaryFiles = {}
  for (const id of fileIds) {
    const file = files[id]
    if (file?.dataURL) picked[id] = file
  }
  return picked
}

function markSavedImageElements(elements: OrderedExcalidrawElement[], files?: BinaryFiles): OrderedExcalidrawElement[] {
  if (!files) return elements
  return elements.map((el) => {
    if (el.type !== 'image' || !('fileId' in el) || !el.fileId || !files[el.fileId]?.dataURL) return el
    if (el.status === 'saved') return el
    return { ...el, status: 'saved' as const }
  })
}

export function sceneElementsSignature(elements: readonly OrderedExcalidrawElement[]): string {
  const parts: string[] = []
  for (const el of elements) {
    if (el.isDeleted) continue
    if (el.type === 'text') {
      const textEl = el as ExcalidrawTextElement
      parts.push(`${el.id}:${el.version}:${textEl.originalText ?? ''}`)
      continue
    }
    parts.push(`${el.id}:${el.version}`)
  }
  return parts.join('|')
}

export function textElementsSignature(elements: readonly OrderedExcalidrawElement[]): string {
  const parts: string[] = []
  for (const el of elements) {
    if (el.isDeleted || el.type !== 'text') continue
    const textEl = el as ExcalidrawTextElement
    parts.push(`${el.id}:${el.version}:${textEl.originalText ?? ''}`)
  }
  return parts.join('|')
}

export function restoreSceneElements(elements: readonly OrderedExcalidrawElement[]): OrderedExcalidrawElement[] {
  return restoreElements(elements, null) as OrderedExcalidrawElement[]
}

function splitUtf8ByMaxBytes(text: string, maxBytes: number, build: (part: string) => WhiteboardChunkWire): string[] {
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
      if (wireByteLength(build(candidate)) <= maxBytes) {
        best = mid
        low = mid + 1
      } else {
        high = mid - 1
      }
    }

    if (best === 0) {
      throw new Error('Whiteboard chunk overhead exceeds data channel limit')
    }

    parts.push(remaining.slice(0, best))
    remaining = remaining.slice(best)
  }

  return parts
}

export function buildWhiteboardWirePackets(wire: WhiteboardWire): WhiteboardOutboundWire[] {
  if (wireByteLength(wire) <= WHITEBOARD_DATA_SAFE_BYTES) {
    return [wire]
  }

  const id =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  const json = JSON.stringify(wire)
  const textParts = splitUtf8ByMaxBytes(json, WHITEBOARD_DATA_SAFE_BYTES, (part) => ({
    type: 'whiteboard_chunk',
    id,
    index: 0,
    part,
  }))
  const chunks = textParts.map((part, index) => ({
    type: 'whiteboard_chunk' as const,
    id,
    index,
    part,
  }))

  const meta: WhiteboardChunkMetaWire = {
    type: 'whiteboard_chunk_meta',
    id,
    chunkCount: chunks.length,
  }

  if (wireByteLength(meta) > WHITEBOARD_DATA_SAFE_BYTES) {
    throw new Error('Whiteboard metadata exceeds data channel limit')
  }

  return [meta, ...chunks]
}

export interface PendingWhiteboardChunks {
  meta: WhiteboardChunkMetaWire
  parts: string[]
  received: number
  updatedAt: number
}

export function createWhiteboardChunkBuffer(): Map<string, PendingWhiteboardChunks> {
  return new Map()
}

export function ingestWhiteboardChunkMeta(
  buffers: Map<string, PendingWhiteboardChunks>,
  meta: WhiteboardChunkMetaWire,
): PendingWhiteboardChunks {
  const existing = buffers.get(meta.id)
  if (existing && existing.meta.chunkCount === meta.chunkCount) {
    existing.updatedAt = Date.now()
    return existing
  }

  const entry: PendingWhiteboardChunks = {
    meta,
    parts: Array.from({ length: meta.chunkCount }, () => ''),
    received: 0,
    updatedAt: Date.now(),
  }
  buffers.set(meta.id, entry)
  return entry
}

export function applyWhiteboardChunkPart(
  entry: PendingWhiteboardChunks,
  chunk: WhiteboardChunkWire,
): PendingWhiteboardChunks | null {
  if (chunk.id !== entry.meta.id) return null
  if (chunk.index < 0 || chunk.index >= entry.parts.length) return null
  if (entry.parts[chunk.index] !== '') return null

  entry.parts[chunk.index] = chunk.part
  entry.received += 1
  entry.updatedAt = Date.now()

  if (entry.received < entry.meta.chunkCount) return null
  return entry
}

export function assembledWhiteboardFromChunks(entry: PendingWhiteboardChunks): WhiteboardWire {
  return JSON.parse(entry.parts.join('')) as WhiteboardWire
}

export function pruneWhiteboardChunkBuffers(buffers: Map<string, PendingWhiteboardChunks>, maxAgeMs = 120_000): void {
  const cutoff = Date.now() - maxAgeMs
  for (const [id, entry] of buffers) {
    if (entry.updatedAt < cutoff) buffers.delete(id)
  }
}

export function parseWhiteboardWire(raw: unknown): WhiteboardWire | null {
  if (!raw || typeof raw !== 'object') return null
  const msg = raw as Record<string, unknown>
  if (typeof msg.type !== 'string') return null

  switch (msg.type) {
    case 'whiteboard_start':
      if (typeof msg.hostIdentity !== 'string' || typeof msg.hostName !== 'string') return null
      if (typeof msg.ts !== 'number') return null
      return msg as WhiteboardWire
    case 'whiteboard_update':
      if (typeof msg.senderIdentity !== 'string' || typeof msg.ts !== 'number') return null
      if (!isElementArray(msg.elements)) return null
      return msg as WhiteboardWire
    case 'whiteboard_stop':
      if (typeof msg.hostIdentity !== 'string' || typeof msg.ts !== 'number') return null
      return msg as WhiteboardWire
    case 'whiteboard_state_request':
      if (typeof msg.ts !== 'number') return null
      return msg as WhiteboardWire
    case 'whiteboard_state':
      if (typeof msg.hostIdentity !== 'string' || typeof msg.hostName !== 'string') return null
      if (typeof msg.ts !== 'number' || !isElementArray(msg.elements)) return null
      return msg as WhiteboardWire
    default:
      return null
  }
}

export function parseWhiteboardChunkMeta(raw: unknown): WhiteboardChunkMetaWire | null {
  if (!raw || typeof raw !== 'object') return null
  const msg = raw as Record<string, unknown>
  if (msg.type !== 'whiteboard_chunk_meta') return null
  if (typeof msg.id !== 'string' || typeof msg.chunkCount !== 'number') return null
  if (msg.chunkCount < 1) return null
  return msg as WhiteboardChunkMetaWire
}

export function parseWhiteboardChunk(raw: unknown): WhiteboardChunkWire | null {
  if (!raw || typeof raw !== 'object') return null
  const msg = raw as Record<string, unknown>
  if (msg.type !== 'whiteboard_chunk') return null
  if (typeof msg.id !== 'string' || typeof msg.index !== 'number' || typeof msg.part !== 'string') return null
  return msg as WhiteboardChunkWire
}

export function sessionFromWire(
  wire: Extract<WhiteboardWire, { type: 'whiteboard_start' | 'whiteboard_state' }>,
): WhiteboardSession {
  return {
    hostIdentity: wire.hostIdentity,
    hostName: wire.hostName,
    updatedAt: wire.ts,
  }
}

export function sceneFromWire(
  wire: Extract<WhiteboardWire, { type: 'whiteboard_update' | 'whiteboard_state' }>,
): WhiteboardScenePayload {
  return {
    elements: wire.elements,
    appState: wire.appState,
    files: wire.files,
  }
}

export function normalizeSceneForWire(scene: WhiteboardScenePayload): WhiteboardScenePayload {
  const files = scene.files ? pickReferencedFiles(scene.elements, scene.files) : undefined
  const elements = markSavedImageElements(restoreSceneElements(scene.elements), files)
  return {
    ...scene,
    elements,
    files: files && Object.keys(files).length > 0 ? files : undefined,
  }
}

export function normalizeRemoteScene(scene: WhiteboardScenePayload): WhiteboardScenePayload {
  const files = scene.files ? pickReferencedFiles(scene.elements, scene.files) : undefined
  const elements = markSavedImageElements(restoreSceneElements(scene.elements), files)
  return {
    ...scene,
    elements,
    files: files && Object.keys(files).length > 0 ? files : undefined,
  }
}
