import * as decoding from 'lib0/decoding'
import * as encoding from 'lib0/encoding'
import { type Room, RoomEvent } from 'livekit-client'
import { isPublishUnavailableError, isRoomConnected } from '#/lib/livekit-publish'
import * as syncProtocol from 'y-protocols/sync'
import * as Y from 'yjs'
import {
  applyYjsChunkPart,
  assembledYjsFromChunks,
  buildYjsWirePackets,
  createYjsChunkBuffer,
  ingestYjsChunkMeta,
  parseYjsWirePacket,
  pruneYjsChunkBuffers,
  WHITEBOARD_YJS_TOPIC,
} from './yjsWire'

const MESSAGE_SYNC = 0
const OUTBOUND_FLUSH_MS = 120
const CHUNK_SEND_GAP_MS = 8

export const LIVEKIT_YJS_ORIGIN = Symbol('livekit-yjs-provider')

export class LiveKitYjsProvider {
  readonly doc: Y.Doc
  synced = false

  private readonly room: Room
  private readonly topic: string
  private destroyed = false
  private readonly chunkBuffers = createYjsChunkBuffer()
  private readonly syncTimers: number[] = []
  private readonly pendingDocUpdates: Uint8Array[] = []
  private readonly publishQueue: Uint8Array[] = []
  private flushTimer: number | null = null
  private publishing = false
  private drainRetryTimer: number | null = null

  constructor(doc: Y.Doc, room: Room, topic = WHITEBOARD_YJS_TOPIC) {
    this.doc = doc
    this.room = room
    this.topic = topic

    doc.on('update', this.onDocUpdate)
    room.on(RoomEvent.DataReceived, this.onDataReceived)
    room.on(RoomEvent.Connected, this.onRoomConnected)
    room.on(RoomEvent.Reconnecting, this.onRoomReconnecting)
    room.on(RoomEvent.Reconnected, this.onRoomReconnected)
    room.on(RoomEvent.Disconnected, this.onRoomDisconnected)

    this.bootstrapSync()
  }

  private isRoomReady(): boolean {
    return !this.destroyed && isRoomConnected(this.room)
  }

  private bootstrapSync() {
    if (this.isRoomReady()) {
      this.requestSync()
    }
    for (const ms of [800, 2000, 4000]) {
      this.syncTimers.push(
        window.setTimeout(() => {
          if (this.isRoomReady()) this.requestSync()
        }, ms),
      )
    }
  }

  private onRoomConnected = () => {
    if (this.destroyed) return
    this.requestSync()
    void this.drainPublishQueue()
  }

  private onRoomReconnecting = () => {
    this.publishing = false
    this.clearDrainRetry()
  }

  private onRoomReconnected = () => {
    this.onRoomConnected()
  }

  private onRoomDisconnected = () => {
    this.publishing = false
    this.clearDrainRetry()
  }

  private clearDrainRetry() {
    if (this.drainRetryTimer != null) {
      window.clearTimeout(this.drainRetryTimer)
      this.drainRetryTimer = null
    }
  }

  private scheduleDrainRetry(delayMs = 400) {
    if (this.drainRetryTimer != null) return
    this.drainRetryTimer = window.setTimeout(() => {
      this.drainRetryTimer = null
      void this.drainPublishQueue()
    }, delayMs)
  }

  /** Push any batched Yjs updates immediately (e.g. on pointer up). */
  flush() {
    if (this.flushTimer != null) {
      window.clearTimeout(this.flushTimer)
      this.flushTimer = null
    }
    this.flushOutbound()
  }

  private onDocUpdate = (update: Uint8Array, origin: unknown) => {
    if (origin === LIVEKIT_YJS_ORIGIN || this.destroyed) return
    this.pendingDocUpdates.push(update)
    if (this.flushTimer != null) return
    this.flushTimer = window.setTimeout(() => {
      this.flushTimer = null
      this.flushOutbound()
    }, OUTBOUND_FLUSH_MS)
  }

  private flushOutbound() {
    if (this.pendingDocUpdates.length === 0 || this.destroyed) return

    const merged = Y.mergeUpdates(this.pendingDocUpdates)
    this.pendingDocUpdates.length = 0

    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, MESSAGE_SYNC)
    syncProtocol.writeUpdate(encoder, merged)
    this.enqueuePackets(buildYjsWirePackets(encoding.toUint8Array(encoder)))
  }

  private requestSync() {
    if (!this.isRoomReady()) return
    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, MESSAGE_SYNC)
    syncProtocol.writeSyncStep1(encoder, this.doc)
    this.enqueuePackets(buildYjsWirePackets(encoding.toUint8Array(encoder)))
  }

  private onDataReceived = (payload: Uint8Array, _participant: unknown, _kind: unknown, topic?: string) => {
    if (topic !== this.topic || this.destroyed) return

    try {
      const parsed = parseYjsWirePacket(payload)
      if (!parsed) return

      if (parsed.kind === 'meta') {
        ingestYjsChunkMeta(this.chunkBuffers, parsed.meta)
        pruneYjsChunkBuffers(this.chunkBuffers)
        return
      }

      if (parsed.kind === 'chunk') {
        const pending = this.chunkBuffers.get(parsed.chunk.id)
        if (!pending) return
        const done = applyYjsChunkPart(pending, parsed.chunk)
        pruneYjsChunkBuffers(this.chunkBuffers)
        if (!done) return
        this.chunkBuffers.delete(parsed.chunk.id)
        this.handleYjsMessage(assembledYjsFromChunks(done))
        return
      }

      this.handleYjsMessage(parsed.data)
    } catch (err) {
      if (import.meta.env.DEV) console.error('[LiveKitYjsProvider] failed to process packet:', err)
    }
  }

  private handleYjsMessage(data: Uint8Array) {
    const decoder = decoding.createDecoder(data)
    const messageType = decoding.readVarUint(decoder)
    if (messageType !== MESSAGE_SYNC) return

    const encoder = encoding.createEncoder()
    encoding.writeVarUint(encoder, MESSAGE_SYNC)
    const syncType = syncProtocol.readSyncMessage(decoder, encoder, this.doc, LIVEKIT_YJS_ORIGIN)
    if (syncType === syncProtocol.messageYjsSyncStep2) {
      this.synced = true
    }

    const reply = encoding.toUint8Array(encoder)
    if (reply.length > 1) {
      this.enqueuePackets(buildYjsWirePackets(reply))
    }
  }

  private enqueuePackets(packets: Uint8Array[]) {
    this.publishQueue.push(...packets)
    void this.drainPublishQueue()
  }

  private async drainPublishQueue() {
    if (this.publishing || this.destroyed || !this.isRoomReady()) return
    this.publishing = true

    while (this.publishQueue.length > 0 && !this.destroyed) {
      if (!this.isRoomReady()) break

      const packet = this.publishQueue.shift()
      if (!packet) break

      try {
        await this.room.localParticipant.publishData(packet, { reliable: true, topic: this.topic })
      } catch (err) {
        if (isPublishUnavailableError(err)) {
          this.publishQueue.unshift(packet)
          break
        }
        if (import.meta.env.DEV) {
          const detail = err instanceof Error ? err.message : JSON.stringify(err)
          console.error('[LiveKitYjsProvider] failed to publish packet:', detail)
        }
      }

      if (this.publishQueue.length > 0) {
        await new Promise((resolve) => window.setTimeout(resolve, CHUNK_SEND_GAP_MS))
      }
    }

    this.publishing = false
    if (this.publishQueue.length > 0 && this.isRoomReady()) {
      this.scheduleDrainRetry()
    }
  }

  destroy() {
    if (this.destroyed) return
    this.destroyed = true
    this.clearDrainRetry()
    if (this.flushTimer != null) window.clearTimeout(this.flushTimer)
    for (const timer of this.syncTimers) window.clearTimeout(timer)
    this.pendingDocUpdates.length = 0
    this.publishQueue.length = 0
    this.doc.off('update', this.onDocUpdate)
    this.room.off(RoomEvent.DataReceived, this.onDataReceived)
    this.room.off(RoomEvent.Connected, this.onRoomConnected)
    this.room.off(RoomEvent.Reconnecting, this.onRoomReconnecting)
    this.room.off(RoomEvent.Reconnected, this.onRoomReconnected)
    this.room.off(RoomEvent.Disconnected, this.onRoomDisconnected)
    this.chunkBuffers.clear()
  }
}

export function createWhiteboardYDoc(): Y.Doc {
  const doc = new Y.Doc()
  doc.getMap('elements')
  doc.getArray('order')
  doc.getMap('files')
  doc.getMap('settings')
  doc.getMap('locks')
  return doc
}