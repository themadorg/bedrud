// TODO oncoming feature
import { useLocalParticipant, useRoomContext } from '@livekit/components-react'

import { ConnectionState, RoomEvent } from 'livekit-client'
import {
  Check,
  ChevronDown,
  Film,
  Link2,
  Maximize,
  Mic,
  MicOff,
  MonitorOff,
  MonitorUp,
  MoreVertical,
  PenLine,
  PhoneOff,
  Settings,
  Video,
  VideoOff,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { BedrudSettingsDialog } from '#/components/settings/BedrudSettingsDialog'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { useMeetingMicKeyboard } from '#/components/meeting/useMeetingMicKeyboard'
import { type NoiseSuppressionMode, useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { AudioProcessorService } from '#/lib/audio-processor.service'
import { useAuthStore } from '#/lib/auth.store'
import { readMeetingDeviceId, writeMeetingDeviceId } from '#/lib/meeting-device-storage'
import { cn } from '#/lib/utils'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { meetControlsDockClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import { waitForScreenSharePublication } from '@/components/meeting/stage/waitForScreenShare'
import { stageOwnerLabel } from '@/components/meeting/stage/stageWire'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useWhiteboardWatch } from '@/components/meeting/whiteboard/whiteboard-watch-context'
import { useYoutubeWatch } from '@/components/meeting/youtube/youtube-watch-context'
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

function btnIconCn(active = false, danger = false, ptt = false, isMobile = false) {
  return cn(
    'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color,box-shadow,border-color] duration-150',
    isMobile ? 'h-[38px] w-[38px] rounded-[10px]' : 'h-11 w-11 rounded-xl',
    ptt
      ? 'meet-ptt-btn'
      : danger
        ? 'bg-[var(--meet-btn-alert-bg)] text-[var(--meet-btn-alert-fg)] hover:bg-[var(--meet-btn-alert-hover)]'
        : active
          ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)] hover:bg-[var(--meet-btn-muted-hover)]'
          : 'bg-[var(--meet-control)] text-[var(--meet-control-fg)] hover:bg-[var(--meet-control-hover)]',
  )
}

function PushToTalkIcon({ size, speaking }: { size: number; speaking: boolean }) {
  if (speaking) return <Mic size={size} />
  return (
    <span className="flex flex-col items-center gap-0.5 leading-none">
      <Mic size={Math.max(size - 2, 14)} strokeWidth={2.25} />
      <span className="text-[8px] font-bold uppercase tracking-[0.12em] text-[var(--meet-btn-muted-fg)]">ptt</span>
    </span>
  )
}

function CtrlBtn({
  tip,
  active = false,
  danger = false,
  ptt = false,
  isMobile = false,
  className,
  onClick,
  onPointerDown,
  onPointerUp,
  onPointerLeave,
  onPointerCancel,
  children,
}: {
  tip: string
  active?: boolean
  danger?: boolean
  ptt?: boolean
  isMobile?: boolean
  className?: string
  onClick?: () => void
  onPointerDown?: (event: React.PointerEvent<HTMLButtonElement>) => void
  onPointerUp?: (event: React.PointerEvent<HTMLButtonElement>) => void
  onPointerLeave?: (event: React.PointerEvent<HTMLButtonElement>) => void
  onPointerCancel?: (event: React.PointerEvent<HTMLButtonElement>) => void
  children: React.ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={onClick}
          onPointerDown={onPointerDown}
          onPointerUp={onPointerUp}
          onPointerLeave={onPointerLeave}
          onPointerCancel={onPointerCancel}
          className={cn(btnIconCn(active, danger, ptt, isMobile), className)}
          aria-label={tip}
          aria-pressed={active}
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

const dividerCn = 'w-px h-7 bg-[var(--meet-border)] mx-0.5 shrink-0 max-sm:hidden'

const meetMenuCn =
  'meet-dialog min-w-60 max-w-[calc(100vw-24px)] rounded-xl border border-[var(--meet-border-subtle)] !bg-[var(--meet-bg-panel)] !text-[var(--meet-fg)] shadow-[var(--meet-shadow)] backdrop-blur-xl'

const meetMenuItemCn =
  'rounded-md gap-2 text-xs !text-[var(--meet-control-fg)] focus:!bg-[var(--meet-control-hover)] focus:!text-[var(--meet-fg)] data-[highlighted]:!bg-[var(--meet-control-hover)] data-[highlighted]:!text-[var(--meet-fg)]'

const meetMenuLabelCn =
  'px-2 pb-0.5 pt-1.5 text-[10px] font-medium uppercase tracking-wider !text-[var(--meet-fg-muted)]'

const meetMenuSeparatorCn = '!bg-[var(--meet-border-subtle)]'

const NOISE_MODES: { value: NoiseSuppressionMode; label: string }[] = [
  { value: 'none', label: 'Off' },
  { value: 'browser', label: 'Browser' },
  { value: 'rnnoise', label: 'RNNoise' },
  { value: 'krisp', label: 'Krisp AI' },
]

/* ── Device list hook (replaces individual DeviceSelector for audio) ──────── */

function useDeviceList(kind: 'audioinput' | 'audiooutput') {
  const room = useRoomContext()
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [activeId, setActiveId] = useState(() => readMeetingDeviceId(kind))

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
    const saved = readMeetingDeviceId(kind)

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
      writeMeetingDeviceId(kind, deviceId)
    },
    [room, kind],
  )

  return { devices, activeId, select }
}

/* ── ControlsBar ──────────────────────────────────────────────────────────── */

export function ControlsBar({ onLeave }: Props) {
  const isMobile = useIsMobile()
  const layout = useMeetingUILayout()
  const { stage, isOwner, claimStage, clearStage } = useMeetingStage()
  const isWhiteboardHost = stage?.kind === 'whiteboard' && isOwner
  const whiteboardEnabled = useExperimentalPreferencesStore((s) => s.whiteboardEnabled)
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)
  const { requestStartWhiteboard } = useWhiteboardWatch()
  const { isHost: isYoutubeHost, openShareDialog, stopShare: stopYoutubeShare } = useYoutubeWatch()
  const {
    localParticipant,
    isMicrophoneEnabled: micEnabled,
    isCameraEnabled: camEnabled,
    isScreenShareEnabled,
  } = useLocalParticipant()
  const stageTakenByOther = Boolean(stage && !isOwner)
  const { isSelfDeafened, toggleSelfDeafen } = useMeetingRoomContext()

  const tokens = useAuthStore((s) => s.tokens)
  const canShare = Boolean(tokens) && Boolean(navigator.mediaDevices?.getDisplayMedia)
  const shareTip = !tokens
    ? 'Sign in to share screen'
    : !navigator.mediaDevices?.getDisplayMedia
      ? 'Screen sharing not supported'
      : stageTakenByOther
        ? `${stage ? stageOwnerLabel(stage) : 'Someone'} is on stage`
        : isScreenShareEnabled
          ? 'Stop sharing'
          : 'Share screen'

  const noiseMode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const setMode = useAudioPreferencesStore((s) => s.setMode)

  const mics = useDeviceList('audioinput')
  const speakers = useDeviceList('audiooutput')

  const iconSize = isMobile ? 16 : 18
  const iconSizeSm = isMobile ? 15 : 17

  const { pttVisible, pttAvailable, micUiEnabled, micTip, pttTip, pushToTalkEnabled, toggleMic, startPtt, stopPtt } =
    useMeetingMicKeyboard(localParticipant, isSelfDeafened, micEnabled)

  // ── Copy link feedback ──
  const [linkCopied, setLinkCopied] = useState(false)
  const linkCopiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [settingsOpen, setSettingsOpen] = useState(false)

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
      {/* Floating controls pill */}
      <div
        id="meet-controls"
        className={cn(
          'absolute -translate-x-1/2 z-30 flex items-center bg-[var(--meet-chrome)] backdrop-blur-xl border border-[var(--meet-border-subtle)] whitespace-nowrap shadow-[var(--meet-shadow),var(--meet-shadow-inset)] transition-[left] duration-200',
          meetControlsDockClass(layout),
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
            canShare && !stageTakenByOther
              ? async () => {
                  if (isScreenShareEnabled) {
                    await localParticipant?.setScreenShareEnabled(false).catch(() => {})
                    if (isOwner && stage?.kind === 'screenshare') clearStage()
                    return
                  }
                  try {
                    const err = claimStage('screenshare')
                    if (err) {
                      toast.error(err)
                      return
                    }
                    await localParticipant?.setScreenShareEnabled(true)
                    const ready = localParticipant
                      ? await waitForScreenSharePublication(localParticipant)
                      : false
                    if (!ready) {
                      clearStage()
                      await localParticipant?.setScreenShareEnabled(false).catch(() => {})
                      toast.error('Screen share track did not start')
                    }
                  } catch {
                    clearStage()
                    await localParticipant?.setScreenShareEnabled(false).catch(() => {})
                    toast.error('Could not start screen sharing')
                  }
                }
              : undefined
          }
          className={cn((!canShare || stageTakenByOther) && 'opacity-40 cursor-not-allowed')}
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
                'flex items-center gap-2 shrink-0 border-none cursor-pointer text-[var(--meet-btn-leave-fg)] text-[13px] font-semibold transition-[background,box-shadow] duration-150',
                isMobile ? 'h-[38px] rounded-[10px] px-3 mx-0.5' : 'h-11 rounded-xl px-[18px] mx-0.5',
                'bg-[var(--meet-btn-leave-bg)] shadow-[0_2px_12px_color-mix(in_oklab,var(--meet-btn-leave-bg)_45%,transparent)] hover:bg-[var(--meet-btn-leave-hover)]',
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
          {pushToTalkEnabled && (
            <CtrlBtn
              tip={pttTip}
              active={pttVisible}
              ptt={pttAvailable && !pttVisible}
              isMobile={isMobile}
              className={cn(!pttAvailable && 'opacity-40 cursor-not-allowed')}
              onPointerDown={(event) => {
                if (!pttAvailable) return
                event.currentTarget.setPointerCapture(event.pointerId)
                startPtt()
              }}
              onPointerUp={(event) => {
                if (!pushToTalkEnabled) return
                if (event.currentTarget.hasPointerCapture(event.pointerId)) {
                  event.currentTarget.releasePointerCapture(event.pointerId)
                }
                stopPtt()
              }}
              onPointerLeave={() => {
                if (pushToTalkEnabled) stopPtt()
              }}
              onPointerCancel={() => {
                if (pushToTalkEnabled) stopPtt()
              }}
            >
              <PushToTalkIcon size={iconSize} speaking={pttVisible} />
            </CtrlBtn>
          )}

          <CtrlBtn
            tip={micTip}
            danger={isSelfDeafened || !micUiEnabled}
            isMobile={isMobile}
            onClick={() => {
              if (isSelfDeafened) {
                toggleSelfDeafen()
                return
              }
              toggleMic()
            }}
          >
            {isSelfDeafened || !micUiEnabled ? (
              <MicOff size={iconSize} />
            ) : (
              <Mic size={iconSize} />
            )}
          </CtrlBtn>

          <CtrlBtn
            tip={isSelfDeafened ? 'Undeafen' : 'Deafen'}
            danger={isSelfDeafened}
            isMobile={isMobile}
            onClick={toggleSelfDeafen}
          >
            <DeafenHeadphonesIcon size={iconSizeSm} off={isSelfDeafened} />
          </CtrlBtn>

          {/* Combined audio dropdown: devices + noise mode + settings */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cn(
                  'flex items-center justify-center shrink-0 border-none bg-transparent cursor-pointer text-[var(--meet-fg-muted)] transition-colors duration-150 hover:text-[var(--meet-fg-strong)]',
                  isMobile ? 'w-5 h-[38px]' : 'w-6 h-11',
                  'rounded-lg',
                )}
                aria-label="Audio settings"
              >
                <ChevronDown size={isMobile ? 12 : 13} />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="end" sideOffset={12} className={meetMenuCn}>
              {/* Microphone devices */}
              {mics.devices.length > 0 && (
                <>
                  <DropdownMenuLabel className={meetMenuLabelCn}>
                    Microphone
                  </DropdownMenuLabel>
                  {mics.devices.map((d, i) => (
                    <DropdownMenuItem
                      key={d.deviceId}
                      onClick={() => mics.select(d.deviceId)}
                      className={meetMenuItemCn}
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
                  <DropdownMenuSeparator className={meetMenuSeparatorCn} />
                </>
              )}

              {/* Speaker devices */}
              {speakers.devices.length > 0 && (
                <>
                  <DropdownMenuLabel className={meetMenuLabelCn}>
                    Speaker
                  </DropdownMenuLabel>
                  {speakers.devices.map((d, i) => (
                    <DropdownMenuItem
                      key={d.deviceId}
                      onClick={() => speakers.select(d.deviceId)}
                      className={meetMenuItemCn}
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
                  <DropdownMenuSeparator className={meetMenuSeparatorCn} />
                </>
              )}

              {/* Noise suppression */}
              <DropdownMenuLabel className={meetMenuLabelCn}>
                Noise Suppression
              </DropdownMenuLabel>
              {NOISE_MODES.map(({ value, label }) => {
                const disabled = value === 'krisp' && !AudioProcessorService.isKrispSupported()
                return (
                  <DropdownMenuItem
                    key={value}
                    disabled={disabled}
                    onSelect={() => setMode(value)}
                    className={cn(meetMenuItemCn, disabled && 'cursor-not-allowed')}
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
                'bg-[var(--meet-control)] text-[var(--meet-control-fg)] hover:bg-[var(--meet-control-hover)]',
                isMobile ? 'h-[34px] w-[34px] rounded-[10px]' : 'h-11 w-[36px] rounded-xl',
              )}
              aria-label="More options"
            >
              <MoreVertical size={iconSizeSm} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="end" sideOffset={12} className={cn(meetMenuCn, 'min-w-[200px]')}>
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
              className={meetMenuItemCn}
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
                className={meetMenuItemCn}
              >
                <Maximize size={13} className="shrink-0" />
                {typeof document !== 'undefined' && document.fullscreenElement ? 'Exit fullscreen' : 'Fullscreen'}
              </DropdownMenuItem>
            )}

            <DropdownMenuItem
              onClick={() => setSettingsOpen(true)}
              className={meetMenuItemCn}
            >
              <Settings size={13} className="shrink-0" />
              Settings
            </DropdownMenuItem>

            {(whiteboardEnabled || (stage?.kind === 'whiteboard' && isWhiteboardHost)) && (
              <>
                <DropdownMenuSeparator className={meetMenuSeparatorCn} />

                {stage?.kind === 'whiteboard' && isWhiteboardHost ? (
                  <DropdownMenuItem
                    onClick={() => clearStage()}
                    className={meetMenuItemCn}
                  >
                    <PenLine size={13} className="shrink-0 text-primary" />
                    Close whiteboard
                  </DropdownMenuItem>
                ) : (
                  whiteboardEnabled && (
                    <DropdownMenuItem
                      onClick={() => {
                        const err = requestStartWhiteboard()
                        if (err) toast.error(err)
                      }}
                      disabled={stageTakenByOther}
                      className={meetMenuItemCn}
                    >
                      <PenLine size={13} className="shrink-0 text-primary" />
                      {stage?.kind === 'whiteboard' ? 'Whiteboard on stage' : 'Open whiteboard'}
                    </DropdownMenuItem>
                  )
                )}
              </>
            )}

            {(youtubeEnabled || (stage?.kind === 'youtube' && isYoutubeHost)) &&
              (stage?.kind === 'youtube' && isYoutubeHost ? (
                <DropdownMenuItem
                  onClick={() => stopYoutubeShare()}
                  className={meetMenuItemCn}
                >
                  <Film size={13} className="shrink-0 text-red-400" />
                  Stop YouTube
                </DropdownMenuItem>
              ) : (
                youtubeEnabled && (
                  <DropdownMenuItem
                    onClick={() => openShareDialog()}
                    disabled={stageTakenByOther}
                    className={meetMenuItemCn}
                  >
                    <Film size={13} className="shrink-0 text-red-400" />
                    {stage?.kind === 'youtube' ? 'YouTube on stage' : 'Share YouTube'}
                  </DropdownMenuItem>
                )
              ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <BedrudSettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </TooltipProvider>
  )
}
