import { WEBXDC_POSTMESSAGE_CHANNEL } from './webxdcConstants'
import { validateSendUpdate, type WebxdcSendUpdate } from './webxdcUpdate'

export type WebxdcSendToChatFile = {
  name: string
  base64: string
  mime?: string
}

export type WebxdcIframeToHost =
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'sendUpdate'
      appId: string
      update: WebxdcSendUpdate
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'ready'
      appId: string
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'setUpdateListener'
      appId: string
      serial: number
    }
  | {
      /** Pull-based catch-up (Desktop-style getAllUpdates since last serial). */
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'getUpdates'
      appId: string
      requestId: string
      after: number
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'rtSend'
      appId: string
      data: number[]
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'rtJoin' | 'rtLeave'
      appId: string
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'sendToChat'
      appId: string
      requestId: string
      text: string
      file: WebxdcSendToChatFile | null
    }
  | {
      channel: typeof WEBXDC_POSTMESSAGE_CHANNEL
      type: 'openExternal'
      appId: string
      url: string
    }

/**
 * Parse untrusted postMessage data from an iframe.
 * Caller must still check event.source and event.origin.
 */
export function parseWebxdcIframeMessage(
  data: unknown,
  boundAppId: string,
): WebxdcIframeToHost | null {
  if (data === null || typeof data !== 'object' || Array.isArray(data)) return null
  const o = data as Record<string, unknown>
  if (o.channel !== WEBXDC_POSTMESSAGE_CHANNEL) return null

  // One iframe host is bound to one instance. Prefer explicit appId when present;
  // if missing (in-app navigations drop query params), fall back to boundAppId.
  // Reject only when an explicit appId conflicts with this host.
  if (typeof o.appId === 'string' && o.appId !== '' && o.appId !== boundAppId) {
    return null
  }
  const appId = typeof o.appId === 'string' && o.appId !== '' ? o.appId : boundAppId

  // ready may arrive before appId is known; still bind to this host instance.
  if (o.type === 'ready') {
    return {
      channel: WEBXDC_POSTMESSAGE_CHANNEL,
      type: 'ready',
      appId,
    }
  }

  switch (o.type) {
    case 'ready':
      return { channel: WEBXDC_POSTMESSAGE_CHANNEL, type: 'ready', appId }
    case 'setUpdateListener': {
      const serial = typeof o.serial === 'number' && Number.isFinite(o.serial) ? o.serial : 0
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'setUpdateListener',
        appId,
        serial,
      }
    }
    case 'getUpdates': {
      if (typeof o.requestId !== 'string' || !o.requestId) return null
      const after = typeof o.after === 'number' && Number.isFinite(o.after) ? o.after : 0
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'getUpdates',
        appId,
        requestId: o.requestId,
        after: Math.max(0, after),
      }
    }
    case 'sendUpdate': {
      const validated = validateSendUpdate(o.update)
      if (!validated.ok) return null
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'sendUpdate',
        appId,
        update: validated.update,
      }
    }
    case 'rtJoin':
    case 'rtLeave':
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: o.type,
        appId,
      }
    case 'rtSend': {
      if (!Array.isArray(o.data)) return null
      if (o.data.length > 128_000) return null
      const dataBytes: number[] = []
      for (const n of o.data) {
        if (typeof n !== 'number' || !Number.isInteger(n) || n < 0 || n > 255) return null
        dataBytes.push(n)
      }
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'rtSend',
        appId,
        data: dataBytes,
      }
    }
    case 'sendToChat': {
      if (typeof o.requestId !== 'string' || !o.requestId) return null
      const text = typeof o.text === 'string' ? o.text : ''
      let file: WebxdcSendToChatFile | null = null
      if (o.file != null && typeof o.file === 'object' && !Array.isArray(o.file)) {
        const f = o.file as Record<string, unknown>
        if (typeof f.name !== 'string' || !f.name) return null
        if (typeof f.base64 !== 'string') return null
        // Cap ~4 MiB base64 for meeting chat safety
        if (f.base64.length > 5_500_000) return null
        file = {
          name: f.name,
          base64: f.base64,
          mime: typeof f.mime === 'string' ? f.mime : undefined,
        }
      }
      if (!file && !text.trim()) return null
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'sendToChat',
        appId,
        requestId: o.requestId,
        text,
        file,
      }
    }
    case 'openExternal': {
      if (typeof o.url !== 'string' || !o.url.trim()) return null
      const url = o.url.trim()
      if (!/^https?:\/\//i.test(url) && !url.startsWith('//')) return null
      return {
        channel: WEBXDC_POSTMESSAGE_CHANNEL,
        type: 'openExternal',
        appId,
        url: url.startsWith('//') ? `https:${url}` : url,
      }
    }
    default:
      return null
  }
}

export function isTrustedWebxdcMessageEvent(
  event: { origin: string; source: MessageEventSource | null },
  expectedOrigin: string,
  expectedSource: MessageEventSource | null,
): boolean {
  if (expectedSource != null && event.source !== expectedSource) return false
  if (event.origin !== expectedOrigin) return false
  return true
}
