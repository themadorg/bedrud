// TODO oncoming feature
import { useLocalParticipant, useRoomContext } from '@livekit/components-react'

import { ConnectionState, RoomEvent } from 'livekit-client'
import {
  Check,
  ChevronDown,
  Mic,
  MicOff,
  MonitorOff,
  MonitorUp,
  MoreVertical,
  Package,
  PhoneOff,
  Video,
  VideoOff,
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
import { SCREEN_SHARE_CAPTURE_OPTIONS } from '#/lib/screen-share-capture'
import { useIsMobile } from '#/lib/use-is-mobile'
import { getPublicSettings, refreshPublicSettings } from '#/lib/use-public-settings'
import { useRequestNoiseMode } from '#/lib/use-request-noise-mode'
import { cn } from '#/lib/utils'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingChatContext, useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { MeetingControlsPill } from '@/components/meeting/MeetingControlsPill'
import { meetingOptionIcon } from '@/components/meeting/MeetingOptionsPanel'
import { meetControlsDockClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import {
  isExpandChromeSource,
  MEETING_CLOSE_ELEVATED_CHROME,
  MEETING_CLOSE_SETTINGS,
  MEETING_OPEN_ROOM_INFO,
  MEETING_OPEN_SETTINGS,
  publishMeetingChromeState,
} from '@/components/meeting/meetingChromeEvents'
import { type MeetingOptionRowId, meetingOptionRows } from '@/components/meeting/meetingOptionRows'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import { stageOwnerLabel } from '@/components/meeting/stage/stageWire'
import { waitForScreenSharePublication } from '@/components/meeting/stage/waitForScreenShare'
import { WebxdcAppsDialog } from '@/components/meeting/webxdc/WebxdcAppsDialog'
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

/** Room-chrome actions merged into the options list both surfaces draw. */
export interface ControlsBarMoreExtras {
  onRoomAccess?: () => void
  isPublic?: boolean
  onRoomInfo?: () => void
  /** Required for the Room info row. */
  roomId?: string
  onToggleVideoSidebar?: () => void
  showVideoSidebarToggle?: boolean
  videoSidebarOpen?: boolean
}

interface Props {
  onLeave: () => void
  /** Room-chrome actions, shown as rows in the phone panel and the desktop ⋯ menu. */
  moreExtras?: ControlsBarMoreExtras
  /** Chat is a control in the phone pill, so the bar owns its state on that width. */
  chatOpen: boolean
  onToggleChat: () => void
}

/* ── CtrlBtn: tooltip-wrapped control button ─────────────────────────────── */

function btnIconCn(active = false, danger = false, ptt = false) {
  return cn(
    'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color,box-shadow,border-color] duration-150',
    'h-11 w-11 rounded-xl',
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
          className={cn(btnIconCn(active, danger, ptt), className)}
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

const dividerCn = 'w-px h-7 bg-[var(--meet-border)] mx-0.5 shrink-0 max-lg:hidden'

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

export function ControlsBar({ onLeave, moreExtras, chatOpen, onToggleChat }: Props) {
  const isMobile = useIsMobile()
  const { unreadCount } = useMeetingChatContext()
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
    getParticipantDisplayName(localParticipant) || localParticipant.name || localParticipant.identity || 'You'

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

  // The desktop bar's two icon sizes. The phone pill sizes its own.
  const iconSize = 18
  const iconSizeSm = 17

  const { pttVisible, pttAvailable, micUiEnabled, micTip, pttTip, pushToTalkEnabled, toggleMic, startPtt, stopPtt } =
    useMeetingMicKeyboard(localParticipant, isSelfDeafened, micEnabled)

  // ── Copy link feedback ──
  const [linkCopied, setLinkCopied] = useState(false)
  const linkCopiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settingsElevated, setSettingsElevated] = useState(false)

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
      const fromWx = isExpandChromeSource((e as CustomEvent).detail)
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

  const optionRows = useMemo(
    () =>
      meetingOptionRows({
        videoSidebarAvailable: Boolean(moreExtras?.showVideoSidebarToggle && moreExtras.onToggleVideoSidebar),
        videoSidebarOpen: Boolean(moreExtras?.videoSidebarOpen),
        roomAccessAvailable: Boolean(moreExtras?.onRoomAccess),
        isPublic: Boolean(moreExtras?.isPublic),
        roomId: moreExtras?.roomId,
        linkCopied,
        isSelfDeafened,
        microphones: mics.devices.map((device, index) => ({
          deviceId: device.deviceId,
          label: device.label || `Microphone ${index + 1}`,
        })),
        activeMicrophoneId: mics.activeId,
        speakers: speakers.devices.map((device, index) => ({
          deviceId: device.deviceId,
          label: device.label || `Speaker ${index + 1}`,
        })),
        activeSpeakerId: speakers.activeId,
        noiseModes: noiseModes.map(({ value, label }) => ({
          value,
          label,
          disabled: value === 'krisp' && !AudioProcessorService.isKrispSupported(),
        })),
        activeNoiseMode: noiseMode,
        fullscreenAvailable: typeof document !== 'undefined' && document.fullscreenEnabled,
        isFullscreen: typeof document !== 'undefined' && Boolean(document.fullscreenElement),
        whiteboardEnabled,
        isWhiteboardOnStage: stage?.kind === 'whiteboard',
        isWhiteboardHost,
        youtubeEnabled,
        isYoutubeOnStage: stage?.kind === 'youtube',
        isYoutubeHost,
        webxdcEnabled,
        isWebxdcOnStage,
        stageTakenByOther,
      }),
    [
      moreExtras,
      linkCopied,
      isSelfDeafened,
      mics,
      speakers,
      noiseModes,
      noiseMode,
      whiteboardEnabled,
      youtubeEnabled,
      webxdcEnabled,
      isWebxdcOnStage,
      stage,
      isWhiteboardHost,
      isYoutubeHost,
      stageTakenByOther,
    ],
  )

  /** Opens the room info panel the shell owns, in place of the deleted in-dialog sub-page. */
  const openRoomInfo = useCallback(() => {
    window.dispatchEvent(new CustomEvent(MEETING_OPEN_ROOM_INFO))
  }, [])

  /** Runs the effect behind a row id, for both the phone panel and the desktop dropdown. */
  const runOptionRow = useCallback(
    (id: MeetingOptionRowId) => {
      if (id.startsWith('microphone:')) {
        void mics.select(id.slice('microphone:'.length))
        return
      }
      if (id.startsWith('speaker:')) {
        void speakers.select(id.slice('speaker:'.length))
        return
      }
      if (id.startsWith('noise:')) {
        requestMode(id.slice('noise:'.length) as NoiseSuppressionMode)
        return
      }
      switch (id) {
        case 'videos':
          moreExtras?.onToggleVideoSidebar?.()
          return
        case 'access':
          moreExtras?.onRoomAccess?.()
          return
        case 'info':
          openRoomInfo()
          return
        case 'copy-link':
          copyRoomLink()
          return
        case 'deafen':
          toggleSelfDeafen()
          return
        case 'settings':
          setSettingsOpen(true)
          return
        case 'fullscreen':
          toggleFullscreen()
          return
        case 'whiteboard': {
          if (stage?.kind === 'whiteboard' && isWhiteboardHost) {
            clearStage()
            return
          }
          const error = requestStartWhiteboard()
          if (error) toast.error(error)
          return
        }
        case 'youtube':
          if (stage?.kind === 'youtube' && isYoutubeHost) stopYoutubeShare()
          else openShareDialog()
          return
        case 'app-gallery':
          setWebxdcAppsOpen(true)
          return
        default:
          // Headings are rendered as labels and never reach here.
          return
      }
    },
    [
      mics,
      speakers,
      requestMode,
      moreExtras,
      openRoomInfo,
      copyRoomLink,
      toggleSelfDeafen,
      toggleFullscreen,
      stage,
      isWhiteboardHost,
      isYoutubeHost,
      clearStage,
      requestStartWhiteboard,
      stopYoutubeShare,
      openShareDialog,
    ],
  )

  /** Claims the stage and starts sharing, or stops and releases it. Both bars call this one. */
  const toggleScreenShare = useCallback(async () => {
    if (isScreenShareEnabled) {
      await localParticipant?.setScreenShareEnabled(false).catch(() => {})
      if (isOwner && stage?.kind === 'screenshare') clearStage()
      return
    }
    try {
      const claimError = claimStage('screenshare')
      if (claimError) {
        toast.error(claimError)
        return
      }
      await localParticipant?.setScreenShareEnabled(true, SCREEN_SHARE_CAPTURE_OPTIONS)
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
  }, [isScreenShareEnabled, localParticipant, isOwner, stage, clearStage, claimStage])

  return (
    <TooltipProvider delayDuration={300}>
      {/*
       * The phone draws the Android pill; the desktop keeps the docked bar. Only this element
       * branches — the dialogs below it are reached from both surfaces and must stay mounted.
       */}
      {isMobile ? (
        <MeetingControlsPill
          rows={optionRows}
          onSelectOption={runOptionRow}
          cameraEnabled={camEnabled}
          onToggleCamera={() => localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})}
          screenShareEnabled={isScreenShareEnabled}
          screenShareAvailable={canShare && !stageTakenByOther}
          onToggleScreenShare={() => void toggleScreenShare()}
          micPushToTalk={pushToTalkEnabled}
          micOpen={micUiEnabled && !isSelfDeafened}
          micTransmitting={pttVisible}
          micAvailable={pushToTalkEnabled ? pttAvailable : true}
          onToggleMic={() => {
            if (isSelfDeafened) {
              toggleSelfDeafen()
              return
            }
            toggleMic()
          }}
          onPushToTalkChange={(held) => (held ? startPtt() : stopPtt())}
          chatOpen={chatOpen}
          unreadCount={unreadCount}
          onToggleChat={onToggleChat}
          onLeave={onLeave}
        />
      ) : (
        <div
          id="meet-controls"
          className={cn(
            // meet-controls-bar: border-radius needs !important (global * { border-radius: 0 })
            'meet-controls-bar absolute -translate-x-1/2 z-30 flex items-center bg-[var(--meet-chrome)] backdrop-blur-xl border border-[var(--meet-border-subtle)] whitespace-nowrap shadow-[var(--meet-shadow),var(--meet-shadow-inset)] transition-[left] duration-200',
            meetControlsDockClass(layout),
            'bottom-5 gap-[3px] p-2',
            'max-w-[calc(var(--app-width,100svw)-16px)]',
          )}
        >
          {/* ── Left: Video + Screen Share ── */}
          <div className="flex items-center gap-px">
            <CtrlBtn
              tip={camEnabled ? 'Disable camera' : 'Enable camera'}
              active={!camEnabled}
              onClick={() => localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})}
            >
              {camEnabled ? <Video size={iconSize} /> : <VideoOff size={iconSize} />}
            </CtrlBtn>
            <DeviceSelector kind="videoinput" />
          </div>

          <CtrlBtn
            tip={shareTip}
            danger={isScreenShareEnabled}
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
                      await localParticipant?.setScreenShareEnabled(true, SCREEN_SHARE_CAPTURE_OPTIONS)
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
              onClick={() => setWebxdcAppsOpen(true)}
            >
              <Package size={iconSize} />
            </CtrlBtn>
          ) : null}

          {/* TODO oncoming feature — recording button removed */}

          <div className={dividerCn} />

          {/* ── Center: Leave ── */}
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={onLeave}
                className={cn(
                  // meet-btn-leave: border-radius needs !important (global * { border-radius: 0 })
                  'meet-btn-leave flex items-center gap-2 shrink-0 border-none cursor-pointer text-[var(--meet-btn-leave-fg)] text-[13px] font-semibold transition-[background,box-shadow] duration-150',
                  'h-11 px-[18px] mx-0.5',
                  'bg-[var(--meet-btn-leave-bg)] shadow-[0_2px_12px_color-mix(in_oklab,var(--meet-btn-leave-bg)_45%,transparent)] hover:bg-[var(--meet-btn-leave-hover)]',
                )}
                aria-label="Leave meeting"
              >
                <PhoneOff size={16} />
                Leave
              </button>
            </TooltipTrigger>
            <TooltipContent side="top" sideOffset={8}>
              Leave meeting
            </TooltipContent>
          </Tooltip>

          <div className={dividerCn} />

          {/* ── Right: Mic + Speaker/Deafen + Combined Audio Dropdown ── */}
          <div className="flex items-center gap-px">
            {pushToTalkEnabled && (
              <CtrlBtn
                tip={pttTip}
                active={pttVisible}
                ptt={pttAvailable && !pttVisible}
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

            <CtrlBtn tip={isSelfDeafened ? 'Undeafen' : 'Deafen'} danger={isSelfDeafened} onClick={toggleSelfDeafen}>
              <DeafenHeadphonesIcon size={iconSizeSm} off={isSelfDeafened} />
            </CtrlBtn>

            {/* Audio devices + noise. The phone reaches these as rows in the controls pill's panel. */}
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
          </div>

          <div className={dividerCn} />

          {/* ── Far right: More options ── */}
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
              {optionRows.map((row) =>
                row.kind === 'heading' ? (
                  <DropdownMenuLabel key={row.id} className={meetMenuLabelCn}>
                    {row.label}
                  </DropdownMenuLabel>
                ) : (
                  <DropdownMenuItem
                    key={row.id}
                    disabled={row.disabled}
                    onClick={() => runOptionRow(row.id)}
                    className={cn(meetMenuItemCn, row.disabled && 'cursor-not-allowed opacity-50')}
                  >
                    <span className="flex h-4 w-4 shrink-0 items-center justify-center [&>svg]:h-[13px] [&>svg]:w-[13px]">
                      {meetingOptionIcon(row)}
                    </span>
                    {row.label}
                    {row.checked && <Check size={12} className="ml-auto shrink-0 text-teal-400" />}
                  </DropdownMenuItem>
                ),
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}

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
