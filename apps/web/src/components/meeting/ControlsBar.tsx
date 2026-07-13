// TODO oncoming feature
import { useLocalParticipant, useRoomContext } from '@livekit/components-react'

import { ConnectionState, RoomEvent } from 'livekit-client'
import {
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Film,
  Globe,
  Info,
  Link2,
  Lock,
  Maximize,
  Mic,
  MicOff,
  MonitorOff,
  MonitorUp,
  MoreVertical,
  Package,
  PenLine,
  PhoneOff,
  Settings,
  Video,
  VideoOff,
  X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { useMeetingMicKeyboard } from '#/components/meeting/useMeetingMicKeyboard'
import { BedrudSettingsDialog } from '#/components/settings/BedrudSettingsDialog'
import { type NoiseSuppressionMode, useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { AudioProcessorService, audioProcessorService } from '#/lib/audio-processor.service'
import { useAuthStore } from '#/lib/auth.store'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { readMeetingDeviceId, writeMeetingDeviceId } from '#/lib/meeting-device-storage'
import { getPublicSettings, refreshPublicSettings } from '#/lib/use-public-settings'
import { useRequestNoiseMode } from '#/lib/use-request-noise-mode'
import { cn } from '#/lib/utils'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import {
  isWebxdcExpandSource,
  MEETING_CLOSE_ELEVATED_CHROME,
  MEETING_CLOSE_SETTINGS,
  MEETING_OPEN_SETTINGS,
  publishMeetingChromeState,
} from '@/components/meeting/meetingChromeEvents'
import { meetControlsDockClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { RoomInfoContent } from '@/components/meeting/RoomInfoPanel'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import { stageOwnerLabel } from '@/components/meeting/stage/stageWire'
import { waitForScreenSharePublication } from '@/components/meeting/stage/waitForScreenShare'
import { WebxdcAppsDialog } from '@/components/meeting/webxdc/WebxdcAppsDialog'
import { useWhiteboardWatch } from '@/components/meeting/whiteboard/whiteboard-watch-context'
import { useYoutubeWatch } from '@/components/meeting/youtube/youtube-watch-context'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

/** Extra ⋯ menu items (merged into the single bottom More menu — no second ⋯). */
export interface ControlsBarMoreExtras {
  onRoomAccess?: () => void
  isPublic?: boolean
  /** Desktop only — mobile Room info is an in-sheet sub-page. */
  onRoomInfo?: () => void
  /** Required for mobile Room info sub-page. */
  roomId?: string
  onToggleVideoSidebar?: () => void
  showVideoSidebarToggle?: boolean
  videoSidebarOpen?: boolean
}

interface Props {
  onLeave: () => void
  /** Mobile room-chrome actions live in the existing bottom ⋯ (not a second top ⋯). */
  moreExtras?: ControlsBarMoreExtras
}

/* ── Mobile detection ──────────────────────────────────────────────────────── */

function useIsMobile(breakpoint = 640) {
  const [mobile, setMobile] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(`(max-width: ${breakpoint - 1}px)`).matches : false,
  )
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint - 1}px)`)
    const onChange = () => setMobile(mq.matches)
    onChange()
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
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
  'meet-dialog min-w-60 max-w-[calc(var(--app-width,100svw)-24px)] rounded-xl border border-[var(--meet-border-subtle)] !bg-[var(--meet-bg-panel)] !text-[var(--meet-fg)] shadow-[var(--meet-shadow)] backdrop-blur-xl'

const meetMenuItemCn =
  'rounded-md gap-2 text-xs !text-[var(--meet-control-fg)] focus:!bg-[var(--meet-control-hover)] focus:!text-[var(--meet-fg)] data-[highlighted]:!bg-[var(--meet-control-hover)] data-[highlighted]:!text-[var(--meet-fg)]'

const meetMenuLabelCn =
  'px-2 pb-0.5 pt-1.5 text-[10px] font-medium uppercase tracking-wider !text-[var(--meet-fg-muted)]'

const meetMenuSeparatorCn = '!bg-[var(--meet-border-subtle)]'

const NOISE_MODES: { value: NoiseSuppressionMode; label: string }[] = [
  { value: 'none', label: 'Off' },
  { value: 'browser', label: 'Browser' },
  { value: 'rnnoise', label: 'RNNoise' },
  { value: 'krisp', label: 'Krisp' },
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

export function ControlsBar({ onLeave, moreExtras }: Props) {
  const isMobile = useIsMobile()
  const layout = useMeetingUILayout()
  const { stage, isOwner, claimStage, clearStage } = useMeetingStage()
  const isWhiteboardHost = stage?.kind === 'whiteboard' && isOwner
  const isWebxdcOnStage = stage?.kind === 'webxdc'
  const whiteboardEnabled = useExperimentalPreferencesStore((s) => s.whiteboardEnabled)
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)
  const webxdcEnabled = useExperimentalPreferencesStore((s) => s.webxdcEnabled)
  const { requestStartWhiteboard } = useWhiteboardWatch()
  const { isHost: isYoutubeHost, openShareDialog, stopShare: stopYoutubeShare } = useYoutubeWatch()
  const {
    localParticipant,
    isMicrophoneEnabled: micEnabled,
    isCameraEnabled: camEnabled,
    isScreenShareEnabled,
  } = useLocalParticipant()
  const stageTakenByOther = Boolean(stage && !isOwner)
  const { isSelfDeafened, toggleSelfDeafen, roomId, getParticipantDisplayName } = useMeetingRoomContext()
  const [webxdcAppsOpen, setWebxdcAppsOpen] = useState(false)
  const selfName =
    getParticipantDisplayName(localParticipant) ||
    localParticipant.name ||
    localParticipant.identity ||
    'You'

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
  const [rnnoiseAllowed, setRnnoiseAllowed] = useState(false)
  const [krispAllowed, setKrispAllowed] = useState(false)
  useEffect(() => {
    refreshPublicSettings()
    void getPublicSettings().then((s) => {
      const rn = !!s.rnnoiseEnabled
      const kr = !!s.krispEnabled
      setRnnoiseAllowed(rn)
      setKrispAllowed(kr)
      audioProcessorService.setNoisePackageAllowed({ rnnoise: rn, krisp: kr })
    })
  }, [])
  const { requestMode } = useRequestNoiseMode({ rnnoiseAllowed, krispAllowed })
  useEffect(() => {
    if ((noiseMode === 'rnnoise' && !rnnoiseAllowed) || (noiseMode === 'krisp' && !krispAllowed)) {
      setMode('browser')
    }
  }, [noiseMode, rnnoiseAllowed, krispAllowed, setMode])
  // Hide RNNoise/Krisp entirely when instance admin has not enabled them.
  const noiseModes = useMemo(
    () =>
      NOISE_MODES.filter((m) => {
        if (m.value === 'rnnoise') return rnnoiseAllowed
        if (m.value === 'krisp') return krispAllowed
        return true
      }),
    [rnnoiseAllowed, krispAllowed],
  )

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
  const [settingsElevated, setSettingsElevated] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [audioOpen, setAudioOpen] = useState(false)
  /** Mobile More drill-down: null = root list. */
  const [morePage, setMorePage] = useState<'info' | null>(null)
  const [moreNavDir, setMoreNavDir] = useState<'forward' | 'back'>('forward')

  useEffect(() => {
    return () => {
      if (linkCopiedTimerRef.current) {
        clearTimeout(linkCopiedTimerRef.current)
        linkCopiedTimerRef.current = null
      }
    }
  }, [])

  // WebXDC left rail / other chrome can open settings without prop drilling.
  useEffect(() => {
    const onSettings = (e: Event) => {
      const fromWx = isWebxdcExpandSource((e as CustomEvent).detail)
      // Toggle: second click on left-rail settings while elevated → close.
      if (fromWx && settingsOpen && settingsElevated) {
        setSettingsOpen(false)
        setSettingsElevated(false)
        publishMeetingChromeState(null)
        return
      }
      setSettingsElevated(fromWx)
      setSettingsOpen(true)
      if (fromWx) publishMeetingChromeState('settings')
    }
    const onClose = () => {
      setSettingsOpen(false)
      setSettingsElevated(false)
    }
    const onCloseElevated = () => {
      if (!settingsElevated) return
      setSettingsOpen(false)
      setSettingsElevated(false)
      publishMeetingChromeState(null)
    }
    window.addEventListener(MEETING_OPEN_SETTINGS, onSettings)
    window.addEventListener(MEETING_CLOSE_SETTINGS, onClose)
    window.addEventListener(MEETING_CLOSE_ELEVATED_CHROME, onCloseElevated)
    return () => {
      window.removeEventListener(MEETING_OPEN_SETTINGS, onSettings)
      window.removeEventListener(MEETING_CLOSE_SETTINGS, onClose)
      window.removeEventListener(MEETING_CLOSE_ELEVATED_CHROME, onCloseElevated)
    }
  }, [settingsOpen, settingsElevated])

  const copyRoomLink = useCallback(() => {
    void navigator.clipboard
      .writeText(window.location.href)
      .then(() => {
        setLinkCopied(true)
        toast.success('Meeting link copied', {
          description: 'Share it so others can join this room.',
        })
        if (linkCopiedTimerRef.current) clearTimeout(linkCopiedTimerRef.current)
        linkCopiedTimerRef.current = setTimeout(() => {
          setLinkCopied(false)
          linkCopiedTimerRef.current = null
        }, 2000)
      })
      .catch(() => {
        toast.error('Could not copy link', {
          description: 'Check clipboard permissions and try again.',
        })
      })
  }, [])

  const toggleFullscreen = useCallback(() => {
    if (typeof document === 'undefined' || !document.fullscreenEnabled) return
    if (document.fullscreenElement) void document.exitFullscreen()
    else void document.documentElement.requestFullscreen()
  }, [])

  type MoreRow =
    | { kind: 'action'; id: string; label: string; icon: React.ReactNode; disabled?: boolean; onSelect: () => void }
    | { kind: 'separator'; id: string }

  const moreRows: MoreRow[] = useMemo(() => {
    const rows: MoreRow[] = []

    if (isMobile && moreExtras) {
      if (moreExtras.showVideoSidebarToggle && moreExtras.onToggleVideoSidebar) {
        rows.push({
          kind: 'action',
          id: 'videos',
          label: moreExtras.videoSidebarOpen ? 'Hide videos' : 'Show videos',
          icon: <Video size={18} className="shrink-0" />,
          onSelect: () => moreExtras.onToggleVideoSidebar?.(),
        })
      }
      if (moreExtras.onRoomAccess) {
        rows.push({
          kind: 'action',
          id: 'access',
          label: moreExtras.isPublic ? 'Public room' : 'Private room',
          icon: moreExtras.isPublic ? (
            <Globe size={18} className="shrink-0 text-accent-400" />
          ) : (
            <Lock size={18} className="shrink-0" />
          ),
          onSelect: () => moreExtras.onRoomAccess?.(),
        })
      }
      // Room info: in-sheet sub-page on mobile (not a second dialog).
      if (moreExtras.roomId) {
        rows.push({
          kind: 'action',
          id: 'info',
          label: 'Room info',
          icon: <Info size={18} className="shrink-0" />,
          onSelect: () => {
            setMoreNavDir('forward')
            setMorePage('info')
          },
        })
      }
      if (rows.length > 0) rows.push({ kind: 'separator', id: 'sep-room' })
    }

    rows.push({
      kind: 'action',
      id: 'copy',
      label: linkCopied ? 'Copied!' : 'Copy room link',
      icon: linkCopied ? (
        <Check size={18} className="shrink-0 text-emerald-400" />
      ) : (
        <Link2 size={18} className="shrink-0" />
      ),
      onSelect: copyRoomLink,
    })

    if (typeof document !== 'undefined' && document.fullscreenEnabled) {
      rows.push({
        kind: 'action',
        id: 'fullscreen',
        label: typeof document !== 'undefined' && document.fullscreenElement ? 'Exit fullscreen' : 'Fullscreen',
        icon: <Maximize size={18} className="shrink-0" />,
        onSelect: toggleFullscreen,
      })
    }

    rows.push({
      kind: 'action',
      id: 'settings',
      label: 'Settings',
      icon: <Settings size={18} className="shrink-0" />,
      onSelect: () => setSettingsOpen(true),
    })

    const hasWhiteboard = whiteboardEnabled || (stage?.kind === 'whiteboard' && isWhiteboardHost)
    if (hasWhiteboard) {
      rows.push({ kind: 'separator', id: 'sep-wb' })
      if (stage?.kind === 'whiteboard' && isWhiteboardHost) {
        rows.push({
          kind: 'action',
          id: 'wb-close',
          label: 'Close whiteboard',
          icon: <PenLine size={18} className="shrink-0 text-primary" />,
          onSelect: () => clearStage(),
        })
      } else if (whiteboardEnabled) {
        rows.push({
          kind: 'action',
          id: 'wb-open',
          label: stage?.kind === 'whiteboard' ? 'Whiteboard on stage' : 'Open whiteboard',
          icon: <PenLine size={18} className="shrink-0 text-primary" />,
          disabled: stageTakenByOther,
          onSelect: () => {
            const err = requestStartWhiteboard()
            if (err) toast.error(err)
          },
        })
      }
    }

    const hasYoutube = youtubeEnabled || (stage?.kind === 'youtube' && isYoutubeHost)
    if (hasYoutube) {
      if (stage?.kind === 'youtube' && isYoutubeHost) {
        rows.push({
          kind: 'action',
          id: 'yt-stop',
          label: 'Stop YouTube',
          icon: <Film size={18} className="shrink-0 text-red-400" />,
          onSelect: () => stopYoutubeShare(),
        })
      } else if (youtubeEnabled) {
        rows.push({
          kind: 'action',
          id: 'yt-share',
          label: stage?.kind === 'youtube' ? 'YouTube on stage' : 'Share YouTube',
          icon: <Film size={18} className="shrink-0 text-red-400" />,
          disabled: stageTakenByOther,
          onSelect: () => openShareDialog(),
        })
      }
    }

    if (webxdcEnabled) {
      rows.push({ kind: 'separator', id: 'sep-webxdc' })
      rows.push({
        kind: 'action',
        id: 'webxdc-apps',
        label: isWebxdcOnStage ? 'Gallery (on stage)' : 'App gallery',
        icon: <Package size={18} className="shrink-0 text-amber-400" />,
        onSelect: () => setWebxdcAppsOpen(true),
      })
    }

    return rows
  }, [
    isMobile,
    moreExtras,
    linkCopied,
    copyRoomLink,
    toggleFullscreen,
    whiteboardEnabled,
    youtubeEnabled,
    webxdcEnabled,
    isWebxdcOnStage,
    stage,
    isWhiteboardHost,
    isYoutubeHost,
    stageTakenByOther,
    clearStage,
    requestStartWhiteboard,
    stopYoutubeShare,
    openShareDialog,
  ])

  const runMoreAction = useCallback((row: Extract<MoreRow, { kind: 'action' }>) => {
    if (row.disabled) return
    // In-sheet pages (room info): stay inside More.
    if (row.id === 'info') {
      row.onSelect()
      return
    }
    // Nested dialogs (Settings, room access): open first, close More same turn.
    row.onSelect()
    setMoreOpen(false)
  }, [])

  useEffect(() => {
    if (!moreOpen) {
      setMorePage(null)
      setMoreNavDir('forward')
    }
  }, [moreOpen])

  const morePageAnim =
    moreNavDir === 'forward'
      ? 'animate-in fade-in-0 slide-in-from-right duration-200 ease-out'
      : 'animate-in fade-in-0 slide-in-from-left duration-200 ease-out'

  return (
    <TooltipProvider delayDuration={300}>
      {/* Floating controls pill */}
      <div
        id="meet-controls"
        className={cn(
          // meet-controls-bar: border-radius needs !important (global * { border-radius: 0 })
          'meet-controls-bar absolute -translate-x-1/2 z-30 flex items-center bg-[var(--meet-chrome)] backdrop-blur-xl border border-[var(--meet-border-subtle)] whitespace-nowrap shadow-[var(--meet-shadow),var(--meet-shadow-inset)] transition-[left] duration-200',
          meetControlsDockClass(layout),
          isMobile
            ? 'bottom-[calc(12px+env(safe-area-inset-bottom))] gap-[2px] p-1.5'
            : 'bottom-5 gap-[3px] p-2',
          'max-w-[calc(var(--app-width,100svw)-16px)]',
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
                    const ready = localParticipant ? await waitForScreenSharePublication(localParticipant) : false
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

        {webxdcEnabled ? (
          <CtrlBtn
            tip={isWebxdcOnStage ? 'App gallery (on stage)' : 'App gallery'}
            active={isWebxdcOnStage}
            isMobile={isMobile}
            onClick={() => setWebxdcAppsOpen(true)}
          >
            <Package size={iconSize} />
          </CtrlBtn>
        ) : null}

        {/* TODO oncoming feature — recording button removed */}

        {!isMobile && <div className={dividerCn} />}

        {/* ── Center: Leave ── */}
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={onLeave}
              className={cn(
                // meet-btn-leave: border-radius needs !important (global * { border-radius: 0 })
                'meet-btn-leave flex items-center gap-2 shrink-0 border-none cursor-pointer text-[var(--meet-btn-leave-fg)] text-[13px] font-semibold transition-[background,box-shadow] duration-150',
                isMobile ? 'h-[38px] px-3 mx-0.5' : 'h-11 px-[18px] mx-0.5',
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
            {isSelfDeafened || !micUiEnabled ? <MicOff size={iconSize} /> : <Mic size={iconSize} />}
          </CtrlBtn>

          <CtrlBtn
            tip={isSelfDeafened ? 'Undeafen' : 'Deafen'}
            danger={isSelfDeafened}
            isMobile={isMobile}
            onClick={toggleSelfDeafen}
          >
            <DeafenHeadphonesIcon size={iconSizeSm} off={isSelfDeafened} />
          </CtrlBtn>

          {/* Audio devices + noise: mobile full-screen sheet, desktop dropdown */}
          {isMobile ? (
            <button
              type="button"
              onClick={() => setAudioOpen(true)}
              className={cn(
                'flex h-[38px] w-5 shrink-0 items-center justify-center rounded-lg border-none bg-transparent cursor-pointer text-[var(--meet-fg-muted)] transition-colors duration-150 hover:text-[var(--meet-fg-strong)]',
              )}
              aria-label="Audio settings"
            >
              <ChevronDown size={12} />
            </button>
          ) : (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className={cn(
                    'flex h-11 w-6 shrink-0 items-center justify-center rounded-lg border-none bg-transparent cursor-pointer text-[var(--meet-fg-muted)] transition-colors duration-150 hover:text-[var(--meet-fg-strong)]',
                  )}
                  aria-label="Audio settings"
                >
                  <ChevronDown size={13} />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="end" sideOffset={12} className={meetMenuCn}>
                {mics.devices.length > 0 && (
                  <>
                    <DropdownMenuLabel className={meetMenuLabelCn}>Microphone</DropdownMenuLabel>
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

                {speakers.devices.length > 0 && (
                  <>
                    <DropdownMenuLabel className={meetMenuLabelCn}>Speaker</DropdownMenuLabel>
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

                <DropdownMenuLabel className={meetMenuLabelCn}>Noise Suppression</DropdownMenuLabel>
                {noiseModes.map(({ value, label }) => {
                  const disabled = value === 'krisp' && !AudioProcessorService.isKrispSupported()
                  return (
                    <DropdownMenuItem
                      key={value}
                      disabled={disabled}
                      onSelect={() => requestMode(value)}
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
          )}
        </div>

        {!isMobile && <div className={dividerCn} />}

        {/* ── Far right: More options ── */}
        {isMobile ? (
          <button
            type="button"
            onClick={() => setMoreOpen(true)}
            className={cn(
              'flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[10px] border-none cursor-pointer transition-[background,color] duration-150',
              'bg-[var(--meet-control)] text-[var(--meet-control-fg)] hover:bg-[var(--meet-control-hover)]',
            )}
            aria-label="More options"
          >
            <MoreVertical size={iconSizeSm} />
          </button>
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cn(
                  'flex h-11 w-[36px] shrink-0 items-center justify-center rounded-xl border-none cursor-pointer transition-[background,color] duration-150',
                  'bg-[var(--meet-control)] text-[var(--meet-control-fg)] hover:bg-[var(--meet-control-hover)]',
                )}
                aria-label="More options"
              >
                <MoreVertical size={iconSizeSm} />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="end" sideOffset={12} className={cn(meetMenuCn, 'min-w-[200px]')}>
              {moreRows.map((row) => {
                if (row.kind === 'separator') {
                  return <DropdownMenuSeparator key={row.id} className={meetMenuSeparatorCn} />
                }
                return (
                  <DropdownMenuItem
                    key={row.id}
                    disabled={row.disabled}
                    onClick={() => row.onSelect()}
                    className={cn(meetMenuItemCn, row.disabled && 'cursor-not-allowed opacity-50')}
                  >
                    <span className="flex h-4 w-4 shrink-0 items-center justify-center [&>svg]:h-[13px] [&>svg]:w-[13px]">
                      {row.icon}
                    </span>
                    {row.label}
                  </DropdownMenuItem>
                )
              })}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Mobile: audio devices + noise (full-screen, like More / Settings). */}
      <Dialog
        open={audioOpen && isMobile}
        onOpenChange={(open) => {
          if (!open) setAudioOpen(false)
        }}
      >
        <DialogContent
          className={cn(
            'meet-dialog flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
            // Visual viewport full-screen (iOS Safari toolbar-safe)
            'fixed left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] max-h-[var(--app-height,100svh)] w-[var(--app-width,100svw)] max-w-[var(--app-width,100svw)] translate-x-0 translate-y-0 rounded-none border-0',
            '[&>button.absolute]:hidden',
          )}
        >
          <header className="flex shrink-0 items-center border-b border-[var(--meet-border)] pt-[env(safe-area-inset-top,0px)]">
            <div className="flex h-12 w-full items-center px-1">
              <DialogTitle className="flex-1 px-3 text-[17px] font-semibold text-[var(--meet-fg-strong)]">
                Audio
              </DialogTitle>
              <button
                type="button"
                onClick={() => setAudioOpen(false)}
                className="flex h-11 w-11 shrink-0 items-center justify-center border-none bg-transparent text-[var(--meet-fg-muted)]"
                aria-label="Close"
              >
                <X size={20} />
              </button>
            </div>
          </header>

          <div className="meet-scroll min-h-0 flex-1 space-y-4 overflow-y-auto p-3 pb-[max(1rem,calc(16px+env(safe-area-inset-bottom,0px)))]">
            {mics.devices.length > 0 && (
              <section>
                <h3 className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--meet-fg-muted)]">
                  Microphone
                </h3>
                <ul className="m-0 list-none overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-0">
                  {mics.devices.map((d, i) => {
                    const active = mics.activeId === d.deviceId
                    return (
                      <li key={d.deviceId} className={cn(i > 0 && 'border-t border-[var(--meet-border)]')}>
                        <button
                          type="button"
                          onClick={() => {
                            void mics.select(d.deviceId)
                          }}
                          className="flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3.5 text-start text-[var(--meet-fg-strong)] transition-colors active:bg-[var(--meet-control)]"
                        >
                          <Check
                            size={16}
                            className={cn('shrink-0 text-teal-400', active ? 'opacity-100' : 'opacity-0')}
                          />
                          <span className="min-w-0 flex-1 truncate text-[15px] font-medium">
                            {d.label || `Microphone ${i + 1}`}
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              </section>
            )}

            {speakers.devices.length > 0 && (
              <section>
                <h3 className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--meet-fg-muted)]">
                  Speaker
                </h3>
                <ul className="m-0 list-none overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-0">
                  {speakers.devices.map((d, i) => {
                    const active = speakers.activeId === d.deviceId
                    return (
                      <li key={d.deviceId} className={cn(i > 0 && 'border-t border-[var(--meet-border)]')}>
                        <button
                          type="button"
                          onClick={() => {
                            void speakers.select(d.deviceId)
                          }}
                          className="flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3.5 text-start text-[var(--meet-fg-strong)] transition-colors active:bg-[var(--meet-control)]"
                        >
                          <Check
                            size={16}
                            className={cn('shrink-0 text-teal-400', active ? 'opacity-100' : 'opacity-0')}
                          />
                          <span className="min-w-0 flex-1 truncate text-[15px] font-medium">
                            {d.label || `Speaker ${i + 1}`}
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              </section>
            )}

            <section>
              <h3 className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--meet-fg-muted)]">
                Noise Suppression
              </h3>
              <ul className="m-0 list-none overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-0">
                {noiseModes.map(({ value, label }, i) => {
                  const disabled = value === 'krisp' && !AudioProcessorService.isKrispSupported()
                  const active = noiseMode === value
                  return (
                    <li key={value} className={cn(i > 0 && 'border-t border-[var(--meet-border)]')}>
                      <button
                        type="button"
                        disabled={disabled}
                        onClick={() => {
                          if (disabled) return
                          requestMode(value)
                        }}
                        className={cn(
                          'flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3.5 text-start transition-colors',
                          disabled
                            ? 'cursor-not-allowed opacity-45'
                            : 'text-[var(--meet-fg-strong)] active:bg-[var(--meet-control)]',
                        )}
                      >
                        <Check
                          size={16}
                          className={cn('shrink-0 text-teal-400', active ? 'opacity-100' : 'opacity-0')}
                        />
                        <span className="min-w-0 flex-1 text-[15px] font-medium">{label}</span>
                        {disabled && (
                          <span className="rounded-sm bg-red-500/15 px-1.5 text-[10px] font-semibold text-red-400">
                            N/A
                          </span>
                        )}
                      </button>
                    </li>
                  )
                })}
              </ul>
            </section>
          </div>
        </DialogContent>
      </Dialog>

      {/* Mobile-only full-screen more sheet (settings-app style + sub-pages). */}
      <Dialog
        open={moreOpen && isMobile}
        onOpenChange={(open) => {
          if (!open) setMoreOpen(false)
        }}
      >
        <DialogContent
          className={cn(
            'meet-dialog flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
            // Visual viewport full-screen (iOS Safari toolbar-safe)
            'fixed left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] max-h-[var(--app-height,100svh)] w-[var(--app-width,100svw)] max-w-[var(--app-width,100svw)] translate-x-0 translate-y-0 rounded-none border-0',
            '[&>button.absolute]:hidden',
          )}
        >
          <header className="flex shrink-0 items-center border-b border-[var(--meet-border)] pt-[env(safe-area-inset-top,0px)]">
            <div className="flex h-12 w-full items-center px-1">
              {morePage ? (
                <button
                  type="button"
                  onClick={() => {
                    setMoreNavDir('back')
                    setMorePage(null)
                  }}
                  className="flex h-11 min-w-0 flex-1 items-center gap-0.5 border-none bg-transparent px-1 text-[var(--meet-accent)]"
                  aria-label="Back to more"
                >
                  <ChevronLeft size={22} className="shrink-0" />
                  <span className="truncate text-[15px]">More</span>
                </button>
              ) : (
                <DialogTitle className="flex-1 px-3 text-[17px] font-semibold text-[var(--meet-fg-strong)]">
                  More
                </DialogTitle>
              )}
              <button
                type="button"
                onClick={() => setMoreOpen(false)}
                className="flex h-11 w-11 shrink-0 items-center justify-center border-none bg-transparent text-[var(--meet-fg-muted)]"
                aria-label="Close"
              >
                <X size={20} />
              </button>
            </div>
          </header>

          {morePage === 'info' && (
            <div
              key="more-info-title"
              className={cn('shrink-0 border-b border-[var(--meet-border)] px-4 py-2', morePageAnim)}
            >
              <h2 className="text-[15px] font-semibold text-[var(--meet-fg-strong)]">Room info</h2>
            </div>
          )}

          <div className="relative min-h-0 flex-1 overflow-hidden pb-[max(0.75rem,env(safe-area-inset-bottom,0px))]">
            <div
              key={morePage ?? 'root'}
              className={cn('absolute inset-0 flex flex-col overflow-hidden', morePageAnim)}
            >
              {morePage === null ? (
                <nav className="meet-scroll min-h-0 flex-1 overflow-y-auto p-3" aria-label="More options">
                  <ul className="m-0 list-none overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-0">
                    {moreRows.map((row, index) => {
                      if (row.kind === 'separator') {
                        return (
                          <li
                            key={row.id}
                            className="h-2 border-t border-[var(--meet-border)] bg-[var(--meet-bg-panel)]"
                            aria-hidden
                          />
                        )
                      }
                      return (
                        <li
                          key={row.id}
                          className={cn(
                            index > 0 &&
                              moreRows[index - 1]?.kind === 'action' &&
                              'border-t border-[var(--meet-border)]',
                          )}
                        >
                          <button
                            type="button"
                            disabled={row.disabled}
                            onClick={() => runMoreAction(row)}
                            className={cn(
                              'flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3.5 text-start transition-colors',
                              row.disabled
                                ? 'cursor-not-allowed opacity-45'
                                : 'active:bg-[var(--meet-control)] text-[var(--meet-fg-strong)]',
                            )}
                          >
                            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)] [&>svg]:h-4 [&>svg]:w-4">
                              {row.icon}
                            </span>
                            <span className="min-w-0 flex-1 text-[15px] font-medium">{row.label}</span>
                            {row.id === 'info' && (
                              <ChevronRight size={18} className="shrink-0 text-[var(--meet-fg-subtle)]" />
                            )}
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                </nav>
              ) : morePage === 'info' && moreExtras?.roomId ? (
                <div className="meet-scroll flex min-h-0 flex-1 flex-col overflow-y-auto">
                  <RoomInfoContent roomId={moreExtras.roomId} active={moreOpen && morePage === 'info'} />
                </div>
              ) : null}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <BedrudSettingsDialog
        open={settingsOpen}
        onOpenChange={(open) => {
          setSettingsOpen(open)
          if (!open) {
            setSettingsElevated(false)
            publishMeetingChromeState(null)
          }
        }}
        elevated={settingsElevated}
      />

      {webxdcEnabled ? (
        <WebxdcAppsDialog
          open={webxdcAppsOpen}
          onOpenChange={setWebxdcAppsOpen}
          roomId={roomId}
          selfName={selfName}
          userId={localParticipant.identity}
        />
      ) : null}
    </TooltipProvider>
  )
}
