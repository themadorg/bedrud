import { WEBXDC_SEND_UPDATE_MAX_SIZE } from './webxdcConstants'
import { isSafeRelativeWebxdcHref } from './webxdcHref'

export type WebxdcSendUpdate = {
  payload: unknown
  info?: string
  href?: string
  document?: string
  summary?: string
  notify?: Record<string, string>
}

export type ValidateSendUpdateResult =
  | { ok: true; update: WebxdcSendUpdate; serialized: string; byteLength: number }
  | { ok: false; reason: string }

function isJsonPayload(payload: unknown): boolean {
  if (payload === undefined) return false
  if (payload === null) return true
  const t = typeof payload
  if (t === 'string' || t === 'number' || t === 'boolean') return true
  if (Array.isArray(payload)) return true
  if (t === 'object') {
    // Reject binary-ish types that apps must base64 themselves
    if (typeof ArrayBuffer !== 'undefined' && payload instanceof ArrayBuffer) return false
    if (typeof Blob !== 'undefined' && payload instanceof Blob) return false
    return true
  }
  return false
}

/**
 * Validate an app sendUpdate body before host accepts it.
 * Size is measured on JSON serialization of the update object (official max).
 */
export function validateSendUpdate(
  raw: unknown,
  maxSize: number = WEBXDC_SEND_UPDATE_MAX_SIZE,
): ValidateSendUpdateResult {
  if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) {
    return { ok: false, reason: 'update must be an object' }
  }
  const obj = raw as Record<string, unknown>
  if (!('payload' in obj)) {
    return { ok: false, reason: 'payload is required' }
  }
  if (!isJsonPayload(obj.payload)) {
    return { ok: false, reason: 'payload must be JSON-serializable (not undefined/binary)' }
  }

  const update: WebxdcSendUpdate = { payload: obj.payload }

  if (obj.info !== undefined) {
    if (typeof obj.info !== 'string') return { ok: false, reason: 'info must be string' }
    // Spec suggests ~50 chars; OpenArena multiplayer lines are longer — keep chat-safe.
    update.info = obj.info.replace(/[\r\n]+/g, ' ').slice(0, 200)
  }
  if (obj.document !== undefined) {
    if (typeof obj.document !== 'string') return { ok: false, reason: 'document must be string' }
    update.document = obj.document.replace(/[\r\n]+/g, ' ').slice(0, 20)
  }
  if (obj.summary !== undefined) {
    if (typeof obj.summary !== 'string') return { ok: false, reason: 'summary must be string' }
    update.summary = obj.summary.replace(/[\r\n]+/g, ' ').slice(0, 20)
  }
  if (obj.href !== undefined) {
    if (typeof obj.href !== 'string') return { ok: false, reason: 'href must be string' }
    if (!isSafeRelativeWebxdcHref(obj.href)) {
      return { ok: false, reason: 'href must be relative' }
    }
    update.href = obj.href
  }
  if (obj.notify !== undefined) {
    if (obj.notify === null || typeof obj.notify !== 'object' || Array.isArray(obj.notify)) {
      return { ok: false, reason: 'notify must be a string map' }
    }
    const notify: Record<string, string> = {}
    for (const [k, v] of Object.entries(obj.notify as Record<string, unknown>)) {
      if (typeof v !== 'string') return { ok: false, reason: 'notify values must be strings' }
      notify[k] = v
    }
    update.notify = notify
  }

  let serialized: string
  try {
    serialized = JSON.stringify(update)
  } catch {
    return { ok: false, reason: 'update is not JSON-serializable' }
  }
  const byteLength = new TextEncoder().encode(serialized).length
  if (byteLength > maxSize) {
    return { ok: false, reason: `update exceeds sendUpdateMaxSize (${byteLength} > ${maxSize})` }
  }
  return { ok: true, update, serialized, byteLength }
}
