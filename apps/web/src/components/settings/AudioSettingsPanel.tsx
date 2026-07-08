import { useMutation } from '@tanstack/react-query'
import { Brain, Check, Globe, Loader2, Mic, MicOff, Shield, Zap } from 'lucide-react'
import React, { useCallback, useEffect, useRef, useState } from 'react'
import { PushToTalkKeyCapture } from '#/components/settings/PushToTalkKeyCapture'
import {
  type AudioPreferences,
  type NoiseSuppressionMode,
  useAudioPreferencesStore,
} from '#/lib/audio-preferences.store'
import { AudioProcessorService } from '#/lib/audio-processor.service'
import { deviceIdToSelectValue, selectValueToDeviceId } from '#/lib/meeting-device-storage'
import { patchUserPreferences } from '#/lib/user-preferences'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { isMeetingTone, meetingSliderClass, type SettingsPanelTone } from './settingsPanelTone'

const MODES: { value: NoiseSuppressionMode; label: string; icon: React.ElementType }[] = [
  { value: 'none', label: 'Off', icon: Shield },
  { value: 'browser', label: 'Browser', icon: Globe },
  { value: 'rnnoise', label: 'RNNoise', icon: Brain },
  { value: 'krisp', label: 'Krisp', icon: Zap },
]

const SEGMENTS = 32

const BEEP_INTERVALS: { value: number; label: string }[] = [
  { value: 3000, label: '3s' },
  { value: 10000, label: '10s' },
  { value: 30000, label: '30s' },
  { value: 60000, label: '1 min' },
  { value: 120000, label: '2 min' },
]

function modeSegmentClass(active: boolean, meeting: boolean, disabled: boolean) {
  return cn(
    'flex min-w-0 flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-2 text-xs font-medium transition-colors',
    active
      ? meeting
        ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
        : 'bg-background text-foreground shadow-sm'
      : meeting
        ? 'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control-hover)] hover:text-[var(--meet-fg-strong)]'
        : 'text-muted-foreground hover:bg-muted/80 hover:text-foreground',
    disabled && 'cursor-not-allowed opacity-40',
  )
}

function VolumeMeter({ volume, meeting }: { volume: number; meeting?: boolean }) {
  const lit = Math.round(volume * SEGMENTS)
  return (
    <div className="flex gap-px">
      {Array.from({ length: SEGMENTS }).map((_, i) => {
        const isLit = i < lit
        const isRed = i >= SEGMENTS * 0.83
        const isAmber = i >= SEGMENTS * 0.65
        return (
          <div
            key={i}
            className={cn(
              'flex-1 h-6 transition-colors duration-75',
              isLit
                ? isRed
                  ? 'bg-destructive'
                  : isAmber
                    ? 'bg-amber-500'
                    : meeting
                      ? 'bg-accent-400'
                      : 'bg-emerald-500'
                : meeting
                  ? 'bg-white/[0.08]'
                  : 'bg-muted/60',
            )}
          />
        )
      })}
    </div>
  )
}

function useMicTest(mode: NoiseSuppressionMode, echoCancellation: boolean, inputGain: number, noiseGate: number) {
  const [testing, setTesting] = useState(false)
  const [volume, setVolume] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [deviceId, setDeviceId] = useState<string>('')

  const streamRef = useRef<MediaStream | null>(null)
  const ctxRef = useRef<AudioContext | null>(null)
  const gainNodeRef = useRef<GainNode | null>(null)
  const gateNodeRef = useRef<GainNode | null>(null)
  const rafRef = useRef<number>(0)
  const cancelledRef = useRef(false)
  const noiseGateRef = useRef(noiseGate)
  useEffect(() => {
    noiseGateRef.current = noiseGate
  }, [noiseGate])
  useEffect(() => {
    if (gainNodeRef.current) gainNodeRef.current.gain.value = inputGain / 100
  }, [inputGain])

  const enumerateDevices = useCallback(async () => {
    const all = await navigator.mediaDevices.enumerateDevices()
    const inputs = all.filter((d) => d.kind === 'audioinput')
    setDevices(inputs)
    if (inputs.length > 0) setDeviceId((prev) => prev || inputs[0].deviceId)
  }, [])

  useEffect(() => {
    enumerateDevices().catch(() => {})
  }, [enumerateDevices])

  const stop = useCallback(() => {
    cancelledRef.current = true
    cancelAnimationFrame(rafRef.current)
    streamRef.current?.getTracks().forEach((t) => {
      t.stop()
    })
    ctxRef.current?.close().catch(() => {})
    streamRef.current = null
    ctxRef.current = null
    gainNodeRef.current = null
    gateNodeRef.current = null
    setVolume(0)
    setTesting(false)
  }, [])

  const start = useCallback(async () => {
    cancelledRef.current = false
    setError(null)
    try {
      const withSuppression = mode === 'browser'
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          deviceId: deviceId ? { exact: deviceId } : undefined,
          noiseSuppression: withSuppression,
          echoCancellation,
          autoGainControl: withSuppression,
        },
      })

      if (cancelledRef.current) {
        stream.getTracks().forEach((t) => {
          t.stop()
        })
        return
      }

      streamRef.current = stream
      await enumerateDevices()

      if (cancelledRef.current) {
        stream.getTracks().forEach((t) => {
          t.stop()
        })
        streamRef.current = null
        return
      }

      const ctx = new AudioContext()
      ctxRef.current = ctx

      const source = ctx.createMediaStreamSource(stream)
      const gainNode = ctx.createGain()
      gainNode.gain.value = inputGain / 100
      gainNodeRef.current = gainNode

      const preAnalyser = ctx.createAnalyser()
      preAnalyser.fftSize = 512
      preAnalyser.smoothingTimeConstant = 0.3

      const gateNode = ctx.createGain()
      gateNode.gain.value = 1
      gateNodeRef.current = gateNode

      const postAnalyser = ctx.createAnalyser()
      postAnalyser.fftSize = 512
      postAnalyser.smoothingTimeConstant = 0.6

      source.connect(gainNode)
      gainNode.connect(preAnalyser)
      preAnalyser.connect(gateNode)
      gateNode.connect(postAnalyser)
      postAnalyser.connect(ctx.destination)

      const preData = new Uint8Array(preAnalyser.frequencyBinCount)
      const postData = new Uint8Array(postAnalyser.frequencyBinCount)

      const tick = () => {
        preAnalyser.getByteFrequencyData(preData)
        const preRms = Math.sqrt(preData.reduce((s, v) => s + v * v, 0) / preData.length) / 128
        const preVol = Math.min(1, preRms * 2.5)
        const threshold = noiseGateRef.current / 100
        const targetGain = preVol < threshold ? 0 : 1
        const timeConst = targetGain === 0 ? 0.005 : 0.08
        gateNodeRef.current?.gain.setTargetAtTime(targetGain, ctx.currentTime, timeConst)

        postAnalyser.getByteFrequencyData(postData)
        const postRms = Math.sqrt(postData.reduce((s, v) => s + v * v, 0) / postData.length) / 128
        setVolume(Math.min(1, postRms * 2.5))

        rafRef.current = requestAnimationFrame(tick)
      }
      rafRef.current = requestAnimationFrame(tick)
      setTesting(true)
    } catch (err) {
      if (!cancelledRef.current) {
        setError(err instanceof Error ? err.message : 'Could not access microphone')
      }
    }
  }, [mode, echoCancellation, deviceId, inputGain, enumerateDevices])

  useEffect(
    () => () => {
      stop()
    },
    [stop],
  )

  return { testing, volume, error, devices, deviceId, setDeviceId, start, stop }
}

function ToggleRow({
  title,
  description,
  meeting,
  children,
}: {
  title: string
  description: string
  meeting: boolean
  children: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
      <div className="min-w-0">
        <p className="text-xs font-medium">{title}</p>
        <p className={cn('mt-0.5 text-[10px]', meeting ? 'text-white/50' : 'text-muted-foreground')}>{description}</p>
      </div>
      {children}
    </div>
  )
}

export function AudioSettingsPanel({ tone = 'default' }: { tone?: SettingsPanelTone }) {
  const meeting = isMeetingTone(tone)
  const mode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const echoCancellation = useAudioPreferencesStore((s) => s.echoCancellation)
  const autoGainControl = useAudioPreferencesStore((s) => s.autoGainControl)
  const inputGain = useAudioPreferencesStore((s) => s.inputGain)
  const noiseGate = useAudioPreferencesStore((s) => s.noiseGate)
  const mutedBeepEnabled = useAudioPreferencesStore((s) => s.mutedBeepEnabled)
  const mutedBeepInterval = useAudioPreferencesStore((s) => s.mutedBeepInterval)
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)
  const pushToTalkKey = useAudioPreferencesStore((s) => s.pushToTalkKey)
  const setMode = useAudioPreferencesStore((s) => s.setMode)
  const setEchoCancellation = useAudioPreferencesStore((s) => s.setEchoCancellation)
  const setAutoGainControl = useAudioPreferencesStore((s) => s.setAutoGainControl)
  const setInputGain = useAudioPreferencesStore((s) => s.setInputGain)
  const setNoiseGate = useAudioPreferencesStore((s) => s.setNoiseGate)
  const setMutedBeepEnabled = useAudioPreferencesStore((s) => s.setMutedBeepEnabled)
  const setMutedBeepInterval = useAudioPreferencesStore((s) => s.setMutedBeepInterval)
  const setPushToTalkEnabled = useAudioPreferencesStore((s) => s.setPushToTalkEnabled)
  const setPushToTalkKey = useAudioPreferencesStore((s) => s.setPushToTalkKey)

  const krispSupported = AudioProcessorService.isKrispSupported()
  const mic = useMicTest(mode, echoCancellation, inputGain, noiseGate)

  const audioPrefsRef = useRef<AudioPreferences>({
    noiseSuppressionMode: mode,
    echoCancellation,
    autoGainControl,
    inputGain,
    noiseGate,
    mutedBeepEnabled,
    mutedBeepInterval,
    pushToTalkEnabled,
    pushToTalkKey,
  })
  audioPrefsRef.current = {
    noiseSuppressionMode: mode,
    echoCancellation,
    autoGainControl,
    inputGain,
    noiseGate,
    mutedBeepEnabled,
    mutedBeepInterval,
    pushToTalkEnabled,
    pushToTalkKey,
  }

  const syncMutation = useMutation({
    mutationFn: () => patchUserPreferences({ audio: audioPrefsRef.current }),
  })
  const mutateRef = useRef(syncMutation.mutate)
  mutateRef.current = syncMutation.mutate

  useEffect(() => {
    const timer = setTimeout(() => mutateRef.current(), 1000)
    return () => clearTimeout(timer)
  }, [
    mode,
    echoCancellation,
    autoGainControl,
    inputGain,
    noiseGate,
    mutedBeepEnabled,
    mutedBeepInterval,
    pushToTalkEnabled,
    pushToTalkKey,
  ])

  useEffect(() => {
    return () => {
      void patchUserPreferences({ audio: audioPrefsRef.current })
    }
  }, [])

  const syncStatus = syncMutation.isPending
    ? 'saving'
    : syncMutation.isError
      ? 'error'
      : syncMutation.isSuccess
        ? 'saved'
        : 'idle'

  const segmentTrackClass = cn(
    'flex w-full rounded-lg border p-0.5',
    meeting ? 'border-white/10 bg-white/[0.04]' : 'border-border bg-muted/40',
  )

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Noise suppression</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Choose how your microphone is processed before others hear you
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 p-5">
          <div className={segmentTrackClass} role="radiogroup" aria-label="Noise suppression mode">
            {MODES.map(({ value, label, icon: Icon }) => {
              const active = mode === value
              const disabled = value === 'krisp' && !krispSupported
              return (
                <label key={value} className={cn(modeSegmentClass(active, meeting, disabled), 'cursor-pointer')}>
                  <input
                    type="radio"
                    name="noise-suppression-mode"
                    value={value}
                    checked={active}
                    onChange={() => setMode(value)}
                    disabled={disabled}
                    className="sr-only"
                  />
                  <Icon className="h-3 w-3 shrink-0" />
                  <span className="truncate">{label}</span>
                </label>
              )
            })}
          </div>

          <div className={cn('divide-y', meeting ? 'divide-white/[0.08]' : undefined)}>
            <ToggleRow title="Echo cancellation" description="Reduce feedback from speakers" meeting={meeting}>
              <Switch checked={echoCancellation} onCheckedChange={setEchoCancellation} />
            </ToggleRow>
            {mode === 'browser' && (
              <ToggleRow title="Auto gain" description="Normalize input volume automatically" meeting={meeting}>
                <Switch checked={autoGainControl} onCheckedChange={setAutoGainControl} />
              </ToggleRow>
            )}
          </div>

          {(mode === 'rnnoise' || mode === 'krisp') && (
            <p className={cn('text-[10px]', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              {mode === 'krisp' ? 'Krisp' : 'RNNoise'} runs in-call only — preview shows gain and gate.
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Microphone test</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Preview input levels with your current settings
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 p-5">
          <div className="flex flex-wrap items-center gap-3">
            {mic.devices.length > 0 && (
              <Select
                value={deviceIdToSelectValue(mic.deviceId)}
                onValueChange={(v) => mic.setDeviceId(selectValueToDeviceId(v))}
                disabled={mic.testing}
              >
                <SelectTrigger
                  className={cn(
                    'min-w-0 flex-1 text-xs',
                    mic.testing && 'opacity-50',
                    meeting && 'border-white/10 bg-white/[0.04] text-white/90',
                  )}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {mic.devices.map((d, i) => (
                    <SelectItem
                      key={`${deviceIdToSelectValue(d.deviceId)}-${i}`}
                      value={deviceIdToSelectValue(d.deviceId)}
                      className="text-xs"
                    >
                      {d.label || `Microphone ${i + 1}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            <Button
              type="button"
              variant={mic.testing ? 'destructive' : 'default'}
              size="sm"
              onClick={mic.testing ? mic.stop : mic.start}
              className="shrink-0 gap-1.5"
            >
              {mic.testing ? <MicOff className="h-3 w-3" /> : <Mic className="h-3 w-3" />}
              {mic.testing ? 'Stop' : 'Test mic'}
            </Button>
          </div>

          {mic.testing && (
            <p className={cn('text-[10px]', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              Headphones recommended
            </p>
          )}
          {mic.error && <p className="text-[10px] text-destructive">{mic.error}</p>}

          <div>
            <VolumeMeter volume={mic.volume} meeting={meeting} />
            <div
              className={cn(
                'mt-1 flex justify-between text-[10px]',
                meeting ? 'text-white/50' : 'text-muted-foreground',
              )}
            >
              <span>Silent</span>
              <span>Loud</span>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Input levels</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Adjust gain and noise gate threshold
          </CardDescription>
        </CardHeader>
        <CardContent className="p-5">
          <div className="grid gap-6 sm:grid-cols-2">
            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-xs font-medium">Gain</span>
                <span className="font-mono text-xs font-semibold tabular-nums text-primary">{inputGain}%</span>
              </div>
              <Slider
                min={0}
                max={300}
                step={1}
                value={[inputGain]}
                onValueChange={(v) => setInputGain(v[0])}
                className={meeting ? meetingSliderClass : undefined}
              />
              <p className={cn('text-[10px]', meeting ? 'text-white/50' : 'text-muted-foreground')}>
                100% = unity gain
              </p>
            </div>

            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-xs font-medium">Noise gate</span>
                <span className="font-mono text-xs font-semibold tabular-nums text-primary">{noiseGate}%</span>
              </div>
              <Slider
                min={0}
                max={100}
                step={1}
                value={[noiseGate]}
                onValueChange={(v) => setNoiseGate(v[0])}
                className={meeting ? meetingSliderClass : undefined}
              />
              <p className={cn('text-[10px]', meeting ? 'text-white/50' : 'text-muted-foreground')}>0% = disabled</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Push to talk</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Hold a key to speak instead of an open mic
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 p-5">
          <div className="flex items-center justify-between gap-4">
            <span className="text-xs font-medium">Enable push to talk</span>
            <Switch checked={pushToTalkEnabled} onCheckedChange={setPushToTalkEnabled} />
          </div>

          {pushToTalkEnabled ? (
            <div
              className={cn(
                'divide-y rounded-lg border px-3',
                meeting ? 'border-white/[0.08] bg-white/[0.02] divide-white/[0.08]' : 'border-border bg-muted/20',
              )}
            >
              <ToggleRow title="Activation key" description="Click the key button to rebind" meeting={meeting}>
                <PushToTalkKeyCapture value={pushToTalkKey} onChange={setPushToTalkKey} meeting={meeting} />
              </ToggleRow>
            </div>
          ) : (
            <div
              className={cn(
                'divide-y rounded-lg border px-3',
                meeting ? 'border-white/[0.08] bg-white/[0.02] divide-white/[0.08]' : 'border-border bg-muted/20',
              )}
            >
              <ToggleRow
                title="Mute shortcut"
                description="Press Space during a call to mute or unmute"
                meeting={meeting}
              >
                <kbd
                  className={cn(
                    'rounded-md border px-2.5 py-1 font-mono text-xs',
                    meeting
                      ? 'border-white/10 bg-white/[0.06] text-white/70'
                      : 'border-border bg-muted/50 text-foreground',
                  )}
                >
                  Space
                </kbd>
              </ToggleRow>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Muted mic alert</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">
            Beep when you speak while your microphone is muted
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 p-5">
          <div className="flex items-center justify-between gap-4">
            <span className="text-xs font-medium">Enable reminder</span>
            <Switch checked={mutedBeepEnabled} onCheckedChange={setMutedBeepEnabled} />
          </div>

          {mutedBeepEnabled && (
            <div className="space-y-2">
              <p className="text-xs font-medium">Reminder interval</p>
              <div className={segmentTrackClass} role="radiogroup" aria-label="Muted mic alert interval">
                {BEEP_INTERVALS.map(({ value, label }) => {
                  const active = mutedBeepInterval === value
                  return (
                    <label key={value} className={cn(modeSegmentClass(active, meeting, false), 'cursor-pointer')}>
                      <input
                        type="radio"
                        name="muted-mic-alert-interval"
                        value={value}
                        checked={active}
                        onChange={() => setMutedBeepInterval(value)}
                        className="sr-only"
                      />
                      {label}
                    </label>
                  )
                })}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {syncStatus !== 'idle' && (
        <div
          className={cn(
            'flex items-center justify-end gap-1.5 text-[11px]',
            meeting ? 'text-white/50' : 'text-muted-foreground',
          )}
        >
          {syncStatus === 'saving' && <Loader2 className="h-3 w-3 animate-spin" />}
          {syncStatus === 'saved' && <Check className="h-3 w-3 text-emerald-500" />}
          <span>
            {syncStatus === 'saving' && 'Saving...'}
            {syncStatus === 'saved' && 'Saved'}
            {syncStatus === 'error' && 'Sync failed'}
          </span>
        </div>
      )}
    </div>
  )
}
