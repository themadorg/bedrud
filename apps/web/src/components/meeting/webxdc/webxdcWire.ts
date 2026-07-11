import { validateSendUpdate, type WebxdcSendUpdate } from './webxdcUpdate'
import { WEBXDC_SEND_UPDATE_MAX_SIZE } from './webxdcConstants'

export type WebxdcStatusWire = {
  v: 1
  kind: 'status'
  appId: string
  senderHint?: string
  serial: number
  ts: number
  update: WebxdcSendUpdate
}

export type WebxdcControlWire = {
  v: 1
  kind: 'control'
  appId: string
  action: 'close' | 'requestSnapshot' | 'snapshot'
  snapshot?: WebxdcStatusWire[]
}

export type WebxdcWire = WebxdcStatusWire | WebxdcControlWire

function isNonEmptyString(x: unknown): x is string {
  return typeof x === 'string' && x.length > 0
}

export function parseWebxdcWire(
  raw: unknown,
  maxUpdateSize: number = WEBXDC_SEND_UPDATE_MAX_SIZE,
): WebxdcWire | null {
  if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) return null
  const o = raw as Record<string, unknown>
  if (o.v !== 1) return null
  if (!isNonEmptyString(o.appId)) return null

  if (o.kind === 'status') {
    if (typeof o.serial !== 'number' || !Number.isFinite(o.serial) || o.serial < 1) return null
    if (typeof o.ts !== 'number' || !Number.isFinite(o.ts)) return null
    const validated = validateSendUpdate(o.update, maxUpdateSize)
    if (!validated.ok) return null
    const wire: WebxdcStatusWire = {
      v: 1,
      kind: 'status',
      appId: o.appId,
      serial: o.serial,
      ts: o.ts,
      update: validated.update,
    }
    if (typeof o.senderHint === 'string') wire.senderHint = o.senderHint
    return wire
  }

  if (o.kind === 'control') {
    if (o.action !== 'close' && o.action !== 'requestSnapshot' && o.action !== 'snapshot') {
      return null
    }
    const wire: WebxdcControlWire = {
      v: 1,
      kind: 'control',
      appId: o.appId,
      action: o.action,
    }
    if (o.action === 'snapshot' && Array.isArray(o.snapshot)) {
      if (o.snapshot.length > 500) return null
      const snaps: WebxdcStatusWire[] = []
      for (const item of o.snapshot) {
        const parsed = parseWebxdcWire(item, maxUpdateSize)
        if (!parsed || parsed.kind !== 'status') return null
        snaps.push(parsed)
      }
      wire.snapshot = snaps
    }
    return wire
  }

  return null
}

export function encodeWebxdcWire(wire: WebxdcWire): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(wire))
}

export function decodeWebxdcWire(
  data: Uint8Array,
  maxUpdateSize: number = WEBXDC_SEND_UPDATE_MAX_SIZE,
): WebxdcWire | null {
  if (data.byteLength > maxUpdateSize + 4096) return null
  try {
    const text = new TextDecoder().decode(data)
    return parseWebxdcWire(JSON.parse(text) as unknown, maxUpdateSize)
  } catch {
    return null
  }
}
