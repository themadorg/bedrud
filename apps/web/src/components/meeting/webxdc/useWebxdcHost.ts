import { useRoomContext } from '@livekit/components-react'
import { RoomEvent, type Room } from 'livekit-client'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { isRoomPublishReady } from '#/lib/livekit-publish'
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
} from './webxdcConstants'
import { parseWebxdcIframeMessage } from './webxdcHostMessage'
import { WebxdcSendUpdateRateLimiter } from './webxdcRateLimit'
import { deriveWebxdcSelfAddrKey } from './webxdcSelfAddr'
import type { WebxdcSendUpdate } from './webxdcUpdate'

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

function encodeRtPacket(instanceId: string, data: number[]): Uint8Array {
  const json = JSON.stringify({
    channel: WEBXDC_POSTMESSAGE_CHANNEL,
    type: 'rt',
    appId: instanceId,
    data,
  })
  return new TextEncoder().encode(json)
}

function parseRtPacket(payload: Uint8Array, boundAppId: string): number[] | null {
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
  const initedRef = useRef(false)

  const postToIframe = useCallback(
    (msg: Record<string, unknown>) => {
      if (!opts) return
      const win = iframeRef.current?.contentWindow
      if (!win) return
      win.postMessage({ channel: WEBXDC_POSTMESSAGE_CHANNEL, ...msg }, opts.iframeOrigin)
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

  const handleNotify = useCallback(
    (update: WebxdcSendUpdate, selfAddr: string, selfName: string) => {
      if (update.info?.trim()) {
        appendSystemMessage({
          event: 'stage',
          actor: selfName,
          message: update.info.trim(),
        })
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
    [appendSystemMessage],
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

  // LiveKit realtime fan-out
  useEffect(() => {
    if (!opts) return
    const onData = (
      payload: Uint8Array,
      participant?: { identity?: string },
      _kind?: unknown,
      topic?: string,
    ) => {
      if (topic !== WEBXDC_REALTIME_TOPIC) return
      if (participant?.identity === room.localParticipant.identity) return
      const data = parseRtPacket(payload, opts.instanceId)
      if (!data) return
      postToIframe({ type: 'rtData', data })
    }
    room.on(RoomEvent.DataReceived, onData)
    return () => {
      room.off(RoomEvent.DataReceived, onData)
    }
  }, [opts, room, postToIframe])

  // Proactive init when iframe loads (Desktop setup is sync before app scripts).
  useEffect(() => {
    if (!opts) return
    const el = iframeRef.current
    if (!el) return
    const onLoad = () => {
      sendInit()
    }
    el.addEventListener('load', onLoad)
    // If already complete (cached), still init.
    try {
      if (el.contentDocument?.readyState === 'complete') sendInit()
    } catch {
      // cross-origin until load — ignore
    }
    return () => el.removeEventListener('load', onLoad)
  }, [opts, iframeRef, sendInit])

  useEffect(() => {
    if (!opts) return
    initedRef.current = false
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

    const onMessage = async (ev: MessageEvent) => {
      const iframe = iframeRef.current
      if (!iframe?.contentWindow) return
      if (ev.source !== iframe.contentWindow) return
      if (ev.origin !== opts.iframeOrigin) return

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
        if (!rate.current.tryTake(opts.instanceId)) return
        try {
          const res = await postWebxdcUpdate(opts.roomId, opts.instanceId, msg.update)
          opts.onChrome?.({
            document: msg.update.document,
            summary: msg.update.summary,
            info: msg.update.info,
          })
          handleNotify(msg.update, selfAddr, opts.selfName)
          // Desktop: core persists then nudges window to pull — including local echo.
          void res
          nudgeStatus()
        } catch {
          /* ignore */
        }
        return
      }

      if (msg.type === 'rtJoin') {
        rtJoined.current = true
        return
      }
      if (msg.type === 'rtLeave') {
        rtJoined.current = false
        return
      }
      if (msg.type === 'rtSend') {
        if (!rtJoined.current) return
        if (!isRoomPublishReady(room as Room)) return
        try {
          await room.localParticipant.publishData(encodeRtPacket(opts.instanceId, msg.data), {
            reliable: false,
            topic: WEBXDC_REALTIME_TOPIC,
          })
        } catch {
          /* ignore */
        }
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
    handleNotify,
    handleSendToChat,
    nudgeStatus,
    room,
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
