import { ConnectionState, Room, RoomEvent, type RoomConnectOptions, type RoomOptions } from 'livekit-client'
import { useEffect, useState } from 'react'

type EngineWithDataChannels = {
  reliableDC?: RTCDataChannel
  lossyDC?: RTCDataChannel
  reliableDCSub?: RTCDataChannel
  lossyDCSub?: RTCDataChannel
  dataSubscriberReadyState?: RTCDataChannelState
  pcManager?: {
    mode?: string
    subscriber?: unknown
  }
}

/** Custom RTC topic for Bedrud chat wire payloads (polls, reactions, chunks). */
export const MEETING_CHAT_TOPIC = 'chat'

/** Signaling WebSocket is open (metadata updates are safe). */
export function isRoomSignalingReady(room: Room): boolean {
  if (room.state === ConnectionState.Disconnected || room.state === ConnectionState.Connecting) {
    return false
  }
  try {
    const ws = room.engine.client.ws
    return ws != null && ws.readyState === WebSocket.OPEN
  } catch {
    return false
  }
}

function hasSubscriberPeerConnection(room: Room): boolean {
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    return engine.pcManager?.subscriber != null
  } catch {
    return false
  }
}

function hasOpenPublisherDataChannel(room: Room): boolean {
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    // Outbound chat / stage sync use the publisher reliable channel.
    return engine.reliableDC?.readyState === 'open'
  } catch {
    return false
  }
}

function hasOpenSubscriberDataChannel(room: Room): boolean {
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    const subState = engine.reliableDCSub?.readyState ?? engine.dataSubscriberReadyState
    return subState === 'open'
  } catch {
    return false
  }
}

/** Peer connection + both publisher (send) and subscriber (receive) data channels when applicable. */
export function isRoomPublishReady(room: Room): boolean {
  if (room.state !== ConnectionState.Connected) return false
  try {
    if (!room.engine.verifyTransport() || !hasOpenPublisherDataChannel(room)) return false
    // publisher-only (default singlePeerConnection): send + receive on publisher reliable DC.
    if (isPublisherOnlyRoom(room)) return true
    if (hasSubscriberPeerConnection(room) && !hasOpenSubscriberDataChannel(room)) return false
    return true
  } catch {
    return false
  }
}

/** @deprecated Prefer {@link isRoomPublishReady} before publishData / data-channel sends. */
export function isRoomConnected(room: Room): boolean {
  return isRoomPublishReady(room)
}

export function isPublishUnavailableError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err)
  const code = err && typeof err === 'object' && 'code' in err ? (err as { code: unknown }).code : null
  return (
    message.includes('PC manager is closed') ||
    message.includes('cannot publish') ||
    message.includes('not connected') ||
    message.includes('could not establish') ||
    code === 12
  )
}

/** Wait until publisher data channels are open (chat / whiteboard / stage sync). */
export async function waitForRoomPublishReady(room: Room, timeoutMs = 45_000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (isRoomPublishReady(room)) return true
    await new Promise((resolve) => window.setTimeout(resolve, 150))
  }
  return false
}

export function isLocalLiveKitHostname(hostname: string): boolean {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]'
}

function isPublisherOnlyRoom(room: Room): boolean {
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    return engine.pcManager?.mode === 'publisher-only'
  } catch {
    return false
  }
}

/** Dual peer connections — required for reliable user data on some remote LiveKit builds. */
export function livekitRoomOptionsForUrl(livekitUrl: string): RoomOptions | undefined {
  const hostname = livekitHostnameFromUrl(livekitUrl)
  if (!hostname || isLocalLiveKitHostname(hostname)) return undefined
  return { singlePeerConnection: false }
}

/** Remote debug sets VITE_LIVEKIT_ICE_RELAY=1 — TURN/TLS relay via port 5349 (not Traefik :443). */
function shouldForceLiveKitIceRelay(): boolean {
  return import.meta.env.VITE_LIVEKIT_ICE_RELAY === '1'
}

function normalizeIceServerUrls(iceServers: RTCIceServer[]): RTCIceServer[] {
  return iceServers.map((server) => {
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls]
    return {
      ...server,
      urls: urls.map((url) => (typeof url === 'string' ? normalizeTurnsTlsPort(url) : url)),
    }
  })
}

/** LiveKit may advertise turns:host:443 (HTTPS entrypoint); embedded TURN TLS listens on 5349. */
export function normalizeTurnsTlsPort(url: string): string {
  return url.replace(/^turns:([^:/?]+):443(?=[/?]|$)/, 'turns:$1:5349')
}

/** Parse hostname from a LiveKit server URL (ws/wss or bare host:port). */
export function livekitHostnameFromUrl(livekitUrl: string): string | null {
  const trimmed = livekitUrl.trim()
  if (!trimmed) return null
  try {
    const withScheme = trimmed.includes('://') ? trimmed : `wss://${trimmed}`
    return new URL(withScheme).hostname
  } catch {
    return null
  }
}

let remoteTransportPatchInstalled = false

/** Keep only TURN/TLS (TCP) — UDP TURN relay breaks SCTP data channels through VPN/TUN. */
export function filterIceServersToTurnsTls(iceServers: RTCIceServer[]): RTCIceServer[] {
  const filtered = iceServers
    .map((server) => {
      const urls = (Array.isArray(server.urls) ? server.urls : [server.urls])
        .filter((url): url is string => typeof url === 'string' && url.startsWith('turns:'))
        .map(normalizeTurnsTlsPort)
      return urls.length > 0 ? { ...server, urls } : null
    })
    .filter((server): server is RTCIceServer => server != null)
  return filtered.length > 0 ? filtered : iceServers
}

/**
 * Patch LiveKit engine to prefer TURN/TLS when relay mode is requested.
 * SCTP (chat) fails over TURN/UDP through VPN TUN interfaces even when media works.
 */
export function ensureRemoteLiveKitTransportPatch(): void {
  if (remoteTransportPatchInstalled || typeof window === 'undefined') return
  remoteTransportPatchInstalled = true

  const engineProto = Object.getPrototypeOf(new Room().engine) as {
    makeRTCConfiguration?: (serverResponse: unknown) => RTCConfiguration
    rtcConfig?: RTCConfiguration
  }
  const original = engineProto.makeRTCConfiguration
  if (!original) return

  engineProto.makeRTCConfiguration = function (
    this: { rtcConfig?: RTCConfiguration },
    serverResponse: unknown,
  ) {
    const config = original.call(this, serverResponse) as RTCConfiguration
    const forceRelay =
      config.iceTransportPolicy === 'relay' || this.rtcConfig?.iceTransportPolicy === 'relay'
    if (config.iceServers?.length) {
      config.iceServers = forceRelay
        ? filterIceServersToTurnsTls(config.iceServers)
        : normalizeIceServerUrls(config.iceServers)
    }
    if (!forceRelay) {
      return config
    }
    config.iceTransportPolicy = 'relay'
    if (import.meta.env.DEV && config.iceServers?.length) {
      console.log('[livekit-transport] using TURNS/TCP ICE servers', config.iceServers)
    }
    return config
  }
}

function relayRtcConnectOptions(): RoomConnectOptions {
  return { rtcConfig: { iceTransportPolicy: 'relay' }, peerConnectionTimeout: 45_000 }
}

/**
 * Remote LiveKit: prefer direct P2P first (chat shares the same peer connection).
 * Pass preferRelay=true after P2P fails to open data channels (SCTP over UDP is flaky on some NATs).
 */
export function livekitConnectOptionsForHost(
  hostname: string,
  preferRelay = false,
): RoomConnectOptions | undefined {
  if (isLocalLiveKitHostname(hostname)) return undefined
  ensureRemoteLiveKitTransportPatch()
  if (preferRelay || shouldForceLiveKitIceRelay()) {
    return relayRtcConnectOptions()
  }
  return { peerConnectionTimeout: 45_000 }
}

/** Prefer livekitHost from join response over window.location (user may open UI via localhost tunnel). */
export function livekitConnectOptionsForUrl(
  livekitUrl: string,
  preferRelay = false,
): RoomConnectOptions | undefined {
  const hostname = livekitHostnameFromUrl(livekitUrl)
  if (!hostname) return undefined
  return livekitConnectOptionsForHost(hostname, preferRelay)
}

export function isLiveKitRelayConnectOptions(connectOptions?: RoomConnectOptions): boolean {
  return connectOptions?.rtcConfig?.iceTransportPolicy === 'relay'
}

/** Poll whether publisher data channels are open (required for chat). */
export function useRoomPublishReady(room: Room | undefined, enabled: boolean): boolean {
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (!enabled || !room) {
      setReady(false)
      return
    }

    let cancelled = false
    const refresh = () => {
      if (!cancelled) setReady(isRoomPublishReady(room))
    }

    const onConnected = () => {
      refresh()
      void waitForRoomPublishReady(room, 45_000).then(() => {
        if (!cancelled) refresh()
      })
    }

    room.on(RoomEvent.Connected, onConnected)
    room.on(RoomEvent.Reconnected, onConnected)
    room.on(RoomEvent.ConnectionStateChanged, refresh)
    room.on(RoomEvent.DCBufferStatusChanged, refresh)
    if (room.state === ConnectionState.Connected) onConnected()
    else refresh()

    const interval = window.setInterval(refresh, 500)

    return () => {
      cancelled = true
      room.off(RoomEvent.Connected, onConnected)
      room.off(RoomEvent.Reconnected, onConnected)
      room.off(RoomEvent.ConnectionStateChanged, refresh)
      room.off(RoomEvent.DCBufferStatusChanged, refresh)
      window.clearInterval(interval)
    }
  }, [enabled, room])

  return ready
}

export type LiveKitPublishDiagnostics = {
  roomState: ConnectionState
  signalingOpen: boolean
  reliableDcState?: RTCDataChannelState
  reliableDcSubState?: RTCDataChannelState
  lossyDcState?: RTCDataChannelState
  publishReady: boolean
  pcMode?: string
  iceTransportPolicy?: RTCIceTransportPolicy
}

export function getLiveKitPublishDiagnostics(
  room: Room,
  connectOptions?: { rtcConfig?: RTCConfiguration },
): LiveKitPublishDiagnostics {
  const engine = room.engine as unknown as EngineWithDataChannels
  return {
    roomState: room.state,
    signalingOpen: isRoomSignalingReady(room),
    reliableDcState: engine.reliableDC?.readyState,
    reliableDcSubState: engine.reliableDCSub?.readyState ?? engine.dataSubscriberReadyState,
    lossyDcState: engine.lossyDC?.readyState,
    publishReady: isRoomPublishReady(room),
    pcMode: engine.pcManager?.mode,
    iceTransportPolicy: connectOptions?.rtcConfig?.iceTransportPolicy,
  }
}