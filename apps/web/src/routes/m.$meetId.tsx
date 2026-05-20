// TODO oncoming feature
import '@livekit/components-styles/components'
import { LiveKitRoom, RoomAudioRenderer, useTracks } from '@livekit/components-react'
import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import type { AudioCaptureOptions } from 'livekit-client'
import { DisconnectReason, Track } from 'livekit-client'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { useAuthStore } from '#/lib/auth.store'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { usePinnedParticipants } from '#/lib/usePinnedParticipants'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { ErrorPage } from '@/components/ErrorPage'
import { AskActionBanner } from '@/components/meeting/AskActionBanner'
import { AudioProcessorManager } from '@/components/meeting/AudioProcessorManager'
import { BeforeUnloadLock } from '@/components/meeting/BeforeUnloadLock'
import { FocusLayout } from '@/components/meeting/FocusLayout'
import { KickDetector } from '@/components/meeting/KickDetector'
import { MeetingProvider, type RoomDeletionEvent, useMeetingChatContext } from '@/components/meeting/MeetingContext'
import { MeetingErrorBoundary } from '@/components/meeting/MeetingErrorBoundary'
import { MeetingHeader } from '@/components/meeting/MeetingHeader'
import { MeetingPanels } from '@/components/meeting/MeetingPanels'
import { MeetingSoundEffects } from '@/components/meeting/MeetingSoundEffects'
import { ParticipantGrid } from '@/components/meeting/ParticipantGrid'
import { SecureContextBanner } from '@/components/meeting/SecureContextBanner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface JoinResponse {
  id: string
  name: string
  token: string
  livekitHost: string
  adminId: string
  settings?: {
    recordingsAllowed: boolean
  }
  activeRecordingId?: string
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

export const Route = createFileRoute('/m/$meetId')({
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
        if (exists) {
          toast.success('Room closed', { id: toastId, description: 'This room is no longer available.' })
          navigate({ to: '/dashboard' })
        } else {
          toast.error('Account deleted', { id: toastId, description: 'Your account has been deleted.' })
          useAuthStore.getState().clear()
          setTimeout(() => navigate({ to: '/auth/login', search: { redirect: undefined } }), 2000)
        }
      })
    } else {
      toast.error('Room closed', { description: 'This room is no longer available.' })
      setTimeout(() => navigate({ to: '/dashboard' }), 5000)
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
        if (!r.preferencesJson) return
        try {
          const parsed = JSON.parse(r.preferencesJson)
          if (parsed?.audio) mergeAudioPrefs(parsed.audio)
          if (parsed?.video) mergeVideoPrefs(parsed.video)
        } catch {
          /* use local defaults */
        }
      })
      .catch(() => {
        /* use local defaults on network failure */
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [joinData, mergeAudioPrefs, mergeVideoPrefs, tokens])

  const [currentToken, setCurrentToken] = useState<string | null>(null)
  const [showReconnectBanner, setShowReconnectBanner] = useState(false)
  const [fatalReconnectError, setFatalReconnectError] = useState<string | null>(null)
  const isDisconnectedRef = useRef(false)
  const disconnectedAtRef = useRef(0)
  const reconnectAttemptRef = useRef(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const attemptReconnect = useCallback(() => {
    if (!meetId) return

    const attempt = ++reconnectAttemptRef.current
    const backoff = Math.min(1000 * 2 ** (attempt - 1), 30000)

    console.log(`[reconnect] attempt=${attempt} backoff=${backoff}ms`)

    api
      .post<{ token: string }>('/api/room/refresh-token', { roomName: meetId })
      .then(({ token }) => {
        if (!isDisconnectedRef.current) {
          console.log('[reconnect] already reconnected natively, caching token')
          setCurrentToken(token)
          return
        }
        console.log(`[reconnect] attempt=${attempt} succeeded`)
        setCurrentToken(token)
      })
      .catch((err: Error) => {
        const status = Number(err.message?.split(':')[0])
        if (status === 410) {
          console.log('[reconnect] fatal: room gone')
          setFatalReconnectError('Room is no longer available.')
          return
        }
        if (status === 403) {
          console.log('[reconnect] fatal: access denied')
          setFatalReconnectError('Access denied.')
          return
        }
        console.log(`[reconnect] attempt=${attempt} failed, retry in ${backoff}ms`)
        retryTimerRef.current = setTimeout(attemptReconnect, backoff)
      })
  }, [meetId])

  const handleDisconnected = useCallback(
    (reason?: DisconnectReason) => {
      if (reason === DisconnectReason.CLIENT_INITIATED) return

      if (retryTimerRef.current) {
        clearTimeout(retryTimerRef.current)
        retryTimerRef.current = null
      }

      isDisconnectedRef.current = true
      disconnectedAtRef.current = Date.now()
      reconnectAttemptRef.current = 0
      setShowReconnectBanner(false)
      setFatalReconnectError(null)

      console.log(`[reconnect] disconnected reason=${reason}`)

      setTimeout(() => {
        if (isDisconnectedRef.current) {
          setShowReconnectBanner(true)
        }
      }, 5000)

      attemptReconnect()
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
    if (retryTimerRef.current) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
  }, [])

  const addRecent = useRecentRoomsStore((s) => s.add)
  const queryClient = useQueryClient()
  const invalidatedRef = useRef(false)

  useEffect(() => {
    return () => {
      if (retryTimerRef.current) {
        clearTimeout(retryTimerRef.current)
        retryTimerRef.current = null
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
          // Archived room owned by current user — show recreate dialog
          if ((data as unknown as ArchivedOwnedResponse).status === 'archived_owned') {
            const ar = data as unknown as ArchivedOwnedResponse
            setArchivedRoom({ name: ar.name, mode: ar.mode, settings: ar.settings })
            return
          }
          addRecent(meetId)
          setJoinData(data)
        })
        .catch((err: Error) => setJoinError(err.message))
    } else if (guestName !== null && guestName !== '') {
      // Guest with confirmed name
      api
        .post<JoinResponse>('/api/room/guest-join', { roomName: meetId, guestName })
        .then((data) => {
          addRecent(meetId)
          setJoinData(data)
        })
        .catch((err: Error) => setJoinError(err.message))
    }
  }, [meetId, tokens, guestName, joinData, addRecent])

  // Still on server or waiting for client mount — show neutral spinner to avoid SSR flash
  if (!mounted) {
    return (
      <div className="fixed inset-0 bg-[#07070f] flex flex-col items-center justify-center gap-3.5">
        <div
          className="w-12 h-12 rounded-full animate-[meet-connecting-spin_0.9s_linear_infinite]"
          style={{
            border: '2px solid color-mix(in oklab, var(--primary) 30%, transparent)',
            borderTopColor: 'var(--primary)',
          }}
        />
      </div>
    )
  }

  // Show guest name dialog for unauthenticated users
  // Guard with mounted to avoid SSR flash — localStorage unavailable on server,
  // so guestName is always null and tokens always null during SSR.
  if (mounted && !tokens && guestName === null) {
    return (
      <div className="fixed inset-0 bg-[#07070f] flex items-center justify-center">
        <div
          className="bg-white/[0.04] border border-white/[0.08] rounded-2xl px-7 py-8 flex flex-col gap-5"
          style={{ width: 'min(340px, calc(100vw - 32px))' }}
        >
          <div>
            <p className="text-white text-[17px] font-semibold m-0">Join as guest</p>
            <p className="text-white/40 text-[13px] mt-1.5 m-0">
              Enter your name to join <span className="text-teal-400">{meetId}</span>
            </p>
          </div>
          <Input
            value={guestInput}
            onChange={(e) => setGuestInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && guestInput.trim()) setGuestName(guestInput.trim())
            }}
            placeholder="Your name"
            className="bg-white/[0.06] border-white/[0.12] text-white placeholder:text-white/40 h-auto py-2.5"
          />
          <div className="flex gap-2.5">
            <Button
              type="button"
              disabled={!guestInput.trim()}
              onClick={() => setGuestName(guestInput.trim())}
              className="flex-1 py-2.5 rounded-lg"
            >
              Join
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => navigate({ to: '/auth/login', search: { redirect: `/m/${meetId}` } })}
              className="px-3.5 py-2.5 rounded-lg border border-white/10 text-white/50 text-[13px]"
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
      <div className="fixed inset-0 bg-[#07070f] flex items-center justify-center">
        <div
          className="bg-white/[0.04] border border-white/[0.08] rounded-2xl px-7 py-8 flex flex-col gap-5"
          style={{ width: 'min(380px, calc(100vw - 32px))' }}
        >
          <div>
            <p className="text-white text-[17px] font-semibold m-0">This meeting has ended</p>
            <p className="text-white/40 text-[13px] mt-1.5 m-0">
              <span className="text-teal-400">{archivedRoom.name}</span> was created by you. Start a new meeting with
              this name?
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
                    setArchivedRoom(null)
                    addRecent(room.name)
                    api
                      .post<JoinResponse>('/api/room/join', { roomName: room.name })
                      .then((data) => {
                        setJoinData(data)
                      })
                      .catch((err: Error) => setJoinError(err.message))
                  })
                  .catch((err: Error) => setJoinError(err.message))
              }}
              className="flex-1 py-2.5 rounded-lg"
            >
              Start new meeting
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => navigate({ to: '/dashboard' })}
              className="px-3.5 py-2.5 rounded-lg border border-white/10 text-white/50 text-[13px]"
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
    return (
      <div
        style={{
          position: 'fixed',
          inset: 0,
          background: '#07070f',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 14,
        }}
      >
        <div
          style={{
            width: 48,
            height: 48,
            borderRadius: '50%',
            border: '2px solid color-mix(in oklab, var(--primary) 30%, transparent)',
            borderTopColor: 'var(--primary)',
            animation: 'meet-connecting-spin 0.9s linear infinite',
          }}
        />
        <p style={{ color: 'rgba(255,255,255,0.3)', fontSize: 13 }}>Joining room…</p>
      </div>
    )
  }

  const { id, token: originalToken, livekitHost: wsUrl, name: roomName, adminId } = joinData
  const token = currentToken ?? originalToken

  if (fatalReconnectError) {
    return <ErrorPage variant="room-error" title="Disconnected" description={fatalReconnectError} showBack />
  }

  if (wasKicked) {
    return <ErrorPage variant="kicked" showBack={false} />
  }

  if (wasRoomDeleted) {
    return (
      <div
        style={{
          position: 'fixed',
          inset: 0,
          background: '#07070f',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 14,
        }}
      >
        <div
          style={{
            width: 48,
            height: 48,
            borderRadius: '50%',
            border: '2px solid color-mix(in oklab, var(--primary) 30%, transparent)',
            borderTopColor: 'var(--primary)',
            animation: 'meet-connecting-spin 0.9s linear infinite',
          }}
        />
        <p style={{ color: 'rgba(255,255,255,0.5)', fontSize: 14 }}>Room deleted. Redirecting…</p>
      </div>
    )
  }

  return (
    <MeetingErrorBoundary>
      <LiveKitRoom
        token={token}
        serverUrl={wsUrl}
        connect
        audio={audioConstraints}
        video={false}
        onDisconnected={handleDisconnected}
        onConnected={handleReconnected}
      >
        <RoomAudioRenderer />
        {/* LiveKitRoom renders as display:contents — this div is the actual viewport container */}
        {showReconnectBanner && (
          <div className="fixed top-0 left-0 right-0 z-[9999] bg-yellow-500/15 border-b border-yellow-500/30 px-4 py-2 text-center text-[13px] text-amber-400 backdrop-blur-sm">
            Reconnecting…
          </div>
        )}
        <div className="fixed inset-0 overflow-hidden" style={{ background: '#07070f' }}>
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
          <MeetingProvider
            roomId={id}
            roomName={roomName}
            adminId={adminId ?? ''}
            // TODO oncoming feature
            recordingsAllowed={false}
            // TODO oncoming feature
            activeRecordingId={undefined}
            onRoomDeletionMessage={handleRoomDeletionMessage}
          >
            <BeforeUnloadLock />
            <KickDetector onKicked={() => setWasKicked(true)} onRoomDeleted={handleRoomDeleted} />
            <AskActionBanner />
            <AudioProcessorManager />
            <MeetingSoundEffects />
            {/* Ambient depth gradients */}
            <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
              <div
                className="absolute rounded-full"
                style={{
                  width: 900,
                  height: 900,
                  background:
                    'radial-gradient(circle, color-mix(in oklab, var(--primary) 5.5%, transparent) 0%, transparent 65%)',
                  top: '-300px',
                  left: '-300px',
                }}
              />
              <div
                className="absolute rounded-full"
                style={{
                  width: 700,
                  height: 700,
                  background: 'radial-gradient(circle, rgba(6,182,212,0.04) 0%, transparent 65%)',
                  bottom: '-200px',
                  right: '-150px',
                }}
              />
            </div>

            <MeetingLayout />

            {/* Vignettes for header/controls legibility */}
            <div
              className="pointer-events-none absolute left-0 right-0 top-0 z-10 h-[calc(96px+env(safe-area-inset-top))]"
              style={{ background: 'linear-gradient(to bottom, rgba(7,7,15,0.65) 0%, transparent 100%)' }}
            />
            <div
              className="pointer-events-none absolute bottom-0 left-0 right-0 z-10 h-[calc(128px+env(safe-area-inset-bottom))]"
              style={{ background: 'linear-gradient(to top, rgba(7,7,15,0.6) 0%, transparent 100%)' }}
            />

            <MeetingHeader meetId={meetId} sessionStartedAt={sessionStartedAt} />

            {/* Side panels */}
            <MeetingPanels navigate={() => navigate({ to: '/dashboard' })} />
          </MeetingProvider>
        </div>
      </LiveKitRoom>
    </MeetingErrorBoundary>
  )
}

// ── Layout switcher (inside LiveKitRoom context) ───────────────
function MeetingLayout() {
  const { pinned, toggle, clear } = usePinnedParticipants()
  const { systemMessages } = useMeetingChatContext()
  const screenShareTracks = useTracks([Track.Source.ScreenShare])
  const isFocusMode = screenShareTracks.length > 0 || pinned.size > 0

  // Auto-pin on spotlight system message
  const lastSpotlightTsRef = useRef(0)
  useEffect(() => {
    const last = [...systemMessages].reverse().find((m) => m.event === 'spotlight')
    if (!last || last.ts <= lastSpotlightTsRef.current) return
    lastSpotlightTsRef.current = last.ts
    if (!pinned.has(last.target!) && last.target) toggle(last.target)
  }, [systemMessages, pinned, toggle])

  useEffect(() => () => clear(), [clear])

  if (isFocusMode) {
    return <FocusLayout pinnedIdentities={pinned} onTogglePin={toggle} />
  }
  return <ParticipantGrid pinnedIdentities={pinned} onTogglePin={toggle} />
}
