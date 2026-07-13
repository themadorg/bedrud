// TODO oncoming feature
import '@livekit/components-styles/components'
import { LiveKitRoom, useRoomContext } from '@livekit/components-react'
import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import type { AudioCaptureOptions } from 'livekit-client'
import { ConnectionState, DisconnectReason, RoomEvent } from 'livekit-client'
import { WifiOff } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { useAuthStore } from '#/lib/auth.store'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useInterfacePreferencesStore } from '#/lib/interface-preferences.store'
import {
  getLiveKitPublishDiagnostics,
  installLiveKitPublisherPromiseFix,
  livekitConnectOptionsForUrl,
  resetLiveKitPublisherPromise,
  waitForRoomPublishReady,
} from '#/lib/livekit-publish'
import { getLiveKitTransportMode } from '#/lib/livekit-transport-type'
import {
  installMeetingDebugGlobals,
  meetingDebugClear,
  meetingDebugLog,
  meetingDebugSetMeta,
} from '#/lib/meeting-debug-log'
import { readMeetingDeviceId } from '#/lib/meeting-device-storage'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { usePinnedParticipants } from '#/lib/usePinnedParticipants'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { ErrorPage } from '@/components/ErrorPage'
import { AskActionBanner } from '@/components/meeting/AskActionBanner'
import { AudioProcessorManager } from '@/components/meeting/AudioProcessorManager'
import { BeforeUnloadLock } from '@/components/meeting/BeforeUnloadLock'
import { FocusLayout } from '@/components/meeting/FocusLayout'
import { KickDetector } from '@/components/meeting/KickDetector'
import { LiveKitTransportFallback } from '@/components/meeting/LiveKitTransportFallback'
import { MeetingProvider, type RoomDeletionEvent, useMeetingChatContext } from '@/components/meeting/MeetingContext'
import { MeetingErrorBoundary } from '@/components/meeting/MeetingErrorBoundary'
import { MeetingRoomAudioRenderer } from '@/components/meeting/MeetingRoomAudioRenderer'
import { MeetingRoomShell } from '@/components/meeting/MeetingRoomShell'
import { MeetingSoundEffects } from '@/components/meeting/MeetingSoundEffects'
import { MeetingWelcomeScreen, type WelcomeJoinChoices } from '@/components/meeting/MeetingWelcomeScreen'
import { MeetLoadingScreen } from '@/components/meeting/MeetLoadingScreen'
import { ParticipantGrid } from '@/components/meeting/ParticipantGrid'
import { SecureContextBanner } from '@/components/meeting/SecureContextBanner'
import { MeetingStageProvider, useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import { StageJoinNotifier } from '@/components/meeting/stage/StageJoinNotifier'
import { StageScreenShareOverlay } from '@/components/meeting/stage/StageScreenShareOverlay'
import { WhiteboardOverlay } from '@/components/meeting/whiteboard/WhiteboardOverlay'
import { WhiteboardWatchProvider } from '@/components/meeting/whiteboard/WhiteboardWatchContext'
import { YoutubeShareDialog } from '@/components/meeting/youtube/YoutubeShareDialog'
import { WebxdcMeetingDropZone } from '@/components/meeting/webxdc/WebxdcMeetingDropZone'
import { WebxdcStageOverlay } from '@/components/meeting/webxdc/WebxdcStageOverlay'
import { WebxdcWatchProvider } from '@/components/meeting/webxdc/WebxdcWatchContext'
import { YoutubeWatchProvider } from '@/components/meeting/youtube/YoutubeWatchContext'
import { YoutubeWatchOverlay } from '@/components/meeting/youtube/YoutubeWatchOverlay'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface JoinResponse {
  id: string
  name: string
  token: string
  livekitHost: string
  adminId: string
  createdBy?: string
  isPublic?: boolean
  settings?: {
    recordingsAllowed: boolean
  }
  activeRecordingId?: string
  /** Bedrud JWT from guest-join so guests can mint WebXDC tickets / call room APIs. */
  accessToken?: string
  refreshToken?: string
  tokens?: { accessToken: string; refreshToken?: string | null }
}

interface ArchivedOwnedResponse {
  status: 'archived_owned'
  name: string
  mode: string
  settings: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee: boolean
    isPersistent: boolean
    recordingsAllowed: boolean
  }
}

/** Always-on transport diagnostics (console + copyable ring buffer for bug reports). */
function LiveKitTransportDiagnostics({
  connectOptions,
  meetId,
  livekitHost,
}: {
  connectOptions?: { rtcConfig?: RTCConfiguration }
  meetId?: string
  livekitHost?: string
}) {
  const room = useRoomContext()
  useEffect(() => {
    // Patch the prototype used by THIS room instance (handles Vite multi-instance cases).
    installLiveKitPublisherPromiseFix(room)
    meetingDebugClear()
    meetingDebugSetMeta({
      meetId: meetId ?? room.name,
      livekitHost: livekitHost ?? '',
      iceTransportPolicy: connectOptions?.rtcConfig?.iceTransportPolicy ?? 'default(direct)',
      singlePeerConnection: true,
      buildTag: 'no-strict-mode-v4',
    })
    installMeetingDebugGlobals(() => room)

    const log = (label: string) => {
      const diag = getLiveKitPublishDiagnostics(room, connectOptions)
      meetingDebugLog(`transport.${label}`, { ...diag })
    }
    const onConnected = () => {
      installLiveKitPublisherPromiseFix(room)
      resetLiveKitPublisherPromise(room)
      log('connected')
      void room.engine.getConnectedServerAddress?.().then((addr) => {
        if (addr) meetingDebugLog('transport.ice_address', { address: addr })
      })
      void getLiveKitTransportMode(room).then((mode) => {
        if (mode !== 'unknown') {
          // p2p/direct = host/srflx ICE to the LiveKit SFU (not peer↔peer)
          meetingDebugLog('transport.ice_mode', {
            mode,
            note: 'p2p means Direct to SFU, not browser-to-browser',
          })
        }
      })
      void waitForRoomPublishReady(room, 45_000).then((ready) => {
        resetLiveKitPublisherPromise(room)
        const diag = getLiveKitPublishDiagnostics(room, connectOptions)
        meetingDebugLog(ready ? 'transport.data_channels_ready' : 'transport.data_channels_timeout', {
          ...diag,
          buildTag: 'no-strict-mode-v4',
        })
      })
    }
    const onState = (state: ConnectionState) => {
      meetingDebugLog('transport.connection_state', {
        state,
        ...getLiveKitPublishDiagnostics(room, connectOptions),
      })
    }
    const onParticipantConnected = (p: { identity: string }) => {
      meetingDebugLog('transport.participant_joined', {
        identity: p.identity,
        remotes: room.remoteParticipants.size,
      })
    }
    const onParticipantDisconnected = (p: { identity: string }) => {
      meetingDebugLog('transport.participant_left', {
        identity: p.identity,
        remotes: room.remoteParticipants.size,
      })
    }

    room.on(RoomEvent.Connected, onConnected)
    room.on(RoomEvent.Reconnected, onConnected)
    room.on(RoomEvent.ConnectionStateChanged, onState)
    room.on(RoomEvent.ParticipantConnected, onParticipantConnected)
    room.on(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
    if (room.state === ConnectionState.Connected) onConnected()

    const interval = window.setInterval(() => {
      if (room.state !== ConnectionState.Connected) return
      meetingDebugLog('transport.heartbeat', getLiveKitPublishDiagnostics(room, connectOptions))
    }, 15_000)

    return () => {
      room.off(RoomEvent.Connected, onConnected)
      room.off(RoomEvent.Reconnected, onConnected)
      room.off(RoomEvent.ConnectionStateChanged, onState)
      room.off(RoomEvent.ParticipantConnected, onParticipantConnected)
      room.off(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
      window.clearInterval(interval)
    }
  }, [room, connectOptions, meetId, livekitHost])
  return null
}

export const Route = createFileRoute('/m/$meetId')({
  ssr: false,
  head: () => ({ meta: [{ title: 'Meeting — Bedrud' }] }),
  component: MeetingPage,
})

async function checkUserStatus(): Promise<boolean> {
  const tokens = useAuthStore.getState().tokens
  if (!tokens?.accessToken) return false

  try {
    const meRes = await fetch('/api/auth/me', {
      headers: { Authorization: `Bearer ${tokens.accessToken}` },
      credentials: 'include',
    })
    if (meRes.ok) return true

    const refreshRes = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
    })
    return refreshRes.ok
  } catch {
    return false
  }
}

function MeetingPage() {
  const { meetId } = Route.useParams()
  const navigate = useNavigate()
  const tokens = useAuthStore((s) => s.tokens)

  // Defer auth-state decisions to client-side to avoid SSR flash.
  // On the server localStorage is unavailable, so tokens is always null — initializing
  // guestName from tokens directly would cause the guest dialog to flash during SSR
  // hydration. Instead we start in a "not yet mounted" state and resolve in an effect.
  const [mounted, setMounted] = useState(false)
  const [joinData, setJoinData] = useState<JoinResponse | null>(null)
  const [joinError, setJoinError] = useState<string | null>(null)
  // null = waiting to decide, '' = authenticated (skip dialog), string = confirmed guest name
  const [guestName, setGuestName] = useState<string | null>(null)
  const [guestInput, setGuestInput] = useState('')
  const [wasKicked, setWasKicked] = useState(false)
  const [wasRoomDeleted, setWasRoomDeleted] = useState(false)
  const [archivedRoom, setArchivedRoom] = useState<{
    name: string
    mode: string
    settings: ArchivedOwnedResponse['settings']
  } | null>(null)
  const redirectTargetRef = useRef({ to: '/auth/login', search: { redirect: undefined } as { redirect?: string } })
  const deletionTypeRef = useRef<'user_deleted' | 'room_closed'>('room_closed')

  const handleRoomDeleted = useCallback(() => {
    setWasRoomDeleted(true)
    const isUserDeleted = deletionTypeRef.current === 'user_deleted'

    if (isUserDeleted) {
      const toastId = toast.loading('Verifying your account...')
      checkUserStatus().then((exists) => {
        if (cancelledRef.current) return
        if (exists) {
          toast.success('Room closed', { id: toastId, description: 'This room is no longer available.' })
          navigate({ to: '/dashboard' })
        } else {
          toast.error('Account deleted', { id: toastId, description: 'Your account has been deleted.' })
          useAuthStore.getState().clear()
          setTimeout(() => {
            if (!cancelledRef.current) navigate({ to: '/auth/login', search: { redirect: undefined } })
          }, 2000)
        }
      })
    } else {
      toast.error('Room closed', { description: 'This room is no longer available.' })
      setTimeout(() => {
        if (!cancelledRef.current) navigate({ to: '/dashboard' })
      }, 5000)
    }
  }, [navigate])

  const handleRoomDeletionMessage = useCallback(
    (_event: RoomDeletionEvent, message: string, isCurrentUserDeleted: boolean) => {
      if (isCurrentUserDeleted) {
        deletionTypeRef.current = 'user_deleted'
        redirectTargetRef.current = { to: '/auth/login', search: { redirect: undefined } }
        toast.error('Account deleted', { description: message || 'Your account has been deleted.' })
      } else {
        deletionTypeRef.current = 'room_closed'
        redirectTargetRef.current = { to: '/dashboard', search: { redirect: undefined } }
        toast.error('Room closed', { description: message || 'This room has been closed.' })
      }
    },
    [],
  )

  // Set initial guestName only after mount (client-side), where tokens are available.
  useEffect(() => {
    setGuestName(tokens ? '' : null)
    setMounted(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tokens])

  // Audio preferences — derive MediaTrackConstraints from stored settings
  const noiseMode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const echoCancellation = useAudioPreferencesStore((s) => s.echoCancellation)
  const autoGainControl = useAudioPreferencesStore((s) => s.autoGainControl)
  const mergeAudioPrefs = useAudioPreferencesStore((s) => s.merge)
  const mergeVideoPrefs = useVideoPreferencesStore((s) => s.merge)
  const mergeExperimentalPrefs = useExperimentalPreferencesStore((s) => s.merge)
  const mergeInterfacePrefs = useInterfacePreferencesStore((s) => s.merge)
  const [welcomeChoices, setWelcomeChoices] = useState<WelcomeJoinChoices | null>(null)
  const welcomeSessionRef = useRef<{ roomKey: string; showWelcome: boolean } | null>(null)
  const welcomeSessionKeyRef = useRef<string | null>(null)

  useEffect(() => {
    if (!joinData) {
      welcomeSessionKeyRef.current = null
      setWelcomeChoices(null)
      return
    }
    if (welcomeSessionKeyRef.current !== joinData.id) {
      welcomeSessionKeyRef.current = joinData.id
      setWelcomeChoices(null)
    }
  }, [joinData])

  // Echo cancellation is always honoured from user preferences.
  // Noise suppression is only enabled for browser mode to avoid double-processing
  // with LiveKit audio processors (RNNoise/Krisp).
  const audioConstraints: AudioCaptureOptions | boolean =
    noiseMode === 'browser'
      ? { noiseSuppression: true, echoCancellation, autoGainControl }
      : { noiseSuppression: false, echoCancellation, autoGainControl: false }

  // One-shot preferences sync from backend. useRef guard ensures this runs exactly
  // once even if joinData is replaced (e.g. reconnect), so a mid-session local
  // change is never overwritten by a stale backend fetch.
  const prefsFetchedRef = useRef(false)
  useEffect(() => {
    if (!joinData || !tokens || prefsFetchedRef.current) return
    prefsFetchedRef.current = true
    api
      .get<{ preferencesJson: string }>('/api/auth/preferences')
      .then((r) => {
        if (cancelledRef.current || !r.preferencesJson) return
        try {
          const parsed = JSON.parse(r.preferencesJson)
          if (parsed?.audio) mergeAudioPrefs(parsed.audio)
          if (parsed?.video) mergeVideoPrefs(parsed.video)
          if (parsed?.experimental) mergeExperimentalPrefs(parsed.experimental)
          if (parsed?.interface) mergeInterfacePrefs(parsed.interface)
        } catch {
          /* use local defaults */
        }
      })
      .catch(() => {
        /* use local defaults on network failure */
      })
  }, [joinData, mergeAudioPrefs, mergeExperimentalPrefs, mergeInterfacePrefs, mergeVideoPrefs, tokens])

  const [preferRelayTransport, setPreferRelayTransport] = useState(false)

  const livekitConnectOptions = useMemo(
    () =>
      typeof window !== 'undefined' && joinData?.livekitHost
        ? livekitConnectOptionsForUrl(joinData.livekitHost, preferRelayTransport)
        : undefined,
    [joinData?.livekitHost, preferRelayTransport],
  )
  // Stable options identity — LiveKitRoom recreates its Room when options JSON changes.
  const livekitRoomOptions = useMemo(() => ({ singlePeerConnection: true }), [])

  // Stable media capture options (must stay above early returns).
  const joinMediaChoicesForHooks = welcomeChoices ?? { micEnabled: true, camEnabled: false }
  const micDeviceIdForHooks = readMeetingDeviceId('audioinput')
  const camDeviceIdForHooks = readMeetingDeviceId('videoinput')
  const livekitAudioStable: AudioCaptureOptions | boolean = useMemo(() => {
    if (!joinMediaChoicesForHooks.micEnabled) return false
    if (typeof audioConstraints === 'object') {
      return { ...audioConstraints, deviceId: micDeviceIdForHooks || undefined }
    }
    return audioConstraints
  }, [joinMediaChoicesForHooks.micEnabled, audioConstraints, micDeviceIdForHooks])
  const livekitVideoStable = useMemo(
    () => (joinMediaChoicesForHooks.camEnabled ? { deviceId: camDeviceIdForHooks || undefined } : false),
    [joinMediaChoicesForHooks.camEnabled, camDeviceIdForHooks],
  )

  const [currentToken, setCurrentToken] = useState<string | null>(null)
  const freshTokenRef = useRef<string | null>(null)
  const [liveKitEpoch, setLiveKitEpoch] = useState(0)
  const [showReconnectBanner, setShowReconnectBanner] = useState(false)
  const [fatalReconnectError, setFatalReconnectError] = useState<string | null>(null)
  const isDisconnectedRef = useRef(false)
  const disconnectedAtRef = useRef(0)
  const reconnectAttemptRef = useRef(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const remountCooldownRef = useRef(0)

  // New overlay state for better disconnect UX
  const [showDisconnectedOverlay, setShowDisconnectedOverlay] = useState(false)
  const [overlayMode, setOverlayMode] = useState<'reconnecting' | 'disconnected'>('reconnecting')
  const disconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const cancelledRef = useRef(false)

  // Do NOT own a Room instance. Passing a long-lived Room into LiveKitRoom + React Strict Mode
  // disconnects it on the first unmount; reusing that closed engine makes every publishData fail
  // with "PC manager is closed" (cached rejected publisher promise). LiveKitRoom creates a
  // fresh Room per mount; key={liveKitEpoch} forces a clean session when we remount.

  // ALL automatic remounts disabled. setLiveKitEpoch was the reconnect loop:
  // disconnect → setLiveKitEpoch → unmount → disconnect → …
  const handleTransportRemount = useCallback((reason: string) => {
    meetingDebugLog('transport.remount_suppressed', { reason })
  }, [])

  const handleTransportRelayFallback = useCallback((reason: string) => {
    // Do not remount for TURN either — it was part of the connect/disconnect storm.
    meetingDebugLog('transport.relay_suppressed', {
      reason,
      note: 'automatic remount disabled to stop reconnect loop',
    })
  }, [])

  /**
   * LiveKit SDK reconnects the same Room automatically on most drops.
   * We only refresh a token + remount when the user hits Retry (never on every disconnect).
   */
  const attemptReconnect = useCallback(() => {
    if (!meetId) return
    const attempt = ++reconnectAttemptRef.current
    meetingDebugLog('reconnect.token_refresh', { attempt })
    api
      .post<{ token: string }>('/api/room/refresh-token', { roomName: meetId })
      .then(({ token }) => {
        if (cancelledRef.current) return
        freshTokenRef.current = token
        // Only swap token if still disconnected — never while LiveKit is mid-session.
        if (isDisconnectedRef.current) {
          setCurrentToken(token)
        }
      })
      .catch((err: Error) => {
        if (cancelledRef.current) return
        const status = Number(err.message?.split(':')[0])
        if (status === 410) setFatalReconnectError('Room is no longer available.')
        else if (status === 403) setFatalReconnectError('Access denied.')
        else meetingDebugLog('reconnect.token_failed', { message: err.message })
      })
  }, [meetId])

  /** Manual full remount — only from the Retry button. */
  const manualHardReconnect = useCallback(() => {
    meetingDebugLog('reconnect.manual_hard', { buildTag: 'no-strict-mode-v4' })
    remountCooldownRef.current = Date.now()
    isDisconnectedRef.current = true
    setPreferRelayTransport(false)
    setLiveKitEpoch((e) => e + 1)
    attemptReconnect()
  }, [attemptReconnect])

  const handleDisconnected = useCallback(
    (reason?: DisconnectReason) => {
      // React unmount / user leave — do nothing (StrictMode used to spam this path).
      if (reason === DisconnectReason.CLIENT_INITIATED || reason === undefined) {
        meetingDebugLog('reconnect.ignore', { reason: String(reason) })
        return
      }

      meetingDebugLog('reconnect.disconnected', { reason: String(reason) })
      console.log(`[reconnect] disconnected reason=${reason}`)

      isDisconnectedRef.current = true
      disconnectedAtRef.current = Date.now()
      setShowDisconnectedOverlay(true)
      setOverlayMode('reconnecting')
      setShowReconnectBanner(true)

      // Soft: refresh token for native SDK reconnect / next connect. No remount.
      attemptReconnect()

      if (disconnectTimeoutRef.current) clearTimeout(disconnectTimeoutRef.current)
      disconnectTimeoutRef.current = setTimeout(() => {
        if (!cancelledRef.current && isDisconnectedRef.current) {
          setOverlayMode('disconnected')
        }
      }, 30_000)
    },
    [attemptReconnect],
  )

  const handleReconnected = useCallback(() => {
    console.log('[reconnect] connected')
    isDisconnectedRef.current = false
    disconnectedAtRef.current = 0
    reconnectAttemptRef.current = 0
    setShowReconnectBanner(false)
    setFatalReconnectError(null)
    setShowDisconnectedOverlay(false)
    setOverlayMode('reconnecting')

    if (retryTimerRef.current) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
    if (disconnectTimeoutRef.current) {
      clearTimeout(disconnectTimeoutRef.current)
      disconnectTimeoutRef.current = null
    }
  }, [])

  const addRecent = useRecentRoomsStore((s) => s.add)
  const queryClient = useQueryClient()
  const invalidatedRef = useRef(false)

  useEffect(() => {
    cancelledRef.current = false
    return () => {
      cancelledRef.current = true
      if (retryTimerRef.current) {
        clearTimeout(retryTimerRef.current)
        retryTimerRef.current = null
      }
      if (disconnectTimeoutRef.current) {
        clearTimeout(disconnectTimeoutRef.current)
        disconnectTimeoutRef.current = null
      }
      if (!invalidatedRef.current) {
        invalidatedRef.current = true
        queryClient.invalidateQueries({ queryKey: ['rooms'] })
      }
    }
  }, [queryClient])

  useEffect(() => {
    if (joinData) return
    if (tokens) {
      // Authenticated: join directly
      api
        .post<JoinResponse>('/api/room/join', { roomName: meetId })
        .then((data) => {
          if (cancelledRef.current) return
          // Archived room owned by current user — show recreate dialog
          if ((data as unknown as ArchivedOwnedResponse).status === 'archived_owned') {
            const ar = data as unknown as ArchivedOwnedResponse
            setArchivedRoom({ name: ar.name, mode: ar.mode, settings: ar.settings })
            return
          }
          if (!data.id) {
            setJoinError('Invalid join response')
            return
          }
          addRecent(meetId)
          setJoinData(data)
        })
        .catch((err: Error) => {
          if (!cancelledRef.current) setJoinError(err.message)
        })
    } else if (guestName !== null && guestName !== '') {
      // Guest with confirmed name
      api
        .post<JoinResponse>('/api/room/guest-join', { roomName: meetId, guestName })
        .then((data) => {
          if (cancelledRef.current) return
          addRecent(meetId)
          // Join data first so the join effect does not race into /api/room/join when tokens appear.
          setJoinData({ ...data, isPublic: data.isPublic ?? false })
          // Store API tokens so mint ticket / updates work (LiveKit token alone is not enough).
          const access = data.tokens?.accessToken || data.accessToken
          const refresh = data.tokens?.refreshToken ?? data.refreshToken ?? null
          if (access) {
            useAuthStore.getState().setTokens({ accessToken: access, refreshToken: refresh }, 'ephemeral')
          }
        })
        .catch((err: Error) => {
          if (!cancelledRef.current) setJoinError(err.message)
        })
    }
  }, [meetId, tokens, guestName, joinData, addRecent])

  // Still on server or waiting for client mount — show neutral spinner to avoid SSR flash
  if (!mounted) {
    return <MeetLoadingScreen />
  }

  // Show guest name dialog for unauthenticated users
  // Guard with mounted to avoid SSR flash — localStorage unavailable on server,
  // so guestName is always null and tokens always null during SSR.
  if (mounted && !tokens && guestName === null) {
    return (
      <div className="meet-room app-fixed-viewport flex items-center justify-center bg-[var(--meet-bg)]">
        <div className="meet-prejoin-panel flex flex-col gap-5">
          <div>
            <p className="m-0 text-[17px] font-semibold text-[var(--meet-fg)]">Join as guest</p>
            <p className="m-0 mt-1.5 text-[13px] text-[var(--meet-fg-muted)]">
              Enter your name to join <span className="text-[var(--meet-btn-muted-fg)]">{meetId}</span>
            </p>
          </div>
          <Input
            value={guestInput}
            onChange={(e) => setGuestInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && guestInput.trim()) setGuestName(guestInput.trim())
            }}
            placeholder="Your name"
            className="h-auto border-[var(--meet-border)] bg-[var(--meet-control)] py-2.5 text-[var(--meet-fg)] placeholder:text-[var(--meet-fg-subtle)]"
          />
          <div className="flex gap-2.5">
            <Button
              type="button"
              disabled={!guestInput.trim()}
              onClick={() => setGuestName(guestInput.trim())}
              className="flex-1 rounded-lg py-2.5"
            >
              Join
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate({ to: '/auth/login', search: { redirect: `/m/${meetId}` } })}
              className="rounded-lg border-[var(--meet-border)] px-3.5 py-2.5 text-[13px] text-[var(--meet-fg-muted)]"
            >
              Sign in
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Archived room owned by current user — show recreate dialog
  if (archivedRoom) {
    return (
      <div className="meet-room app-fixed-viewport flex items-center justify-center bg-[var(--meet-bg)]">
        <div className="meet-prejoin-panel meet-prejoin-panel-wide flex flex-col gap-5">
          <div>
            <p className="m-0 text-[17px] font-semibold text-[var(--meet-fg)]">This meeting has ended</p>
            <p className="m-0 mt-1.5 text-[13px] text-[var(--meet-fg-muted)]">
              <span className="text-[var(--meet-btn-muted-fg)]">{archivedRoom.name}</span> was created by you. Start a
              new meeting with this name?
            </p>
          </div>
          <div className="flex gap-2.5">
            <Button
              type="button"
              onClick={() => {
                api
                  .post<{ name: string }>('/api/room/create', {
                    name: archivedRoom.name,
                    mode: archivedRoom.mode,
                    settings: archivedRoom.settings,
                  })
                  .then((room) => {
                    if (cancelledRef.current) return
                    setArchivedRoom(null)
                    addRecent(room.name)
                    api
                      .post<JoinResponse>('/api/room/join', { roomName: room.name })
                      .then((data) => {
                        if (cancelledRef.current) return
                        if ((data as unknown as ArchivedOwnedResponse).status === 'archived_owned') {
                          const ar = data as unknown as ArchivedOwnedResponse
                          setArchivedRoom({ name: ar.name, mode: ar.mode, settings: ar.settings })
                          return
                        }
                        if (!data.id) {
                          setJoinError('Invalid join response')
                          return
                        }
                        setJoinData(data)
                      })
                      .catch((err: Error) => {
                        if (!cancelledRef.current) setJoinError(err.message)
                      })
                  })
                  .catch((err: Error) => {
                    if (!cancelledRef.current) setJoinError(err.message)
                  })
              }}
              className="flex-1 rounded-lg py-2.5"
            >
              Start new meeting
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate({ to: '/dashboard' })}
              className="rounded-lg border-[var(--meet-border)] px-3.5 py-2.5 text-[13px] text-[var(--meet-fg-muted)]"
            >
              Dashboard
            </Button>
          </div>
        </div>
      </div>
    )
  }

  if (joinError) {
    return (
      <ErrorPage
        variant="room-error"
        error={joinError}
        showBack={false}
        onRetry={() => {
          setJoinError(null)
          setJoinData(null)
          setArchivedRoom(null)
        }}
      />
    )
  }

  if (!joinData) {
    welcomeSessionRef.current = null
    return <MeetLoadingScreen label="Joining room…" />
  }

  if (welcomeSessionRef.current?.roomKey !== joinData.id) {
    welcomeSessionRef.current = {
      roomKey: joinData.id,
      showWelcome: useInterfacePreferencesStore.getState().showWelcomeScreen,
    }
  }

  // Guard: joinData without id (e.g. archived_owned mis-set as join) must not crash render.
  const skipWelcome = !welcomeSessionRef.current?.showWelcome
  if (!skipWelcome && welcomeChoices === null) {
    return (
      <MeetingWelcomeScreen
        roomId={joinData.id}
        roomName={joinData.name}
        isPublic={joinData.isPublic ?? false}
        onJoin={(choices) => setWelcomeChoices(choices)}
      />
    )
  }

  // joinData is guaranteed non-null past the early returns above.
  const {
    id,
    token: originalToken,
    livekitHost: wsUrl,
    name: roomName,
    adminId,
    createdBy,
    isPublic = false,
  } = joinData
  const token = currentToken ?? originalToken
  const livekitAudio = livekitAudioStable
  const livekitVideo = livekitVideoStable

  if (fatalReconnectError) {
    return <ErrorPage variant="room-error" title="Disconnected" description={fatalReconnectError} showBack />
  }

  if (wasKicked) {
    return <ErrorPage variant="kicked" showBack={false} />
  }

  if (wasRoomDeleted) {
    return <MeetLoadingScreen label="Room deleted. Redirecting…" />
  }

  return (
    <LiveKitRoom
      key={liveKitEpoch}
      token={token}
      serverUrl={wsUrl}
      connect
      connectOptions={livekitConnectOptions}
      options={livekitRoomOptions}
      audio={livekitAudio}
      video={livekitVideo}
      onDisconnected={handleDisconnected}
      onConnected={handleReconnected}
      onError={(err) => {
        meetingDebugLog('transport.livekit_error', {
          message: err instanceof Error ? err.message : String(err),
        })
      }}
    >
      <MeetingErrorBoundary>
        <LiveKitTransportDiagnostics
          connectOptions={livekitConnectOptions}
          meetId={meetId}
          livekitHost={joinData.livekitHost}
        />
        <LiveKitTransportFallback
          connectOptions={livekitConnectOptions}
          onNeedRemount={handleTransportRemount}
          onNeedRelay={handleTransportRelayFallback}
        />
        {/* LiveKitRoom renders as display:contents — this div is the actual viewport container */}
        {showReconnectBanner && (
          <div className="fixed top-0 start-0 end-0 z-[9999] bg-yellow-500/15 border-b border-yellow-500/30 px-4 py-2 text-center text-[13px] text-amber-400 backdrop-blur-sm">
            Reconnecting…
          </div>
        )}

        {showDisconnectedOverlay && (
          <DisconnectedOverlay
            mode={overlayMode}
            onRetry={() => {
              setOverlayMode('reconnecting')
              if (disconnectTimeoutRef.current) {
                clearTimeout(disconnectTimeoutRef.current)
                disconnectTimeoutRef.current = null
              }
              disconnectTimeoutRef.current = setTimeout(() => {
                if (!cancelledRef.current && isDisconnectedRef.current) {
                  setOverlayMode('disconnected')
                }
              }, 30_000)
              // User-initiated only — hard remount + token refresh.
              manualHardReconnect()
            }}
            onLeave={() => {
              navigate({ to: '/dashboard' })
            }}
          />
        )}
        <div className="meet-room app-fixed-viewport overflow-hidden bg-[var(--meet-bg)]">
          {/* Skip links */}
          <a
            href="#meet-grid"
            className="fixed -top-40 left-2 z-[9999] rounded-md bg-white px-3 py-1.5 text-[13px] font-medium text-black focus:top-2 transition-all duration-150 pointer-events-none focus:pointer-events-auto"
          >
            Skip to video grid
          </a>
          <a
            href="#meet-controls"
            className="fixed -top-40 left-2 z-[9999] rounded-md bg-white px-3 py-1.5 text-[13px] font-medium text-black focus:top-[38px] transition-all duration-150 pointer-events-none focus:pointer-events-auto"
          >
            Skip to controls
          </a>
          <SecureContextBanner />
          {/* Stage must wrap YouTube / whiteboard / WebXDC providers (they call useMeetingStage). */}
          <MeetingStageProvider>
            <MeetingProvider
              roomId={id}
              roomName={roomName}
              adminId={adminId ?? ''}
              createdBy={createdBy ?? ''}
              isPublic={isPublic}
              // TODO oncoming feature
              recordingsAllowed={false}
              // TODO oncoming feature
              activeRecordingId={undefined}
              onRoomDeletionMessage={handleRoomDeletionMessage}
            >
              <YoutubeWatchProvider>
                <WhiteboardWatchProvider>
                  {/* WebXDC needs MeetingProvider (roomId) + MeetingStageProvider (claimStage). */}
                  <WebxdcWatchProvider>
                    <StageJoinNotifier />
                    <BeforeUnloadLock />
                    <KickDetector onKicked={() => setWasKicked(true)} onRoomDeleted={handleRoomDeleted} />
                    <AskActionBanner />
                    <AudioProcessorManager />
                    <MeetingRoomAudioRenderer />
                    <MeetingSoundEffects />
                    <WebxdcMeetingDropZone />
                    {/* Ambient depth gradients */}
                    <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
                      <div
                        className="absolute rounded-full"
                        style={{
                          width: 900,
                          height: 900,
                          background: 'radial-gradient(circle, var(--meet-ambient-a) 0%, transparent 65%)',
                          top: '-300px',
                          left: '-300px',
                        }}
                      />
                      <div
                        className="absolute rounded-full"
                        style={{
                          width: 700,
                          height: 700,
                          background: 'radial-gradient(circle, var(--meet-ambient-b) 0%, transparent 65%)',
                          bottom: '-200px',
                          right: '-150px',
                        }}
                      />
                    </div>

                    <MeetingRoomShell meetId={meetId} navigate={() => navigate({ to: '/dashboard' })}>
                      <MeetingLayout />
                      <YoutubeWatchOverlay />
                      <WhiteboardOverlay />
                      <StageScreenShareOverlay />
                      <WebxdcStageOverlay />

                      <div
                        className="pointer-events-none absolute start-0 end-0 top-0 z-10 h-[calc(96px+env(safe-area-inset-top))]"
                        style={{
                          background: 'linear-gradient(to bottom, var(--meet-vignette-top) 0%, transparent 100%)',
                        }}
                      />
                      <div
                        className="pointer-events-none absolute bottom-0 start-0 end-0 z-10 h-[calc(128px+env(safe-area-inset-bottom))]"
                        style={{
                          background: 'linear-gradient(to top, var(--meet-vignette-bottom) 0%, transparent 100%)',
                        }}
                      />
                    </MeetingRoomShell>
                    <YoutubeShareDialog />
                  </WebxdcWatchProvider>
                </WhiteboardWatchProvider>
              </YoutubeWatchProvider>
            </MeetingProvider>
          </MeetingStageProvider>
        </div>
      </MeetingErrorBoundary>
    </LiveKitRoom>
  )
}

// ── Disconnected overlay (new UX for poor connection) ───────────────
function DisconnectedOverlay({
  mode,
  onRetry,
  onLeave,
}: {
  mode: 'reconnecting' | 'disconnected'
  onRetry: () => void
  onLeave: () => void
}) {
  return (
    <div className="app-fixed-viewport z-[99999] flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div className="mx-4 max-w-sm text-center">
        {mode === 'reconnecting' ? (
          <>
            <div className="mx-auto mb-4 h-10 w-10 animate-spin rounded-full border-4 border-white/20 border-t-primary" />
            <p className="text-lg font-medium text-white">Reconnecting…</p>
            <p className="mt-1 text-sm text-white/60">Trying to restore your connection</p>
          </>
        ) : (
          <>
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-500/10">
              <WifiOff className="h-6 w-6 text-red-400" />
            </div>
            <p className="text-lg font-medium text-white">Connection lost</p>
            <p className="mt-1 text-sm text-white/60">We couldn’t reconnect after several attempts.</p>

            <div className="mt-6 flex flex-col gap-2 sm:flex-row sm:justify-center">
              <button
                type="button"
                onClick={onRetry}
                className="rounded-lg bg-white px-5 py-2.5 text-sm font-medium text-black transition hover:bg-white/90"
              >
                Retry now
              </button>
              <button
                type="button"
                onClick={onLeave}
                className="rounded-lg border border-white/20 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-white/10"
              >
                Leave meeting
              </button>
            </div>
            <p className="mt-3 text-[11px] text-white/40">You can also try refreshing the page</p>
          </>
        )}
      </div>
    </div>
  )
}

// ── Layout switcher (inside LiveKitRoom context) ───────────────
function MeetingLayout() {
  const { stage } = useMeetingStage()
  const { pinned, toggle, clear } = usePinnedParticipants()
  const { systemMessages } = useMeetingChatContext()
  const isFocusMode = pinned.size > 0

  const lastSpotlightTsRef = useRef(0)
  useEffect(() => {
    const last = [...systemMessages].reverse().find((m) => m.event === 'spotlight')
    if (!last || last.ts <= lastSpotlightTsRef.current) return
    lastSpotlightTsRef.current = last.ts
    if (!pinned.has(last.target!) && last.target) toggle(last.target)
  }, [systemMessages, pinned, toggle])

  useEffect(() => () => clear(), [clear])

  if (stage) return null

  if (isFocusMode) {
    return <FocusLayout pinnedIdentities={pinned} onTogglePin={toggle} />
  }
  return <ParticipantGrid pinnedIdentities={pinned} onTogglePin={toggle} />
}
