import { useRoomContext } from '@livekit/components-react'
import { RoomEvent, type Room } from 'livekit-client'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { isRoomPublishReady, prepareRoomForDataPublish } from '#/lib/livekit-publish'
import {
  normalizeChatAttachment,
  useMeetingChatContext,
  type ChatAttachment,
} from '@/components/meeting/MeetingContext'
import { listWebxdcUpdates, postWebxdcUpdate } from './webxdcApi'
import {
  WEBXDC_POSTMESSAGE_CHANNEL,
  WEBXDC_REALTIME_MAX_SIZE,
  WEBXDC_REALTIME_TOPIC,
  WEBXDC_SEND_UPDATE_INTERVAL_MS,
  WEBXDC_SEND_UPDATE_MAX_SIZE,
  WEBXDC_STATUS_TOPIC,
} from './webxdcConstants'
import { parseWebxdcIframeMessage } from './webxdcHostMessage'
import { WebxdcSendUpdateRateLimiter } from './webxdcRateLimit'
import { deriveWebxdcSelfAddrKey } from './webxdcSelfAddr'
import type { WebxdcSendUpdate } from './webxdcUpdate'
import { decodeWebxdcWire, encodeWebxdcWire } from './webxdcWire'

export type WebxdcHostOpts = {
  roomId: string
  instanceId: string
  iframeOrigin: string
  selfAddr?: string
  selfName: string
  selfAvatarUrl?: string
  userId: string
  sendUpdateIntervalMs?: number
  sendUpdateMaxSize?: number
  onChrome?: (meta: { document?: string; summary?: string; info?: string }) => void
}

/**
 * Compact binary realtime packet (LiveKit data size budget ~15KiB).
 * Layout: magic "BXRT" | u8 appIdLen | appId utf8 | payload bytes
 * Falls back to legacy JSON {channel,type,appId,data:number[]} for older peers.
 */
const RT_MAGIC = new TextEncoder().encode('BXRT')

function encodeRtPacket(instanceId: string, data: number[]): Uint8Array {
  const idBytes = new TextEncoder().encode(instanceId)
  if (idBytes.length > 255) throw new Error('instanceId too long')
  const payload = new Uint8Array(data.length)
  for (let i = 0; i < data.length; i++) payload[i] = data[i]! & 0xff
  const out = new Uint8Array(4 + 1 + idBytes.length + payload.length)
  out.set(RT_MAGIC, 0)
  out[4] = idBytes.length
  out.set(idBytes, 5)
  out.set(payload, 5 + idBytes.length)
  return out
}

function parseRtPacket(payload: Uint8Array, boundAppId: string): number[] | null {
  // Binary format
  if (
    payload.length >= 5 &&
    payload[0] === RT_MAGIC[0] &&
    payload[1] === RT_MAGIC[1] &&
    payload[2] === RT_MAGIC[2] &&
    payload[3] === RT_MAGIC[3]
  ) {
    const idLen = payload[4]!
    if (payload.length < 5 + idLen) return null
    const appId = new TextDecoder().decode(payload.subarray(5, 5 + idLen))
    if (appId !== boundAppId) return null
    const body = payload.subarray(5 + idLen)
    if (body.length > WEBXDC_REALTIME_MAX_SIZE) return null
    return Array.from(body)
  }
  // Legacy JSON (number array) — keep for short transition
  try {
    const raw = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
    if (raw.channel !== WEBXDC_POSTMESSAGE_CHANNEL || raw.type !== 'rt') return null
    if (raw.appId !== boundAppId || !Array.isArray(raw.data)) return null
    if (raw.data.length > WEBXDC_REALTIME_MAX_SIZE) return null
    const out: number[] = []
    for (const n of raw.data) {
      if (typeof n !== 'number' || !Number.isInteger(n) || n < 0 || n > 255) return null
      out.push(n)
    }
    return out
  } catch {
    return null
  }
}

function base64ToBlob(base64: string, mime: string): Blob {
  const bin = atob(base64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return new Blob([bytes], { type: mime || 'application/octet-stream' })
}

/**
 * Parent-side bridge (Desktop host patterns adapted to postMessage):
 * - init identity on ready / iframe load
 * - status: iframe pulls via getUpdates; parent only nudges
 * - realtime over LiveKit webxdc-rt
 * - sendToChat → meeting chat with confirm
 */
export function useWebxdcHost(
  iframeRef: React.RefObject<HTMLIFrameElement | null>,
  opts: WebxdcHostOpts | null,
) {
  const room = useRoomContext()
  const { appendSystemMessage, sendChat } = useMeetingChatContext()
  const rate = useRef(
    new WebxdcSendUpdateRateLimiter(opts?.sendUpdateIntervalMs ?? WEBXDC_SEND_UPDATE_INTERVAL_MS),
  )
  const rtJoined = useRef(false)
  /** Queue realtime packets until LiveKit publish is ready (OpenArena joins early). */
  const rtQueue = useRef<number[][]>([])
  /** Queue status updates when sendUpdateInterval blocks (spec: delay, never drop). */
  const statusQueue = useRef<WebxdcSendUpdate[]>([])
  const statusFlushTimer = useRef<number | null>(null)
  const initedRef = useRef(false)
  /** Track instance so we only clear rt state on real app switch, not effect re-runs. */
  const rtInstanceRef = useRef<string | null>(null)

  const postToIframe = useCallback(
    (msg: Record<string, unknown>) => {
      if (!opts) return
      const win = iframeRef.current?.contentWindow
      if (!win) return
      // Use '*' — a wrong targetOrigin does NOT throw; the browser silently drops
      // the message. That starved OpenArena of rtData (WHO_IS never arrived).
      // Source isolation is still enforced on the receive path (ev.source + originOk).
      try {
        win.postMessage({ channel: WEBXDC_POSTMESSAGE_CHANNEL, ...msg }, '*')
      } catch {
        /* ignore */
      }
    },
    [iframeRef, opts],
  )

  const buildInit = useCallback(() => {
    if (!opts) return null
    const selfAddr =
      (opts.selfAddr && opts.selfAddr.trim()) ||
      deriveWebxdcSelfAddrKey({
        roomId: opts.roomId,
        appId: opts.instanceId,
        userId: opts.userId,
      })
    return {
      type: 'init' as const,
      appId: opts.instanceId,
      selfAddr,
      selfName: opts.selfName || 'You',
      selfAvatarUrl: opts.selfAvatarUrl || '',
      sendUpdateInterval: opts.sendUpdateIntervalMs ?? WEBXDC_SEND_UPDATE_INTERVAL_MS,
      sendUpdateMaxSize: opts.sendUpdateMaxSize ?? WEBXDC_SEND_UPDATE_MAX_SIZE,
    }
  }, [opts])

  const sendInit = useCallback(() => {
    const init = buildInit()
    if (!init) return
    postToIframe(init)
    initedRef.current = true
  }, [buildInit, postToIframe])

  const handleGetUpdates = useCallback(
    async (requestId: string, after: number) => {
      if (!opts) return
      try {
        const { updates, maxSerial } = await listWebxdcUpdates(
          opts.roomId,
          opts.instanceId,
          after,
        )
        const shaped = updates.map((u) => {
          const serial = Number(u.serial ?? 0)
          return {
            ...u,
            serial,
            max_serial: maxSerial,
          }
        })
        postToIframe({
          type: 'updates',
          requestId,
          updates: shaped,
          maxSerial,
        })
      } catch {
        postToIframe({ type: 'updates', requestId, updates: [], maxSerial: after })
      }
    },
    [opts, postToIframe],
  )

  const nudgeStatus = useCallback(() => {
    postToIframe({ type: 'statusNudge' })
  }, [postToIframe])

  /** Spec `info` → meeting chat system line (Desktop: info is a chat message). Fan-out to peers. */
  const publishInfoToChat = useCallback(
    (info: string, actor: string) => {
      const message = info.trim()
      if (!message) return
      appendSystemMessage({ event: 'stage', actor, message })
      // LiveKit does not echo to self; peers receive on topic "system".
      try {
        if (!isRoomPublishReady(room as Room) || !prepareRoomForDataPublish(room as Room)) return
        const payload = new TextEncoder().encode(
          JSON.stringify({
            type: 'system',
            event: 'stage',
            actor,
            message,
            ts: Date.now(),
          }),
        )
        void room.localParticipant.publishData(payload, { reliable: true, topic: 'system' })
      } catch {
        /* ignore */
      }
    },
    [appendSystemMessage, room],
  )

  const handleNotify = useCallback(
    (update: WebxdcSendUpdate, selfAddr: string, selfName: string) => {
      if (update.info?.trim()) {
        publishInfoToChat(update.info, selfName)
      }
      if (!update.notify) return
      const text = update.notify[selfAddr] ?? update.notify['*']
      if (text?.trim()) {
        toast.message(selfName ? `${selfName}: ${text.trim()}` : text.trim(), { duration: 6000 })
        if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
          try {
            new Notification('Bedrud mini-app', { body: text.trim() })
          } catch {
            /* ignore */
          }
        }
      }
    },
    [publishInfoToChat],
  )

  /**
   * Persist update, local-echo to iframe (statusPush), LiveKit status fan-out, chat info.
   * Desktop: core stores then notifies open windows; own updates are included in the listener.
   */
  const processSendUpdate = useCallback(
    async (update: WebxdcSendUpdate, selfAddr: string, selfName: string) => {
      if (!opts) return
      try {
        const res = await postWebxdcUpdate(opts.roomId, opts.instanceId, update)
        const serial = Number(res.serial ?? 0)
        const maxSerial = Number(res.maxSerial ?? serial)
        opts.onChrome?.({
          document: update.document,
          summary: update.summary,
          info: update.info,
        })
        handleNotify(update, selfAddr, selfName)
        // Immediate local echo — fixes webxdc-test "current update missing" (4/0).
        if (serial > 0) {
          postToIframe({
            type: 'statusPush',
            update: {
              ...update,
              serial,
              max_serial: maxSerial,
            },
          })
        }
        // LiveKit fan-out so peers pull / receive without waiting for 2s poll.
        try {
          if (isRoomPublishReady(room as Room) && prepareRoomForDataPublish(room as Room) && serial > 0) {
            const wire = encodeWebxdcWire({
              v: 1,
              kind: 'status',
              appId: opts.instanceId,
              serial,
              ts: Date.now(),
              update,
            })
            void room.localParticipant.publishData(wire, {
              reliable: true,
              topic: WEBXDC_STATUS_TOPIC,
            })
          }
        } catch {
          /* ignore */
        }
        nudgeStatus()
      } catch {
        /* ignore network errors */
      }
    },
    [opts, handleNotify, postToIframe, room, nudgeStatus],
  )

  const flushStatusQueue = useCallback(
    (selfAddr: string, selfName: string) => {
      if (!opts) return
      if (statusFlushTimer.current != null) {
        window.clearTimeout(statusFlushTimer.current)
        statusFlushTimer.current = null
      }
      const next = statusQueue.current.shift()
      if (!next) return
      if (rate.current.tryTake(opts.instanceId)) {
        void processSendUpdate(next, selfAddr, selfName)
      } else {
        statusQueue.current.unshift(next)
      }
      if (statusQueue.current.length === 0) return
      const wait = Math.max(50, rate.current.msUntilReady(opts.instanceId))
      statusFlushTimer.current = window.setTimeout(() => {
        statusFlushTimer.current = null
        flushStatusQueue(selfAddr, selfName)
      }, wait)
    },
    [opts, processSendUpdate],
  )

  const enqueueOrSendUpdate = useCallback(
    (update: WebxdcSendUpdate, selfAddr: string, selfName: string) => {
      if (!opts) return
      const key = opts.instanceId
      if (statusQueue.current.length === 0 && rate.current.tryTake(key)) {
        void processSendUpdate(update, selfAddr, selfName)
        return
      }
      // Spec: host may delay faster callers — queue instead of silent drop.
      if (statusQueue.current.length >= 32) statusQueue.current.shift()
      statusQueue.current.push(update)
      if (statusFlushTimer.current != null) return
      const wait = Math.max(50, rate.current.msUntilReady(key))
      statusFlushTimer.current = window.setTimeout(() => {
        statusFlushTimer.current = null
        flushStatusQueue(selfAddr, selfName)
      }, wait)
    },
    [opts, processSendUpdate, flushStatusQueue],
  )

  const handleSendToChat = useCallback(
    async (
      requestId: string,
      text: string,
      file: { name: string; base64: string; mime?: string } | null,
    ) => {
      const preview =
        (text?.trim() ? text.trim().slice(0, 80) : '') +
        (file ? `${text?.trim() ? ' + ' : ''}file “${file.name}”` : '')
      const ok = window.confirm(
        `Send this from the mini-app to the meeting chat?\n\n${preview || '(empty)'}`,
      )
      if (!ok) {
        postToIframe({
          type: 'sendToChatResult',
          requestId,
          ok: false,
          error: 'User cancelled',
        })
        return
      }
      try {
        let attachments: ChatAttachment[] | undefined
        let message = text?.trim() || ''
        if (file) {
          const mime = file.mime || 'application/octet-stream'
          const blob = base64ToBlob(file.base64, mime)
          const form = new FormData()
          form.append('file', blob, file.name)
          // Allow non-image files through chat upload (default route is image-only).
          form.append('asFile', '1')
          const roomId = opts?.roomId
          if (!roomId) {
            postToIframe({
              type: 'sendToChatResult',
              requestId,
              ok: false,
              error: 'Room not ready',
            })
            return
          }
          // Server StoreNamed returns kind image|file with url — show real attachment in chat.
          const raw = await api.post<Record<string, unknown>>(`/api/room/${roomId}/chat/upload`, form)
          // Attach original name if server omitted it (image path).
          if (raw && typeof raw === 'object' && !raw.name && file.name) {
            raw.name = file.name
          }
          const attachment = normalizeChatAttachment(raw)
          if (!attachment) {
            postToIframe({
              type: 'sendToChatResult',
              requestId,
              ok: false,
              error: 'Upload did not return a usable file attachment',
            })
            return
          }
          // Prefer explicit file bubble for non-images.
          if (attachment.kind === 'image' && !mime.startsWith('image/')) {
            attachments = [
              {
                kind: 'file',
                url: attachment.url,
                mime: mime || attachment.mime,
                name: file.name,
                size: attachment.size || blob.size,
              },
            ]
          } else {
            attachments = [attachment]
          }
          // File-only: empty text is OK — bubble shows the file card.
        }
        if (!message && !attachments?.length) {
          postToIframe({
            type: 'sendToChatResult',
            requestId,
            ok: false,
            error: 'Nothing to send',
          })
          return
        }
        sendChat(message, attachments)
        postToIframe({ type: 'sendToChatResult', requestId, ok: true })
      } catch (e) {
        postToIframe({
          type: 'sendToChatResult',
          requestId,
          ok: false,
          error: e instanceof Error ? e.message : String(e),
        })
      }
    },
    [opts?.roomId, postToIframe, sendChat],
  )

  const publishRt = useCallback(
    async (data: number[]) => {
      if (!opts) return
      // OpenArena posts WHO_IS immediately; queue until LiveKit DC is open.
      if (!isRoomPublishReady(room as Room) || !prepareRoomForDataPublish(room as Room)) {
        if (rtQueue.current.length >= 64) rtQueue.current.shift()
        rtQueue.current.push(data)
        return
      }
      try {
        // Reliable: OpenArena signaling (WHO_IS_SERVER) cannot be lossy.
        await room.localParticipant.publishData(encodeRtPacket(opts.instanceId, data), {
          reliable: true,
          topic: WEBXDC_REALTIME_TOPIC,
        })
      } catch {
        if (rtQueue.current.length >= 64) rtQueue.current.shift()
        rtQueue.current.push(data)
      }
    },
    [opts, room],
  )

  // Flush queued realtime once publish is ready.
  useEffect(() => {
    if (!opts) return
    const id = window.setInterval(() => {
      if (!rtJoined.current) return
      if (!isRoomPublishReady(room as Room)) return
      const q = rtQueue.current
      if (q.length === 0) return
      rtQueue.current = []
      for (const data of q) void publishRt(data)
    }, 200)
    return () => window.clearInterval(id)
  }, [opts, room, publishRt])

  // LiveKit realtime + status fan-out
  useEffect(() => {
    if (!opts) return
    const onData = (
      payload: Uint8Array,
      participant?: { identity?: string },
      _kind?: unknown,
      topic?: string,
    ) => {
      if (participant?.identity === room.localParticipant.identity) return

      if (topic === WEBXDC_REALTIME_TOPIC) {
        const data = parseRtPacket(payload, opts.instanceId)
        if (!data) return
        postToIframe({ type: 'rtData', data })
        return
      }

      // Status updates (Desktop: core → open window statusUpdate → pull).
      // Chat `info` is fan-out separately on topic "system" (see publishInfoToChat).
      if (topic === WEBXDC_STATUS_TOPIC) {
        const wire = decodeWebxdcWire(payload)
        if (!wire || wire.kind !== 'status' || wire.appId !== opts.instanceId) return
        // Low-latency push into the app (deduped by serial in hostbridge).
        postToIframe({
          type: 'statusPush',
          update: {
            ...wire.update,
            serial: wire.serial,
            max_serial: wire.serial,
          },
        })
        opts.onChrome?.({
          document: wire.update.document,
          summary: wire.update.summary,
          info: wire.update.info,
        })
        nudgeStatus()
      }
    }
    room.on(RoomEvent.DataReceived, onData)
    return () => {
      room.off(RoomEvent.DataReceived, onData)
    }
  }, [opts, room, postToIframe, nudgeStatus])

  // Proactive init when iframe loads (Desktop setup is sync before app scripts).
  // Retry a few times — OpenArena reads webxdc.selfName early; late init → "?Setup Missing?".
  useEffect(() => {
    if (!opts) return
    const el = iframeRef.current
    if (!el) return
    const fire = () => {
      sendInit()
    }
    el.addEventListener('load', fire)
    fire()
    const t1 = window.setTimeout(fire, 50)
    const t2 = window.setTimeout(fire, 250)
    const t3 = window.setTimeout(fire, 1000)
    return () => {
      el.removeEventListener('load', fire)
      window.clearTimeout(t1)
      window.clearTimeout(t2)
      window.clearTimeout(t3)
    }
  }, [opts, iframeRef, sendInit])

  useEffect(() => {
    if (!opts) return
    initedRef.current = false
    // Only clear realtime state when the bound instance actually changes.
    // Re-running this effect (callback identity churn) must not drop rtJoined
    // mid-session or OpenArena WHO_IS goes silent again.
    if (rtInstanceRef.current !== opts.instanceId) {
      rtInstanceRef.current = opts.instanceId
      rtJoined.current = false
      rtQueue.current = []
      statusQueue.current = []
      if (statusFlushTimer.current != null) {
        window.clearTimeout(statusFlushTimer.current)
        statusFlushTimer.current = null
      }
    }
    rate.current = new WebxdcSendUpdateRateLimiter(
      opts.sendUpdateIntervalMs ?? WEBXDC_SEND_UPDATE_INTERVAL_MS,
    )

    const selfAddr =
      (opts.selfAddr && opts.selfAddr.trim()) ||
      deriveWebxdcSelfAddrKey({
        roomId: opts.roomId,
        appId: opts.instanceId,
        userId: opts.userId,
      })

    const originOk = (origin: string) => {
      if (origin === opts.iframeOrigin) return true
      // Tolerate localhost vs 127.0.0.1 and missing/extra default ports.
      try {
        const a = new URL(origin)
        const b = new URL(opts.iframeOrigin)
        const norm = (h: string) => (h === '127.0.0.1' ? 'localhost' : h)
        return (
          a.protocol === b.protocol &&
          norm(a.hostname) === norm(b.hostname) &&
          (a.port || (a.protocol === 'https:' ? '443' : '80')) ===
            (b.port || (b.protocol === 'https:' ? '443' : '80'))
        )
      } catch {
        return false
      }
    }

    const onMessage = async (ev: MessageEvent) => {
      const iframe = iframeRef.current
      if (!iframe?.contentWindow) return
      if (ev.source !== iframe.contentWindow) return
      if (!originOk(ev.origin)) return

      // Accept ready even if appId omitted on first tick (query not parsed yet).
      const raw = ev.data
      if (
        raw &&
        typeof raw === 'object' &&
        !Array.isArray(raw) &&
        (raw as Record<string, unknown>).channel === WEBXDC_POSTMESSAGE_CHANNEL &&
        (raw as Record<string, unknown>).type === 'ready'
      ) {
        sendInit()
        return
      }

      const msg = parseWebxdcIframeMessage(ev.data, opts.instanceId)
      if (!msg) return

      if (msg.type === 'ready') {
        sendInit()
        return
      }

      if (msg.type === 'getUpdates') {
        await handleGetUpdates(msg.requestId, msg.after)
        return
      }

      // Legacy setUpdateListener message — bridge no longer needs parent action
      // (pull is self-contained), but accept for older bridge builds.
      if (msg.type === 'setUpdateListener') {
        await handleGetUpdates(`legacy-${msg.serial}`, msg.serial)
        return
      }

      if (msg.type === 'sendUpdate') {
        enqueueOrSendUpdate(msg.update, selfAddr, opts.selfName)
        return
      }

      if (msg.type === 'rtJoin') {
        rtJoined.current = true
        return
      }
      if (msg.type === 'rtLeave') {
        rtJoined.current = false
        rtQueue.current = []
        return
      }
      if (msg.type === 'rtSend') {
        // Race: OpenArena joinRealtimeChannel + WHO_IS fire as soon as modules
        // load, often before this effect attaches. rtJoin is lost → all WHO_IS
        // packets were dropped and both peers forever host alone (numSends: 0).
        // Treat first rtSend as an implicit join (bound to this iframe source).
        if (!rtJoined.current) {
          rtJoined.current = true
        }
        await publishRt(msg.data)
        return
      }

      if (msg.type === 'sendToChat') {
        await handleSendToChat(msg.requestId, msg.text, msg.file)
        return
      }

      if (msg.type === 'openExternal') {
        const url = msg.url
        const allow = window.confirm(
          `This mini-app wants to open an external link.\n\n${url}\n\nExternal sites may track you. Open anyway?`,
        )
        if (allow) {
          window.open(url, '_blank', 'noopener,noreferrer')
        }
      }
    }

    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [
    opts,
    iframeRef,
    sendInit,
    handleGetUpdates,
    handleSendToChat,
    enqueueOrSendUpdate,
    publishRt,
  ])

  // Peer catch-up: nudge iframe to pull (same as Desktop statusUpdate events).
  useEffect(() => {
    if (!opts) return
    const id = window.setInterval(() => {
      nudgeStatus()
    }, 2000)
    return () => window.clearInterval(id)
  }, [opts, nudgeStatus])

  return { postToIframe, sendInit, nudgeStatus }
}
