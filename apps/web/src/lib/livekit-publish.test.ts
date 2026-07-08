import { ConnectionState, Room } from 'livekit-client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  filterIceServersToTurnsTls,
  getLiveKitPublishDiagnostics,
  isPublishUnavailableError,
  isRoomConnected,
  isRoomPublishReady,
  isRoomSignalingReady,
  livekitConnectOptionsForHost,
  livekitConnectOptionsForUrl,
  livekitHostnameFromUrl,
  livekitRoomOptionsForUrl,
  normalizeTurnsTlsPort,
} from './livekit-publish'

describe('livekit-publish', () => {
  it('detects publish-ready room state', () => {
    const ready = {
      state: ConnectionState.Connected,
      engine: {
        verifyTransport: () => true,
        reliableDC: { readyState: 'open' },
      },
    } as unknown as Room
    expect(isRoomPublishReady(ready)).toBe(true)
    expect(isRoomConnected(ready)).toBe(true)

    const publisherOnlyReady = {
      state: ConnectionState.Connected,
      engine: {
        verifyTransport: () => true,
        reliableDC: { readyState: 'open' },
        pcManager: {},
      },
    } as unknown as Room
    expect(isRoomPublishReady(publisherOnlyReady)).toBe(true)

    const subscriberNotReady = {
      state: ConnectionState.Connected,
      engine: {
        verifyTransport: () => true,
        reliableDC: { readyState: 'open' },
        reliableDCSub: { readyState: 'connecting' },
        pcManager: { subscriber: {} },
      },
    } as unknown as Room
    expect(isRoomPublishReady(subscriberNotReady)).toBe(false)

    const subscriberReady = {
      state: ConnectionState.Connected,
      engine: {
        verifyTransport: () => true,
        reliableDC: { readyState: 'open' },
        reliableDCSub: { readyState: 'open' },
        pcManager: { subscriber: {} },
      },
    } as unknown as Room
    expect(isRoomPublishReady(subscriberReady)).toBe(true)

    const noReliableDc = {
      state: ConnectionState.Connected,
      engine: { verifyTransport: () => true, reliableDC: { readyState: 'connecting' }, lossyDC: { readyState: 'open' } },
    } as unknown as Room
    expect(isRoomPublishReady(noReliableDc)).toBe(false)

    expect(isRoomConnected({ state: ConnectionState.Reconnecting } as Room)).toBe(false)

    const signaling = {
      state: ConnectionState.Connected,
      engine: { client: { ws: { readyState: WebSocket.OPEN } } },
    } as unknown as Room
    expect(isRoomSignalingReady(signaling)).toBe(true)
  })

  it('detects transient publish errors', () => {
    expect(isPublishUnavailableError(new Error('PC manager is closed'))).toBe(true)
    expect(isPublishUnavailableError({ name: 'UnexpectedConnectionState', code: 12 })).toBe(true)
    expect(isPublishUnavailableError(new Error('permission denied'))).toBe(false)
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('prefers P2P for remote hosts by default (TURN fallback via patched ICE servers)', () => {
    expect(livekitConnectOptionsForHost('localhost')).toBeUndefined()
    expect(livekitConnectOptionsForHost('debug.example.com')?.rtcConfig?.iceTransportPolicy).toBeUndefined()
    expect(livekitConnectOptionsForHost('debug.example.com')?.peerConnectionTimeout).toBe(45_000)
  })

  it('forces TURN relay when VITE_LIVEKIT_ICE_RELAY=1 (WireGuard)', () => {
    vi.stubEnv('VITE_LIVEKIT_ICE_RELAY', '1')
    expect(livekitConnectOptionsForHost('debug.example.com')?.rtcConfig?.iceTransportPolicy).toBe('relay')
    expect(livekitConnectOptionsForHost('debug.example.com')?.peerConnectionTimeout).toBe(45_000)
  })

  it('forces TURN relay when preferRelay fallback is requested', () => {
    expect(livekitConnectOptionsForHost('debug.example.com', true)?.rtcConfig?.iceTransportPolicy).toBe('relay')
  })

  it('rewrites TURNS port 443 to 5349', () => {
    expect(normalizeTurnsTlsPort('turns:debug.example.com:443?transport=tcp')).toBe(
      'turns:debug.example.com:5349?transport=tcp',
    )
    expect(normalizeTurnsTlsPort('turns:debug.example.com:5349?transport=tcp')).toBe(
      'turns:debug.example.com:5349?transport=tcp',
    )
  })

  it('filters ICE servers to TURNS/TCP only and fixes port 443', () => {
    const filtered = filterIceServersToTurnsTls([
      {
        urls: [
          'turn:debug.example.com:3478?transport=udp',
          'turns:debug.example.com:443?transport=tcp',
          'turns:debug.example.com:5349?transport=tcp',
        ],
      },
    ])
    expect(filtered[0]?.urls).toEqual([
      'turns:debug.example.com:5349?transport=tcp',
      'turns:debug.example.com:5349?transport=tcp',
    ])
  })

  it('parses livekit host from URL and prefers P2P unless relay env is set', () => {
    expect(livekitHostnameFromUrl('wss://debug.example.com/livekit')).toBe('debug.example.com')
    expect(livekitHostnameFromUrl('ws://127.0.0.1:7072')).toBe('127.0.0.1')
    expect(livekitConnectOptionsForUrl('wss://debug.example.com/livekit')?.peerConnectionTimeout).toBe(45_000)
    expect(livekitConnectOptionsForUrl('wss://debug.example.com/livekit')?.rtcConfig?.iceTransportPolicy).toBeUndefined()
    expect(livekitConnectOptionsForUrl('ws://127.0.0.1:7072')).toBeUndefined()
  })

  it('reports publish diagnostics', () => {
    const room = {
      state: ConnectionState.Connected,
      engine: {
        verifyTransport: () => true,
        reliableDC: { readyState: 'open' },
        lossyDC: { readyState: 'connecting' },
        pcManager: { mode: 'publisher-primary' },
        client: { ws: { readyState: WebSocket.OPEN } },
      },
    } as unknown as Room
    const diag = getLiveKitPublishDiagnostics(room, { rtcConfig: { iceTransportPolicy: 'relay' } })
    expect(diag.publishReady).toBe(true)
    expect(diag.reliableDcState).toBe('open')
    expect(diag.pcMode).toBe('publisher-primary')
    expect(diag.iceTransportPolicy).toBe('relay')
  })

  it('uses dual peer connections for remote LiveKit hosts', () => {
    expect(livekitRoomOptionsForUrl('wss://debug.example.com/livekit')).toEqual({ singlePeerConnection: false })
    expect(livekitRoomOptionsForUrl('ws://127.0.0.1:7072')).toBeUndefined()
    expect(livekitRoomOptionsForUrl('ws://localhost:7072')).toBeUndefined()
  })
})