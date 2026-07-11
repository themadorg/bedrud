/** Client for experimental WebXDC room APIs (uses shared api client → Bearer JWT). */

import { ApiError, api } from '#/lib/api'

export type WebxdcPublicConfig = {
  enabled: boolean
  experimental: boolean
  baseDomain?: string
  sendUpdateMaxSize?: number
  sendUpdateIntervalMs?: number
  /** Admin/config: show gallery UI from Apps button. */
  galleryEnabled?: boolean
  /** local | remote | both | semi-remote */
  gallerySource?: string
  /** Remote catalog JSON URL (server fetches; never embedded as HTML). */
  galleryRemoteUrl?: string
  /** Admin-uploaded packages shared across all rooms. */
  instanceCatalogEnabled?: boolean
}

export type WebxdcGalleryEntry = {
  id: string
  name: string
  description?: string
  category?: string
  iconUrl?: string
  /** True when a server-side raster icon sidecar exists (instance/room packages). */
  hasIcon?: boolean
  xdcUrl?: string
  /** Set for instance-catalog packages already stored on this server. */
  packageId?: string
  sourceCodeUrl?: string
  origin: string
}

export async function listWebxdcGallery(): Promise<{
  entries: WebxdcGalleryEntry[]
  source?: string
  warning?: string
}> {
  try {
    return await api.get<{ entries: WebxdcGalleryEntry[]; source?: string; warning?: string }>('/api/webxdc/gallery')
  } catch (e) {
    rethrow(e)
  }
}

/** Server downloads + validates .xdc then stores as a room package (same path as upload). */
export async function importWebxdcGalleryApp(
  roomId: string,
  body: { xdcUrl: string; name?: string },
): Promise<WebxdcPackage> {
  try {
    return await api.post<WebxdcPackage>(`/api/rooms/${roomId}/webxdc/gallery/import`, body)
  } catch (e) {
    rethrow(e)
  }
}

export type WebxdcPackage = {
  id: string
  roomId: string
  name: string
  sizeBytes: number
  contentHash: string
  sourceCodeUrl?: string
  iconPath?: string
  createdAt?: string
}

export type WebxdcInstance = {
  id: string
  roomId: string
  packageId: string
  document?: string
  summary?: string
  lastInfo?: string
  package?: WebxdcPackage
}

export type WebxdcStartResponse = {
  id: string
  roomId: string
  packageId: string
  name: string
  iframeOrigin: string
  iframeUrl: string
  ticket: string
  expiresAt: string
  /** Opaque per-app address (HMAC); different per instance for same user. */
  selfAddr?: string
  selfName?: string
  experimental: boolean
}

export type WebxdcTicketResponse = {
  ticket: string
  iframeUrl: string
  iframeOrigin: string
  expiresAt: string
  selfAddr?: string
  selfName?: string
}

function rethrow(err: unknown): never {
  if (err instanceof ApiError) {
    throw new Error(err.message || `Request failed (${err.status})`)
  }
  throw err instanceof Error ? err : new Error(String(err))
}

export async function fetchWebxdcConfig(): Promise<WebxdcPublicConfig> {
  try {
    const data = await api.get<{ webxdc: WebxdcPublicConfig }>('/api/webxdc/config')
    return data.webxdc
  } catch (e) {
    rethrow(e)
  }
}

export async function listWebxdcPackages(roomId: string): Promise<WebxdcPackage[]> {
  try {
    const data = await api.get<{ packages: WebxdcPackage[] }>(`/api/rooms/${roomId}/webxdc/packages`)
    return data.packages ?? []
  } catch (e) {
    rethrow(e)
  }
}

export async function uploadWebxdcPackage(roomId: string, file: File): Promise<WebxdcPackage> {
  try {
    const fd = new FormData()
    fd.append('file', file)
    // FormData body: shared api client skips Content-Type so the browser sets multipart boundary,
    // and attaches Authorization: Bearer (required by RequireBearerForMutations).
    return await api.post<WebxdcPackage>(`/api/rooms/${roomId}/webxdc/packages`, fd)
  } catch (e) {
    rethrow(e)
  }
}

export async function listWebxdcInstances(roomId: string): Promise<WebxdcInstance[]> {
  try {
    const data = await api.get<{ instances: WebxdcInstance[] }>(`/api/rooms/${roomId}/webxdc/instances`)
    return data.instances ?? []
  } catch (e) {
    rethrow(e)
  }
}

export async function startWebxdcInstance(roomId: string, packageId: string): Promise<WebxdcStartResponse> {
  try {
    return await api.post<WebxdcStartResponse>(`/api/rooms/${roomId}/webxdc/instances`, { packageId })
  } catch (e) {
    rethrow(e)
  }
}

export async function mintWebxdcTicket(roomId: string, instanceId: string): Promise<WebxdcTicketResponse> {
  try {
    return await api.post<WebxdcTicketResponse>(`/api/rooms/${roomId}/webxdc/instances/${instanceId}/ticket`, {})
  } catch (e) {
    rethrow(e)
  }
}

export async function closeWebxdcInstance(roomId: string, instanceId: string): Promise<void> {
  try {
    await api.post(`/api/rooms/${roomId}/webxdc/instances/${instanceId}/close`, {})
  } catch (e) {
    rethrow(e)
  }
}

export async function postWebxdcUpdate(
  roomId: string,
  instanceId: string,
  update: unknown,
): Promise<{ serial: number; maxSerial: number; update: unknown; nudge?: unknown }> {
  try {
    return await api.post<{ serial: number; maxSerial: number; update: unknown; nudge?: unknown }>(
      `/api/rooms/${roomId}/webxdc/instances/${instanceId}/updates`,
      update,
    )
  } catch (e) {
    rethrow(e)
  }
}

export async function listWebxdcUpdates(
  roomId: string,
  instanceId: string,
  after: number,
): Promise<{ updates: Array<Record<string, unknown>>; maxSerial: number }> {
  try {
    return await api.get<{ updates: Array<Record<string, unknown>>; maxSerial: number }>(
      `/api/rooms/${roomId}/webxdc/instances/${instanceId}/updates?after=${after}`,
    )
  } catch (e) {
    rethrow(e)
  }
}
