import { useMutation } from '@tanstack/react-query'
import { Check, Loader2, Monitor, Video, VideoOff } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  deviceIdToSelectValue,
  readMeetingDeviceId,
  selectValueToDeviceId,
  writeMeetingDeviceId,
} from '#/lib/meeting-device-storage'
import { patchUserPreferences } from '#/lib/user-preferences'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { isMeetingTone, panelSurfaceClass, type SettingsPanelTone } from './settingsPanelTone'

interface VideoSettingsPanelProps {
  tone?: SettingsPanelTone
  onCameraDeviceChange?: (deviceId: string) => void | Promise<void>
}

function useVideoInputDevices() {
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [deviceId, setDeviceIdState] = useState(() => readMeetingDeviceId('videoinput'))

  const refresh = useCallback(async () => {
    if (!navigator.mediaDevices) return
    try {
      const all = await navigator.mediaDevices.enumerateDevices()
      const inputs = all.filter((d) => d.kind === 'videoinput')
      setDevices(inputs)
      setDeviceIdState((prev) => {
        if (prev && inputs.some((d) => d.deviceId === prev)) return prev
        const saved = readMeetingDeviceId('videoinput')
        if (saved && inputs.some((d) => d.deviceId === saved)) return saved
        return inputs[0]?.deviceId ?? ''
      })
    } catch {
      /* ignore */
    }
  }, [])

  useEffect(() => {
    refresh()
    navigator.mediaDevices?.addEventListener('devicechange', refresh)
    return () => navigator.mediaDevices?.removeEventListener('devicechange', refresh)
  }, [refresh])

  const setDeviceId = useCallback((next: string, onChange?: (deviceId: string) => void | Promise<void>) => {
    setDeviceIdState(next)
    writeMeetingDeviceId('videoinput', next)
    void onChange?.(next)
  }, [])

  return { devices, deviceId, setDeviceId, refresh }
}

function useCameraPreview(deviceId: string) {
  const [previewing, setPreviewing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)

  const stop = useCallback(() => {
    streamRef.current?.getTracks().forEach((track) => {
      track.stop()
    })
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    setPreviewing(false)
  }, [])

  useEffect(() => {
    if (!previewing) return

    let cancelled = false
    setError(null)

    async function run() {
      streamRef.current?.getTracks().forEach((track) => {
        track.stop()
      })
      streamRef.current = null

      if (!navigator.mediaDevices?.getUserMedia) {
        setError('Camera is not available in this browser.')
        setPreviewing(false)
        return
      }

      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: deviceId ? { deviceId: { exact: deviceId } } : true,
        })
        if (cancelled) {
          stream.getTracks().forEach((track) => {
            track.stop()
          })
          return
        }
        streamRef.current = stream
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          await videoRef.current.play().catch(() => {})
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Could not access camera')
          setPreviewing(false)
        }
      }
    }

    void run()
    return () => {
      cancelled = true
      streamRef.current?.getTracks().forEach((track) => {
        track.stop()
      })
      streamRef.current = null
    }
  }, [previewing, deviceId])

  const start = useCallback(() => setPreviewing(true), [])

  return { videoRef, previewing, error, start, stop }
}

export function VideoSettingsPanel({ tone = 'default', onCameraDeviceChange }: VideoSettingsPanelProps) {
  const meeting = isMeetingTone(tone)
  const mirrorWebcam = useVideoPreferencesStore((s) => s.mirrorWebcam)
  const setMirrorWebcam = useVideoPreferencesStore((s) => s.setMirrorWebcam)
  const cameras = useVideoInputDevices()
  const preview = useCameraPreview(cameras.deviceId)
  const { videoRef, previewing, error, start, stop } = preview

  const videoPrefsRef = useRef({ mirrorWebcam })
  videoPrefsRef.current = { mirrorWebcam }

  const syncMutation = useMutation({
    mutationFn: () => patchUserPreferences({ video: videoPrefsRef.current }),
  })
  const mutateRef = useRef(syncMutation.mutate)
  mutateRef.current = syncMutation.mutate

  useEffect(() => {
    const timer = setTimeout(() => mutateRef.current(), 1000)
    return () => clearTimeout(timer)
  }, [mirrorWebcam])

  useEffect(() => {
    return () => {
      stop()
      void patchUserPreferences({ video: videoPrefsRef.current })
    }
  }, [stop])

  const syncStatus = syncMutation.isPending
    ? 'saving'
    : syncMutation.isError
      ? 'error'
      : syncMutation.isSuccess
        ? 'saved'
        : 'idle'

  const selectTriggerClass = cn(
    'min-w-0 flex-1 text-xs',
    meeting && 'border-[var(--meet-border)] bg-[var(--meet-control)] text-[var(--meet-fg)]',
  )

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Camera preview</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Choose a video source and check how you look before joining
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 p-5">
          <div className="flex flex-wrap items-center gap-3">
            {cameras.devices.length > 0 && (
              <Select
                value={deviceIdToSelectValue(cameras.deviceId)}
                onValueChange={(value) => cameras.setDeviceId(selectValueToDeviceId(value), onCameraDeviceChange)}
              >
                <SelectTrigger className={selectTriggerClass}>
                  <SelectValue placeholder="Select camera" />
                </SelectTrigger>
                <SelectContent>
                  {cameras.devices.map((device, index) => (
                    <SelectItem
                      key={`${deviceIdToSelectValue(device.deviceId)}-${index}`}
                      value={deviceIdToSelectValue(device.deviceId)}
                      className="text-xs"
                    >
                      {device.label || `Camera ${index + 1}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            <Button
              type="button"
              variant={previewing ? 'destructive' : 'default'}
              size="sm"
              onClick={previewing ? stop : start}
              className="shrink-0 gap-1.5"
            >
              {previewing ? <VideoOff className="h-3 w-3" /> : <Video className="h-3 w-3" />}
              {previewing ? 'Stop' : 'Start preview'}
            </Button>
          </div>

          {error && <p className="text-[10px] text-destructive">{error}</p>}

          <div
            className={cn(
              'overflow-hidden rounded-lg border bg-[var(--meet-control)]',
              meeting ? 'border-[var(--meet-border)]' : 'border-border bg-muted/40',
            )}
          >
            {previewing ? (
              <video
                ref={videoRef}
                autoPlay
                playsInline
                muted
                className="aspect-video w-full object-cover"
                style={{ transform: mirrorWebcam ? 'scaleX(-1)' : undefined }}
              />
            ) : (
              <div className="flex aspect-video w-full flex-col items-center justify-center gap-2">
                <VideoOff
                  className={cn(
                    'h-10 w-10 opacity-50',
                    meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground',
                  )}
                />
                <span className={cn('text-xs', meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground')}>
                  Preview is off
                </span>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <div className={panelSurfaceClass(tone)}>
        <div
          className={cn(
            'flex items-center justify-between gap-4 px-5 py-4',
            meeting ? 'border-b border-[var(--meet-border)]' : 'border-b',
          )}
        >
          <div className="flex min-w-0 items-start gap-3">
            <Monitor
              className={cn(
                'mt-0.5 h-4 w-4 shrink-0',
                meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground',
              )}
            />
            <div className="min-w-0">
              <p className="text-sm font-medium">Mirror my video</p>
              <p className={cn('text-xs', meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground')}>
                Flip the preview horizontally so it behaves like a mirror
              </p>
            </div>
          </div>
          <Switch checked={mirrorWebcam} onCheckedChange={setMirrorWebcam} />
        </div>

        {syncStatus !== 'idle' && (
          <div
            className={cn(
              'flex items-center justify-end gap-1.5 px-5 py-2.5',
              meeting ? 'border-t border-[var(--meet-border)]' : 'border-t',
            )}
          >
            {syncStatus === 'saving' && (
              <Loader2
                className={cn(
                  'h-3 w-3 animate-spin',
                  meeting ? 'text-[var(--meet-fg-subtle)]' : 'text-muted-foreground/50',
                )}
              />
            )}
            {syncStatus === 'saved' && <Check className="h-3 w-3 text-emerald-500" />}
            <span className={cn('text-[11px]', meeting ? 'text-[var(--meet-fg-subtle)]' : 'text-muted-foreground/50')}>
              {syncStatus === 'saving' && 'Saving...'}
              {syncStatus === 'saved' && 'Saved'}
              {syncStatus === 'error' && 'Sync failed'}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
