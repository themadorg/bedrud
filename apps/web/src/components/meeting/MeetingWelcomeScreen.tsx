import { Headphones, Mic, MicOff, Video, VideoOff } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { API_URL } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { useInterfacePreferencesStore } from '#/lib/interface-preferences.store'
import {
  deviceIdToSelectValue,
  type MeetingDeviceKind,
  readMeetingDeviceId,
  selectValueToDeviceId,
  writeMeetingDeviceId,
} from '#/lib/meeting-device-storage'
import { patchUserPreferences } from '#/lib/user-preferences'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { WelcomePresenceBackdrop } from './WelcomePresenceBackdrop'

const METER_SEGMENTS = 24
const PRESENCE_COUNT_POLL_MS = 30_000

function welcomePresenceCountLabel(count: number) {
  if (count === 0) return 'No one in the meeting yet'
  if (count === 1) return '1 person in the meeting'
  return `${count} people in the meeting`
}

function useWelcomePresenceCount(roomId: string, enabled: boolean) {
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    if (!enabled) {
      setCount(null)
      return
    }

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined

    async function refresh() {
      try {
        const res = await fetch(`${API_URL}/api/room/${encodeURIComponent(roomId)}/presence?countOnly=1`, {
          credentials: 'include',
        })
        if (cancelled) return
        if (res.status === 429) {
          timer = setTimeout(refresh, 60_000)
          return
        }
        if (res.status === 404 || !res.ok) {
          setCount(null)
          return
        }
        const data = (await res.json()) as { count?: number }
        setCount(typeof data.count === 'number' ? data.count : null)
        timer = setTimeout(refresh, PRESENCE_COUNT_POLL_MS)
      } catch {
        if (!cancelled) timer = setTimeout(refresh, PRESENCE_COUNT_POLL_MS)
      }
    }

    void refresh()
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [roomId, enabled])

  return count
}

export interface WelcomeJoinChoices {
  micEnabled: boolean
  camEnabled: boolean
}

interface MeetingWelcomeScreenProps {
  roomId: string
  roomName: string
  isPublic?: boolean
  onJoin: (choices: WelcomeJoinChoices) => void
}

function deviceLabel(device: MediaDeviceInfo, kind: MeetingDeviceKind, index: number) {
  if (device.label) return device.label
  if (kind === 'audioinput') return `Microphone ${index + 1}`
  if (kind === 'videoinput') return `Camera ${index + 1}`
  return `Speaker ${index + 1}`
}

function WelcomeVolumeMeter({ volume }: { volume: number }) {
  const lit = Math.round(volume * METER_SEGMENTS)
  return (
    <div className="flex gap-px">
      {Array.from({ length: METER_SEGMENTS }).map((_, i) => {
        const isLit = i < lit
        const isRed = i >= METER_SEGMENTS * 0.83
        const isAmber = i >= METER_SEGMENTS * 0.65
        return (
          <div
            key={i}
            className={cn(
              'h-5 flex-1 transition-colors duration-75',
              isLit
                ? isRed
                  ? 'bg-[var(--meet-meter-fill-high)]'
                  : isAmber
                    ? 'bg-[var(--meet-meter-fill-mid)]'
                    : 'bg-[var(--meet-meter-fill)]'
                : 'bg-[var(--meet-meter-track)]',
            )}
          />
        )
      })}
    </div>
  )
}

function useMediaDevices(kind: MediaDeviceKind) {
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [deviceId, setDeviceIdState] = useState(() => readMeetingDeviceId(kind))

  const refresh = useCallback(async () => {
    if (!navigator.mediaDevices) return
    try {
      const all = await navigator.mediaDevices.enumerateDevices()
      const filtered = all.filter((d) => d.kind === kind)
      setDevices(filtered)
      setDeviceIdState((prev) => {
        if (prev && filtered.some((d) => d.deviceId === prev)) return prev
        const saved = readMeetingDeviceId(kind)
        if (saved && filtered.some((d) => d.deviceId === saved)) return saved
        return filtered[0]?.deviceId ?? ''
      })
    } catch {
      /* ignore */
    }
  }, [kind])

  useEffect(() => {
    refresh()
    navigator.mediaDevices?.addEventListener('devicechange', refresh)
    return () => navigator.mediaDevices?.removeEventListener('devicechange', refresh)
  }, [refresh])

  const setDeviceId = useCallback(
    (next: string) => {
      setDeviceIdState(next)
      writeMeetingDeviceId(kind, next)
    },
    [kind],
  )

  return { devices, deviceId, setDeviceId }
}

function useWelcomePreview({
  micEnabled,
  camEnabled,
  micDeviceId,
  camDeviceId,
}: {
  micEnabled: boolean
  camEnabled: boolean
  micDeviceId: string
  camDeviceId: string
}) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const ctxRef = useRef<AudioContext | null>(null)
  const rafRef = useRef<number>(0)
  const [volume, setVolume] = useState(0)
  const [error, setError] = useState<string | null>(null)

  const stopPreview = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    streamRef.current?.getTracks().forEach((track) => {
      track.stop()
    })
    streamRef.current = null
    ctxRef.current?.close().catch(() => {})
    ctxRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    setVolume(0)
  }, [])

  useEffect(() => {
    let cancelled = false

    async function start() {
      stopPreview()
      setError(null)

      if (!micEnabled && !camEnabled) return
      if (!navigator.mediaDevices?.getUserMedia) {
        setError('Media devices are not available in this browser.')
        return
      }

      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: micEnabled
            ? {
                deviceId: micDeviceId ? { exact: micDeviceId } : undefined,
                echoCancellation: true,
                noiseSuppression: true,
              }
            : false,
          video: camEnabled
            ? {
                deviceId: camDeviceId ? { exact: camDeviceId } : undefined,
              }
            : false,
        })

        if (cancelled) {
          stream.getTracks().forEach((track) => {
            track.stop()
          })
          return
        }

        streamRef.current = stream

        if (camEnabled && videoRef.current) {
          videoRef.current.srcObject = stream
          await videoRef.current.play().catch(() => {})
        }

        if (micEnabled && stream.getAudioTracks().length > 0) {
          const ctx = new AudioContext()
          ctxRef.current = ctx
          const source = ctx.createMediaStreamSource(new MediaStream([stream.getAudioTracks()[0]]))
          const analyser = ctx.createAnalyser()
          analyser.fftSize = 512
          analyser.smoothingTimeConstant = 0.6
          source.connect(analyser)
          const data = new Uint8Array(analyser.frequencyBinCount)

          const tick = () => {
            analyser.getByteFrequencyData(data)
            const rms = Math.sqrt(data.reduce((sum, value) => sum + value * value, 0) / data.length) / 128
            setVolume(Math.min(1, rms * 2.5))
            rafRef.current = requestAnimationFrame(tick)
          }
          rafRef.current = requestAnimationFrame(tick)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Could not access camera or microphone')
        }
      }
    }

    start()

    return () => {
      cancelled = true
      stopPreview()
    }
  }, [micEnabled, camEnabled, micDeviceId, camDeviceId, stopPreview])

  return { videoRef, volume, error, stopPreview }
}

export function MeetingWelcomeScreen({ roomId, roomName, isPublic = false, onJoin }: MeetingWelcomeScreenProps) {
  const tokens = useAuthStore((s) => s.tokens)
  const setShowWelcomeScreen = useInterfacePreferencesStore((s) => s.setShowWelcomeScreen)
  const mirrorWebcam = useVideoPreferencesStore((s) => s.mirrorWebcam)
  const presenceCount = useWelcomePresenceCount(roomId, isPublic)
  const [micEnabled, setMicEnabled] = useState(true)
  const [camEnabled, setCamEnabled] = useState(false)

  const mics = useMediaDevices('audioinput')
  const cameras = useMediaDevices('videoinput')
  const speakers = useMediaDevices('audiooutput')

  const { videoRef, volume, error, stopPreview } = useWelcomePreview({
    micEnabled,
    camEnabled,
    micDeviceId: mics.deviceId,
    camDeviceId: cameras.deviceId,
  })

  function handleJoin() {
    stopPreview()
    onJoin({ micEnabled, camEnabled })
  }

  function handleSkipWelcome() {
    setShowWelcomeScreen(false)
    if (tokens) {
      void patchUserPreferences({ interface: { showWelcomeScreen: false } })
    }
  }

  return (
    <div className="meet-room fixed inset-0 flex items-center justify-center overflow-hidden bg-[var(--meet-bg)] p-4">
      <WelcomePresenceBackdrop roomId={roomId} enabled={isPublic} />
      <div className="relative z-10 flex max-h-[calc(100vh-32px)] w-full max-w-[480px] flex-col items-center gap-3">
        <div className="meet-welcome-panel meet-prejoin-panel flex max-h-full w-full flex-col gap-5 overflow-y-auto meet-scroll">
          <div>
            <p className="m-0 text-[17px] font-semibold text-[var(--meet-fg)]">Ready to join?</p>
            <p className="m-0 mt-1.5 text-[13px] text-[var(--meet-fg-muted)]">
              Check your camera and microphone before joining{' '}
              <span className="text-[var(--meet-btn-muted-fg)]">{roomName}</span>
            </p>
            {isPublic && presenceCount !== null && (
              <p className="m-0 mt-1 text-xs text-[var(--meet-fg-subtle)]">
                {welcomePresenceCountLabel(presenceCount)}
              </p>
            )}
          </div>

          <div className="meet-welcome-preview relative overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-control)]">
            {camEnabled ? (
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                className="h-full w-full object-cover"
                style={{ transform: mirrorWebcam ? 'scaleX(-1)' : undefined }}
              />
            ) : (
              <div className="flex aspect-video w-full flex-col items-center justify-center gap-2 text-[var(--meet-fg-muted)]">
                <VideoOff className="h-10 w-10 opacity-50" />
                <span className="text-xs">Camera is off</span>
              </div>
            )}
            <div className="absolute bottom-3 end-3 flex gap-2">
              <button
                type="button"
                aria-label={micEnabled ? 'Turn microphone off' : 'Turn microphone on'}
                onClick={() => setMicEnabled((value) => !value)}
                className={cn(
                  'flex h-9 w-9 items-center justify-center rounded-full border backdrop-blur-sm transition-colors',
                  micEnabled
                    ? 'border-[var(--meet-border)] bg-[var(--meet-chrome)] text-[var(--meet-fg)]'
                    : 'border-destructive/40 bg-destructive/15 text-destructive',
                )}
              >
                {micEnabled ? <Mic className="h-4 w-4" /> : <MicOff className="h-4 w-4" />}
              </button>
              <button
                type="button"
                aria-label={camEnabled ? 'Turn camera off' : 'Turn camera on'}
                onClick={() => setCamEnabled((value) => !value)}
                className={cn(
                  'flex h-9 w-9 items-center justify-center rounded-full border backdrop-blur-sm transition-colors',
                  camEnabled
                    ? 'border-[var(--meet-border)] bg-[var(--meet-chrome)] text-[var(--meet-fg)]'
                    : 'border-destructive/40 bg-destructive/15 text-destructive',
                )}
              >
                {camEnabled ? <Video className="h-4 w-4" /> : <VideoOff className="h-4 w-4" />}
              </button>
            </div>
          </div>

          <div className="space-y-2">
            <WelcomeVolumeMeter volume={micEnabled ? volume : 0} />
            <div className="flex justify-between text-[10px] text-[var(--meet-fg-subtle)]">
              <span>Silent</span>
              <span>Loud</span>
            </div>
            {!micEnabled && (
              <p className="m-0 text-[11px] text-[var(--meet-fg-muted)]">Microphone is muted — you can still join.</p>
            )}
            {error && <p className="m-0 text-[11px] text-destructive">{error}</p>}
          </div>

          <div className="space-y-3">
            {cameras.devices.length > 0 && (
              <WelcomeDeviceRow
                label="Camera"
                icon={Video}
                devices={cameras.devices}
                kind="videoinput"
                value={cameras.deviceId}
                onValueChange={cameras.setDeviceId}
                disabled={!camEnabled}
              />
            )}
            {mics.devices.length > 0 && (
              <WelcomeDeviceRow
                label="Microphone"
                icon={Mic}
                devices={mics.devices}
                kind="audioinput"
                value={mics.deviceId}
                onValueChange={mics.setDeviceId}
                disabled={!micEnabled}
              />
            )}
            {speakers.devices.length > 0 && (
              <WelcomeDeviceRow
                label="Speaker"
                icon={Headphones}
                devices={speakers.devices}
                kind="audiooutput"
                value={speakers.deviceId}
                onValueChange={speakers.setDeviceId}
              />
            )}
          </div>

          <Button type="button" onClick={handleJoin} className="w-full rounded-lg py-2.5">
            Join meeting
          </Button>
        </div>
        <button
          type="button"
          onClick={handleSkipWelcome}
          className="text-xs text-[var(--meet-fg-muted)] underline-offset-2 transition-colors hover:text-[var(--meet-fg)] hover:underline"
        >
          Don&apos;t show this again
        </button>
      </div>
    </div>
  )
}

function WelcomeDeviceRow({
  label,
  icon: Icon,
  devices,
  kind,
  value,
  onValueChange,
  disabled,
}: {
  label: string
  icon: typeof Mic
  devices: MediaDeviceInfo[]
  kind: MeetingDeviceKind
  value: string
  onValueChange: (deviceId: string) => void
  disabled?: boolean
}) {
  return (
    <div className="flex items-center gap-3">
      <div className="flex w-24 shrink-0 items-center gap-1.5 text-xs text-[var(--meet-fg-muted)]">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span>{label}</span>
      </div>
      <Select
        value={deviceIdToSelectValue(value)}
        onValueChange={(next) => onValueChange(selectValueToDeviceId(next))}
        disabled={disabled}
      >
        <SelectTrigger className="h-9 min-w-0 flex-1 border-[var(--meet-border)] bg-[var(--meet-control)] text-xs text-[var(--meet-fg)]">
          <SelectValue placeholder={`Select ${label.toLowerCase()}`} />
        </SelectTrigger>
        <SelectContent>
          {devices.map((device, index) => (
            <SelectItem
              key={`${deviceIdToSelectValue(device.deviceId)}-${index}`}
              value={deviceIdToSelectValue(device.deviceId)}
              className="text-xs"
            >
              {deviceLabel(device, kind, index)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
