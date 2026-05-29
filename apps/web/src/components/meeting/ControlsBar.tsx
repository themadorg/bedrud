// TODO oncoming feature
import { useLocalParticipant, useRoomContext } from '@livekit/components-react'
import { ConnectionState, RoomEvent } from 'livekit-client'
import {
  Check,
  ChevronDown,
  Keyboard,
  Link2,
  Maximize,
  Mic,
  MicOff,
  MonitorOff,
  MonitorUp,
  MoreVertical,
  PhoneOff,
  Settings,
  Video,
  VideoOff,
  Volume2,
  VolumeX,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { type NoiseSuppressionMode, useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { AudioProcessorService } from '#/lib/audio-processor.service'
import { useAuthStore } from '#/lib/auth.store'
import { cn } from '#/lib/utils'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

interface Props {
  onLeave: () => void
}

/* ── Mobile detection ──────────────────────────────────────────────────────── */

function useIsMobile(breakpoint = 640) {
  const [mobile, setMobile] = useState(false)
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint - 1}px)`)
    setMobile(mq.matches)
    const handler = (e: MediaQueryListEvent) => setMobile(e.matches)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [breakpoint])
  return mobile
}

/* ── CtrlBtn: tooltip-wrapped control button ─────────────────────────────── */

function btnIconCn(active = false, danger = false, isMobile = false) {
  return cn(
    'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color] duration-150',
    isMobile ? 'h-[38px] w-[38px] rounded-[10px]' : 'h-11 w-11 rounded-xl',
    danger
      ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
      : active
        ? 'bg-primary/25 text-teal-400 hover:bg-primary/30'
        : 'bg-white/[0.07] text-white/75 hover:bg-white/[0.12]',
  )
}

function CtrlBtn({
  tip,
  active = false,
  danger = false,
  isMobile = false,
  className,
  onClick,
  children,
}: {
  tip: string
  active?: boolean
  danger?: boolean
  isMobile?: boolean
  className?: string
  onClick?: () => void
  children: React.ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={onClick}
          className={cn(btnIconCn(active, danger, isMobile), className)}
          aria-label={tip}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={8}>
        {tip}
      </TooltipContent>
    </Tooltip>
  )
}

const dividerCn = 'w-px h-7 bg-white/[0.08] mx-0.5 shrink-0 max-sm:hidden'

const darkMenuCn = 'min-w-60 max-w-[calc(100vw-24px)] bg-[#0f0f1c]/98 border border-white/5 rounded-xl backdrop-blur-xl'

const NOISE_MODES: { value: NoiseSuppressionMode; label: string }[] = [
  { value: 'none', label: 'Off' },
  { value: 'browser', label: 'Browser' },
  { value: 'rnnoise', label: 'RNNoise' },
  { value: 'krisp', label: 'Krisp AI' },
]

const STORAGE_KEYS: Record<string, string> = {
  audioinput: 'bedrud_mic_device',
  audiooutput: 'bedrud_speaker_device',
}

/* ── Device list hook (replaces individual DeviceSelector for audio) ──────── */

function useDeviceList(kind: 'audioinput' | 'audiooutput') {
  const room = useRoomContext()
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [activeId, setActiveId] = useState(() => localStorage.getItem(STORAGE_KEYS[kind]) ?? '')

  // Sync activeId from the room's actual active device
  const syncActiveFromRoom = useCallback(() => {
    const actual = room.getActiveDevice(kind)
    if (actual) setActiveId(actual)
  }, [room, kind])

  const cancelledRef = useRef(false)

  useEffect(() => {
    cancelledRef.current = false
    if (!navigator.mediaDevices) return
    const refresh = async () => {
      try {
        const all = await navigator.mediaDevices.enumerateDevices()
        if (cancelledRef.current) return
        const filtered = all.filter((d) => d.kind === kind)
        setDevices(filtered)
        syncActiveFromRoom()
      } catch {
        /* permissions not yet granted */
      }
    }
    refresh()
    navigator.mediaDevices.addEventListener('devicechange', refresh)
    return () => {
      cancelledRef.current = true
      navigator.mediaDevices.removeEventListener('devicechange', refresh)
    }
  }, [kind, syncActiveFromRoom])

  // Restore saved device on room connect, then sync actual active device
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEYS[kind])

    const applyDevice = async () => {
      if (saved) {
        await room.switchActiveDevice(kind, saved).catch(() => {})
      }
      if (!cancelledRef.current) {
        // Always sync to what the room is actually using (handles fallback)
        syncActiveFromRoom()
      }
    }

    if (room.state === ConnectionState.Connected) {
      applyDevice()
      return
    }
    const handler = () => {
      applyDevice()
    }
    room.once(RoomEvent.Connected, handler)
    return () => {
      room.off(RoomEvent.Connected, handler)
    }
  }, [room, kind, syncActiveFromRoom])

  // Listen for device changes from the room (e.g. system default change)
  useEffect(() => {
    const handler = () => syncActiveFromRoom()
    room.on(RoomEvent.ActiveDeviceChanged, handler)
    return () => {
      room.off(RoomEvent.ActiveDeviceChanged, handler)
    }
  }, [room, syncActiveFromRoom])

  const select = useCallback(
    async (deviceId: string) => {
      await room.switchActiveDevice(kind, deviceId).catch(() => {})
      setActiveId(deviceId)
      localStorage.setItem(STORAGE_KEYS[kind], deviceId)
    },
    [room, kind],
  )

  return { devices, activeId, select }
}

/* ── ControlsBar ──────────────────────────────────────────────────────────── */

export function ControlsBar({ onLeave }: Props) {
  const isMobile = useIsMobile()
  const { localParticipant } = useLocalParticipant()
  const { isSelfDeafened, toggleSelfDeafen } = useMeetingRoomContext()

  const micEnabled = localParticipant?.isMicrophoneEnabled ?? false
  const camEnabled = localParticipant?.isCameraEnabled ?? false
  const isScreenShareEnabled = localParticipant?.isScreenShareEnabled ?? false

  const tokens = useAuthStore((s) => s.tokens)
  const canShare = Boolean(tokens) && Boolean(navigator.mediaDevices?.getDisplayMedia)
  const shareTip = !tokens
    ? 'Sign in to share screen'
    : !navigator.mediaDevices?.getDisplayMedia
      ? 'Screen sharing not supported'
      : isScreenShareEnabled
        ? 'Stop sharing'
        : 'Share screen'

  const noiseMode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const setMode = useAudioPreferencesStore((s) => s.setMode)

  const mics = useDeviceList('audioinput')
  const speakers = useDeviceList('audiooutput')

  const iconSize = isMobile ? 16 : 18
  const iconSizeSm = isMobile ? 15 : 17

  // ── Push-to-talk ──
  const pttActiveRef = useRef(false)
  const pttInitMicRef = useRef(false)
  const [pttInitMic, setPttInitMic] = useState(false)
  const [pttVisible, setPttVisible] = useState(false)

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.code !== 'Space' || e.repeat || pttActiveRef.current) return
      const tgt = e.target as HTMLElement
      if (['INPUT', 'TEXTAREA', 'SELECT'].includes(tgt.tagName) || tgt.isContentEditable) return
      if (!localParticipant || isSelfDeafened) return
      pttActiveRef.current = true
      pttInitMicRef.current = localParticipant.isMicrophoneEnabled
      setPttInitMic(localParticipant.isMicrophoneEnabled)
      localParticipant.setMicrophoneEnabled(!pttInitMicRef.current)
      setPttVisible(true)
      e.preventDefault()
    }
    function handleKeyUp(e: KeyboardEvent) {
      if (e.code !== 'Space' || !pttActiveRef.current) return
      pttActiveRef.current = false
      localParticipant?.setMicrophoneEnabled(pttInitMicRef.current)
      setPttVisible(false)
    }
    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('keyup', handleKeyUp)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('keyup', handleKeyUp)

      // Restore original mic state if PTT was active on unmount (e.g. Space held while leaving meeting)
      if (pttActiveRef.current) {
        if (localParticipant) {
          localParticipant.setMicrophoneEnabled(pttInitMicRef.current).catch(() => {})
        }
        pttActiveRef.current = false
        setPttVisible(false)
      }
    }
  }, [localParticipant, isSelfDeafened])

  // ── Copy link feedback ──
  const [linkCopied, setLinkCopied] = useState(false)
  const linkCopiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (linkCopiedTimerRef.current) {
        clearTimeout(linkCopiedTimerRef.current)
        linkCopiedTimerRef.current = null
      }
    }
  }, [])

  return (
    <TooltipProvider delayDuration={300}>
      {/* Push-to-talk badge */}
      {pttVisible && (
        <div className="fixed bottom-20 left-1/2 z-50 flex items-center gap-2 bg-primary/90 border border-teal-400/40 rounded-full px-4 py-1.5 text-white text-xs font-semibold shadow-[0_4px_24px_color-mix(in_oklab,var(--primary)_50%,transparent)]">
          <Mic size={13} />
          {pttInitMic ? 'Push-to-Mute active' : 'Push-to-Talk active'}
        </div>
      )}

      {/* Floating controls pill */}
      <div
        id="meet-controls"
        className={cn(
          'absolute left-1/2 -translate-x-1/2 z-30 flex items-center bg-[#0c0c16]/90 backdrop-blur-xl border border-white/[0.07] whitespace-nowrap shadow-[0_8px_40px_rgba(0,0,0,0.55),inset_0_1px_0_rgba(255,255,255,0.04)]',
          isMobile
            ? 'bottom-[calc(12px+env(safe-area-inset-bottom))] gap-[2px] rounded-2xl p-1.5'
            : 'bottom-5 gap-[3px] rounded-[18px] p-2',
          'max-w-[calc(100vw-16px)]',
        )}
      >
        {/* ── Left: Video + Screen Share ── */}
        <div className="flex items-center gap-px">
          <CtrlBtn
            tip={camEnabled ? 'Disable camera' : 'Enable camera'}
            active={!camEnabled}
            isMobile={isMobile}
            onClick={() => localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})}
          >
            {camEnabled ? <Video size={iconSize} /> : <VideoOff size={iconSize} />}
          </CtrlBtn>
          {!isMobile && <DeviceSelector kind="videoinput" />}
        </div>

        <CtrlBtn
          tip={shareTip}
          danger={isScreenShareEnabled}
          isMobile={isMobile}
          onClick={
            canShare ? () => localParticipant?.setScreenShareEnabled(!isScreenShareEnabled).catch(() => {}) : undefined
          }
          className={cn(!canShare && 'opacity-40 cursor-not-allowed')}
        >
          {isScreenShareEnabled ? <MonitorOff size={iconSizeSm} /> : <MonitorUp size={iconSizeSm} />}
        </CtrlBtn>

        {/* TODO oncoming feature — recording button removed */}

        {!isMobile && <div className={dividerCn} />}

        {/* ── Center: Leave ── */}
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={onLeave}
              className={cn(
                'flex items-center gap-2 shrink-0 border-none cursor-pointer text-white text-[13px] font-semibold transition-[background,box-shadow] duration-150',
                isMobile ? 'h-[38px] rounded-[10px] px-3 mx-0.5' : 'h-11 rounded-xl px-[18px] mx-0.5',
                'bg-red-500/80 shadow-[0_2px_12px_rgba(239,68,68,0.35)] hover:bg-red-500/90',
              )}
              aria-label="Leave meeting"
            >
              <PhoneOff size={isMobile ? 15 : 16} />
              {!isMobile && 'Leave'}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top" sideOffset={8}>
            Leave meeting
          </TooltipContent>
        </Tooltip>

        {!isMobile && <div className={dividerCn} />}

        {/* ── Right: Mic + Speaker/Deafen + Combined Audio Dropdown ── */}
        <div className="flex items-center gap-px">
          <CtrlBtn
            tip={isSelfDeafened ? 'Undeafen & Unmute' : micEnabled ? 'Mute (Space)' : 'Unmute (Space)'}
            danger={!micEnabled || isSelfDeafened}
            isMobile={isMobile}
            onClick={() => {
              if (isSelfDeafened) {
                toggleSelfDeafen()
                return
              }
              localParticipant?.setMicrophoneEnabled(!micEnabled).catch(() => {})
            }}
          >
            {micEnabled ? <Mic size={iconSize} /> : <MicOff size={iconSize} />}
          </CtrlBtn>

          <CtrlBtn
            tip={isSelfDeafened ? 'Undeafen' : 'Deafen'}
            danger={isSelfDeafened}
            isMobile={isMobile}
            onClick={toggleSelfDeafen}
          >
            {isSelfDeafened ? <VolumeX size={iconSizeSm} /> : <Volume2 size={iconSizeSm} />}
          </CtrlBtn>

          {/* Combined audio dropdown: devices + noise mode + settings */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cn(
                  'flex items-center justify-center shrink-0 border-none bg-transparent cursor-pointer text-white/50 transition-colors duration-150 hover:text-white/60',
                  isMobile ? 'w-5 h-[38px]' : 'w-6 h-11',
                  'rounded-lg',
                )}
                aria-label="Audio settings"
              >
                <ChevronDown size={isMobile ? 12 : 13} />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="end" sideOffset={12} className={darkMenuCn}>
              {/* Microphone devices */}
              {mics.devices.length > 0 && (
                <>
                  <DropdownMenuLabel className="text-white/50 text-[10px] uppercase tracking-wider px-2 pt-1.5 pb-0.5">
                    Microphone
                  </DropdownMenuLabel>
                  {mics.devices.map((d, i) => (
                    <DropdownMenuItem
                      key={d.deviceId}
                      onClick={() => mics.select(d.deviceId)}
                      className="rounded-md gap-2 text-xs text-white/75 focus:text-white focus:bg-white/10"
                    >
                      <Check
                        size={12}
                        className={cn(
                          'shrink-0 text-teal-400',
                          mics.activeId === d.deviceId ? 'opacity-100' : 'opacity-0',
                        )}
                      />
                      <span className="truncate">{d.label || `Microphone ${i + 1}`}</span>
                    </DropdownMenuItem>
                  ))}
                  <DropdownMenuSeparator className="bg-white/[0.06]" />
                </>
              )}

              {/* Speaker devices */}
              {speakers.devices.length > 0 && (
                <>
                  <DropdownMenuLabel className="text-white/50 text-[10px] uppercase tracking-wider px-2 pt-1.5 pb-0.5">
                    Speaker
                  </DropdownMenuLabel>
                  {speakers.devices.map((d, i) => (
                    <DropdownMenuItem
                      key={d.deviceId}
                      onClick={() => speakers.select(d.deviceId)}
                      className="rounded-md gap-2 text-xs text-white/75 focus:text-white focus:bg-white/10"
                    >
                      <Check
                        size={12}
                        className={cn(
                          'shrink-0 text-teal-400',
                          speakers.activeId === d.deviceId ? 'opacity-100' : 'opacity-0',
                        )}
                      />
                      <span className="truncate">{d.label || `Speaker ${i + 1}`}</span>
                    </DropdownMenuItem>
                  ))}
                  <DropdownMenuSeparator className="bg-white/[0.06]" />
                </>
              )}

              {/* Noise suppression */}
              <DropdownMenuLabel className="text-white/50 text-[10px] uppercase tracking-wider px-2 pt-1.5 pb-0.5">
                Noise Suppression
              </DropdownMenuLabel>
              {NOISE_MODES.map(({ value, label }) => {
                const disabled = value === 'krisp' && !AudioProcessorService.isKrispSupported()
                return (
                  <DropdownMenuItem
                    key={value}
                    disabled={disabled}
                    onSelect={() => setMode(value)}
                    className={cn(
                      'rounded-md gap-2 text-xs text-white/75 focus:text-white focus:bg-white/10',
                      disabled && 'cursor-not-allowed',
                    )}
                  >
                    <Check
                      size={12}
                      className={cn('shrink-0 text-teal-400', noiseMode === value ? 'opacity-100' : 'opacity-0')}
                    />
                    <span className="flex-1">{label}</span>
                    {disabled && <span className="text-[9px] text-red-400 bg-red-500/15 rounded-sm px-1">N/A</span>}
                  </DropdownMenuItem>
                )
              })}

              <DropdownMenuSeparator className="bg-white/[0.06]" />

              {/* Settings link */}
              <DropdownMenuItem
                onClick={() => window.open('/dashboard/settings/audio', '_blank')}
                className="rounded-md gap-2 text-xs text-white/50 focus:text-white focus:bg-white/10"
              >
                <Settings size={12} className="shrink-0" />
                Audio settings
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {!isMobile && <div className={dividerCn} />}

        {/* ── Far right: More options ── */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className={cn(
                'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color] duration-150',
                'bg-white/[0.07] text-white/75 hover:bg-white/[0.12]',
                isMobile ? 'h-[34px] w-[34px] rounded-[10px]' : 'h-11 w-[36px] rounded-xl',
              )}
              aria-label="More options"
            >
              <MoreVertical size={iconSizeSm} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="end" sideOffset={12} className={cn(darkMenuCn, 'min-w-[200px]')}>
            <DropdownMenuItem
              onClick={() => {
                navigator.clipboard.writeText(window.location.href)
                setLinkCopied(true)
                if (linkCopiedTimerRef.current) clearTimeout(linkCopiedTimerRef.current)
                linkCopiedTimerRef.current = setTimeout(() => {
                  setLinkCopied(false)
                  linkCopiedTimerRef.current = null
                }, 2000)
              }}
              className="rounded-md gap-2 text-xs text-white/75 focus:text-white focus:bg-white/10"
            >
              {linkCopied ? (
                <Check size={13} className="shrink-0 text-emerald-400" />
              ) : (
                <Link2 size={13} className="shrink-0" />
              )}
              {linkCopied ? 'Copied!' : 'Copy room link'}
            </DropdownMenuItem>

            {typeof document !== 'undefined' && document.fullscreenEnabled && (
              <DropdownMenuItem
                onClick={() => {
                  if (document.fullscreenElement) document.exitFullscreen()
                  else document.documentElement.requestFullscreen()
                }}
                className="rounded-md gap-2 text-xs text-white/75 focus:text-white focus:bg-white/10"
              >
                <Maximize size={13} className="shrink-0" />
                {typeof document !== 'undefined' && document.fullscreenElement ? 'Exit fullscreen' : 'Fullscreen'}
              </DropdownMenuItem>
            )}

            <DropdownMenuSeparator className="bg-white/[0.06]" />

            <DropdownMenuItem disabled className="rounded-md gap-2 text-[11px] text-white/50 focus:text-white/50">
              <Keyboard size={12} className="shrink-0" />
              Space — Push to talk/mute
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </TooltipProvider>
  )
}
