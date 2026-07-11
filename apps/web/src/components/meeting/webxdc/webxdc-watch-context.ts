import { createContext, useContext } from 'react'
import type { WebxdcStartResponse, WebxdcTicketResponse } from './webxdcApi'

export type WebxdcSession = {
  instanceId: string
  packageId: string
  name: string
  iframeUrl: string
  iframeOrigin: string
  ticket: string
  selfAddr?: string
  selfName?: string
}

export interface WebxdcWatchContextValue {
  session: WebxdcSession | null
  isHost: boolean
  busy: boolean
  error: string | null
  /** Upload .xdc, start instance, put on stage for everyone. */
  shareFile: (file: File) => Promise<void>
  /** Start an already-uploaded package and put on stage. */
  sharePackage: (packageId: string, name?: string) => Promise<void>
  /** Stop stage share (host only) and close local view. */
  stopShare: () => Promise<void>
  /** Close local iframe without clearing stage (non-host leave). */
  leaveLocal: () => void
}

export const WebxdcWatchContext = createContext<WebxdcWatchContextValue | null>(null)

export function useOptionalWebxdcWatch(): WebxdcWatchContextValue | null {
  return useContext(WebxdcWatchContext)
}

export function useWebxdcWatch(): WebxdcWatchContextValue {
  const ctx = useContext(WebxdcWatchContext)
  if (!ctx) throw new Error('useWebxdcWatch must be used inside WebxdcWatchProvider')
  return ctx
}

export function startResponseToSession(res: WebxdcStartResponse): WebxdcSession {
  return {
    instanceId: res.id,
    packageId: res.packageId,
    name: res.name,
    iframeUrl: res.iframeUrl,
    iframeOrigin: res.iframeOrigin,
    ticket: res.ticket,
    selfAddr: res.selfAddr,
    selfName: res.selfName,
  }
}

export function ticketResponseToSession(
  t: WebxdcTicketResponse,
  meta: { instanceId: string; packageId: string; name: string },
): WebxdcSession {
  return {
    instanceId: meta.instanceId,
    packageId: meta.packageId,
    name: meta.name,
    iframeUrl: t.iframeUrl,
    iframeOrigin: t.iframeOrigin,
    ticket: t.ticket,
    selfAddr: t.selfAddr,
    selfName: t.selfName,
  }
}
