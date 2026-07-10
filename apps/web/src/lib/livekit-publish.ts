import { ConnectionState, Room, type RoomConnectOptions, RoomEvent, type RoomOptions } from 'livekit-client'
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
  } | null
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

/** True when the WebRTC stack is gone — retries will never help until reconnect. */
export function isRoomTransportDead(room: Room): boolean {
  if (room.state === ConnectionState.Disconnected) return true
  // Still establishing — not dead yet.
  if (room.state === ConnectionState.Connecting || room.state === ConnectionState.Reconnecting) {
    return false
  }
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    // LiveKit throws "PC manager is closed" when this is missing after teardown.
    // Only treat as dead once we expected to be connected.
    return room.state === ConnectionState.Connected && engine.pcManager == null
  } catch {
    return true
  }
}

/** Peer connection + both publisher (send) and subscriber (receive) data channels when applicable. */
export function isRoomPublishReady(room: Room): boolean {
  if (room.state !== ConnectionState.Connected) return false
  if (isRoomTransportDead(room) || isLiveKitEngineClosed(room)) return false
  try {
    const engine = room.engine as unknown as EngineWithDataChannels
    if (engine.pcManager == null) return false
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

/** Errors where publishData cannot succeed without a full transport rebuild. */
export function isFatalPublishError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err)
  const code = err && typeof err === 'object' && 'code' in err ? (err as { code: unknown }).code : null
  return message.includes('PC manager is closed') || message.includes('publisher is closed') || code === 12
}

type EngineWithPublisherPromise = EngineWithDataChannels & {
  publisherConnectionPromise?: Promise<void>
  _isClosed?: boolean
  isClosed?: boolean
}

/**
 * LiveKit caches the first ensureDataTransportConnected() result on
 * `publisherConnectionPromise`. If that first call races a disconnect (React Strict
 * Mode remount, brief PC teardown), every later publishData rethrows
 * "PC manager is closed" even after DCs look open again.
 *
 * When the reliable data channel is already open, force the promise to a resolved
 * state so ensurePublisherConnected short-circuits and dc.send() runs. Clearing to
 * undefined is NOT enough: re-running ensureDataTransportConnected can still throw
 * if pcManager flickers during renegotiation.
 */
export function resetLiveKitPublisherPromise(room: Room): void {
  try {
    const engine = room.engine as unknown as EngineWithPublisherPromise & {
      reliableDC?: { readyState?: string }
    }
    if (engine.publisherConnectionPromise != null) {
      void Promise.resolve(engine.publisherConnectionPromise).catch(() => {})
    }
    // Prefer "already ready" over "re-check" when DC is open.
    if (engine.reliableDC?.readyState === 'open' && engine.pcManager != null) {
      engine.publisherConnectionPromise = Promise.resolve()
    } else {
      engine.publisherConnectionPromise = undefined
    }
  } catch {
    /* ignore */
  }
}

/** Call immediately before publishData — returns false if we must not publish. */
export function prepareRoomForDataPublish(room: Room): boolean {
  if (room.state !== ConnectionState.Connected) return false
  if (isRoomTransportDead(room) || isLiveKitEngineClosed(room)) return false
  try {
    const engine = room.engine as unknown as EngineWithPublisherPromise & {
      reliableDC?: { readyState?: string }
    }
    if (engine.pcManager == null) return false
    if (engine.reliableDC?.readyState !== 'open') return false
    // Neutralize stuck rejected promise; mark publisher as connected.
    if (engine.publisherConnectionPromise != null) {
      void Promise.resolve(engine.publisherConnectionPromise).catch(() => {})
    }
    engine.publisherConnectionPromise = Promise.resolve()
    return true
  } catch {
    return false
  }
}

/**
 * Patch LiveKit RTCEngine.ensurePublisherConnected on the prototype used by this
 * Room instance. If the reliable DC is already open, skip re-validation entirely.
 */
export function installLiveKitPublisherPromiseFix(room?: Room): void {
  if (typeof window === 'undefined') return
  try {
    const probeEngine = room?.engine ?? new Room().engine
    const proto = Object.getPrototypeOf(probeEngine) as {
      ensurePublisherConnected?: (kind: unknown) => Promise<void>
      reliableDC?: { readyState?: string }
      pcManager?: unknown
      publisherConnectionPromise?: Promise<void>
    }
    const original = proto.ensurePublisherConnected
    if (!original || (original as { __bedrudPatched?: boolean }).__bedrudPatched) {
      if (room) resetLiveKitPublisherPromise(room)
      return
    }
    function ensurePublisherConnectedPatched(this: typeof proto, kind: unknown) {
      const eng = this as EngineWithPublisherPromise & {
        reliableDC?: { readyState?: string }
      }
      if (eng.reliableDC?.readyState === 'open' && eng.pcManager != null) {
        if (eng.publisherConnectionPromise != null) {
          void Promise.resolve(eng.publisherConnectionPromise).catch(() => {})
        }
        eng.publisherConnectionPromise = Promise.resolve()
        return Promise.resolve()
      }
      if (eng.publisherConnectionPromise != null) {
        void Promise.resolve(eng.publisherConnectionPromise).catch(() => {})
        eng.publisherConnectionPromise = undefined
      }
      return original!.call(this, kind)
    }
    ;(ensurePublisherConnectedPatched as { __bedrudPatched?: boolean }).__bedrudPatched = true
    proto.ensurePublisherConnected = ensurePublisherConnectedPatched
    if (room) resetLiveKitPublisherPromise(room)
  } catch {
    /* ignore — best effort */
  }
}

// Install as soon as this module loads in the browser (prototype of a probe Room).
if (typeof window !== 'undefined') {
  installLiveKitPublisherPromiseFix()
}

/** True when engine was fully closed (not just reconnecting). */
export function isLiveKitEngineClosed(room: Room): boolean {
  try {
    const engine = room.engine as unknown as EngineWithPublisherPromise
    return engine._isClosed === true || engine.isClosed === true
  } catch {
    return true
  }
}

/** Transient: may recover after ICE/DC open (or short reconnect). Not for closed PC. */
export function isPublishUnavailableError(err: unknown): boolean {
  if (isFatalPublishError(err)) return true
  const message = err instanceof Error ? err.message : String(err)
  return (
    message.includes('cannot publish') || message.includes('not connected') || message.includes('could not establish')
  )
}

/** Wait until publisher data channels are open (chat / whiteboard / stage sync). */
export async function waitForRoomPublishReady(room: Room, timeoutMs = 45_000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (isRoomPublishReady(room)) return true
    // Don't burn the full timeout when the peer connection is already torn down.
    if (isRoomTransportDead(room) || room.state === ConnectionState.Disconnected) {
      return false
    }
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

/**
 * Room options for the LiveKit host URL.
 *
 * Use the SDK default (singlePeerConnection: true). Forcing dual PC
 * (subscriber-primary) broke reliable data for chat/stage between clients
 * on remote SFUs while media still looked "connected".
 *
 * Always returns a value when hostname parses so LiveKitRoom gets a stable options
 * object from the caller's useMemo (do not depend on host string for options shape).
 */
export function livekitRoomOptionsForUrl(livekitUrl: string): RoomOptions {
  // hostname unused for options shape; kept for API compatibility / future host-specific opts
  void livekitHostnameFromUrl(livekitUrl)
  return { singlePeerConnection: true }
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
  const filtered: RTCIceServer[] = []
  for (const server of iceServers) {
    const urls = (Array.isArray(server.urls) ? server.urls : [server.urls])
      .filter((url): url is string => typeof url === 'string' && url.startsWith('turns:'))
      .map(normalizeTurnsTlsPort)
    if (urls.length > 0) filtered.push({ ...server, urls })
  }
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

  engineProto.makeRTCConfiguration = function (this: { rtcConfig?: RTCConfiguration }, serverResponse: unknown) {
    const config = original.call(this, serverResponse) as RTCConfiguration
    const forceRelay = config.iceTransportPolicy === 'relay' || this.rtcConfig?.iceTransportPolicy === 'relay'
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
export function livekitConnectOptionsForHost(hostname: string, preferRelay = false): RoomConnectOptions | undefined {
  if (isLocalLiveKitHostname(hostname)) return undefined
  ensureRemoteLiveKitTransportPatch()
  if (preferRelay || shouldForceLiveKitIceRelay()) {
    return relayRtcConnectOptions()
  }
  return { peerConnectionTimeout: 45_000 }
}

/** Prefer livekitHost from join response over window.location (user may open UI via localhost tunnel). */
export function livekitConnectOptionsForUrl(livekitUrl: string, preferRelay = false): RoomConnectOptions | undefined {
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
