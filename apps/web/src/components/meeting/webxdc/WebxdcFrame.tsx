import { useLocalParticipant } from '@livekit/components-react'
import {
  Info,
  LogOut,
  Maximize2,
  MessageSquare,
  Mic,
  MicOff,
  Minimize2,
  RefreshCw,
  Settings,
  Video,
  VideoOff,
  X,
} from 'lucide-react'
import { type CSSProperties, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { RailDeafenIcon } from '@/components/meeting/DeafenHeadphonesIcon'
import { DeviceSelector } from '@/components/meeting/DeviceSelector'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import {
  MEETING_CHROME_STATE,
  type MeetingChromePanel,
  type MeetingChromeStateDetail,
  requestCloseElevatedChrome,
  requestOpenMeetingChat,
  requestOpenMeetingRoomInfo,
  requestOpenMeetingSettings,
} from '@/components/meeting/meetingChromeEvents'
import { useMeetingMicKeyboard } from '@/components/meeting/useMeetingMicKeyboard'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useWebxdcHost } from './useWebxdcHost'

type Props = {
  roomId: string
  instanceId: string
  iframeUrl: string
  iframeOrigin: string
  appName: string
  selfName: string
  selfAddr?: string
  selfAvatarUrl?: string
  userId: string
  onClose?: () => void
}

type Box = { top: number; left: number; width: number; height: number }

/**
 * WebXDC chrome:
 * - Collapsed: title + expand/close top-right only
 * - Expanded: left rail only (logo, chat, settings, info, mic, deafen, video, leave)
 * - Chat/settings/info open without collapsing expand (panels dock above/left)
 */
export function WebxdcFrame({
  roomId,
  instanceId,
  iframeUrl,
  iframeOrigin,
  appName,
  selfName,
  selfAddr,
  selfAvatarUrl,
  userId,
  onClose,
}: Props) {
  const hostRef = useRef<HTMLIFrameElement>(null)
  const placeholderRef = useRef<HTMLDivElement>(null)
  const [documentTitle, setDocumentTitle] = useState<string | null>(null)
  const [summary, setSummary] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [box, setBox] = useState<Box | null>(null)
  const [portalReady, setPortalReady] = useState(false)
  /** Which elevated left-rail panel is open (for icon active glow). */
  const [activePanel, setActivePanel] = useState<MeetingChromePanel>(null)
  /** Bump to remount the cross-origin iframe (reload mini-app). */
  const [iframeKey, setIframeKey] = useState(0)

  // Force sandbox tokens on the DOM — some browsers keep a stale sandbox if only
  // the React prop changes after first mount (pointer-lock must be present).
  useLayoutEffect(() => {
    const el = hostRef.current
    if (!el) return
    el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-pointer-lock')
    el.setAttribute('allow', 'pointer-lock *; fullscreen *; autoplay *; gamepad *')
    el.setAttribute('allowfullscreen', '')
  }, [iframeUrl, iframeKey])

  const { localParticipant, isMicrophoneEnabled: micEnabled, isCameraEnabled: camEnabled } = useLocalParticipant()
  const { isSelfDeafened, toggleSelfDeafen } = useMeetingRoomContext()
  const { micUiEnabled, micTip, toggleMic } = useMeetingMicKeyboard(localParticipant, isSelfDeafened, micEnabled)
  const micOff = isSelfDeafened || !micUiEnabled

  useWebxdcHost(hostRef, {
    roomId,
    instanceId,
    iframeOrigin,
    selfAddr,
    selfName,
    selfAvatarUrl,
    userId,
    onChrome: (meta) => {
      if (meta.document !== undefined) setDocumentTitle(meta.document || null)
      if (meta.summary !== undefined) setSummary(meta.summary || null)
    },
  })

  useEffect(() => {
    setPortalReady(typeof document !== 'undefined')
  }, [])

  useLayoutEffect(() => {
    if (expanded) return
    const el = placeholderRef.current
    if (!el) return

    const update = () => {
      const r = el.getBoundingClientRect()
      setBox({ top: r.top, left: r.left, width: r.width, height: r.height })
    }
    update()
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(update) : null
    ro?.observe(el)
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      ro?.disconnect()
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [expanded])

  useEffect(() => {
    if (!expanded) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      // Elevated chat/settings/info own Escape (also stopPropagation in focus trap). Keep expand open.
      const t = e.target
      if (
        t instanceof Element &&
        (t.closest('[data-elevated-chat="true"]') ||
          t.closest('[data-elevated-settings="true"]') ||
          t.closest('[data-elevated-room-info="true"]'))
      ) {
        return
      }
      e.preventDefault()
      setExpanded(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [expanded])

  useEffect(() => {
    if (!expanded) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prev
    }
  }, [expanded])

  // Sync left-rail active glow with elevated chat/settings/info.
  useEffect(() => {
    if (!expanded) {
      setActivePanel(null)
      return
    }
    const onState = (e: Event) => {
      const detail = (e as CustomEvent<MeetingChromeStateDetail>).detail
      setActivePanel(detail?.panel ?? null)
    }
    window.addEventListener(MEETING_CHROME_STATE, onState)
    return () => window.removeEventListener(MEETING_CHROME_STATE, onState)
  }, [expanded])

  // Collapse expand → close elevated left docks (chat / settings / info).
  const wasExpandedRef = useRef(false)
  useEffect(() => {
    if (expanded) {
      wasExpandedRef.current = true
      return
    }
    if (wasExpandedRef.current) {
      wasExpandedRef.current = false
      requestCloseElevatedChrome()
      setActivePanel(null)
    }
  }, [expanded])

  const titleBits = [appName, documentTitle, summary].filter(Boolean).join(' · ')
  const iconBtn = 'h-9 w-9 shrink-0'
  const railActiveClass =
    'bg-primary/15 text-primary shadow-[0_0_0_1px_color-mix(in_oklab,var(--primary)_45%,transparent),0_0_12px_color-mix(in_oklab,var(--primary)_35%,transparent)] hover:bg-primary/20 hover:text-primary'
  const wxDetail = { source: 'webxdc-expand' as const }

  const leaveApp = () => {
    setExpanded(false)
    onClose?.()
  }

  // Expanded-only: everything on the left (vertical). No bottom bar.
  // Top: chrome panels. Bottom (always visible): mic · deafen · video · leave.
  const leftRail = expanded ? (
    <aside
      className="flex w-12 min-h-0 shrink-0 flex-col items-center border-r border-border bg-background px-1 py-2"
      aria-label="Meeting actions"
    >
      <div className="flex w-full shrink-0 flex-col items-center gap-1.5">
        <div
          className="mb-1 flex h-9 w-9 items-center justify-center overflow-hidden rounded-md border border-border bg-muted/40"
          title="Bedrud"
          aria-hidden
        >
          <img src="/favicon.svg" alt="" className="h-5 w-5 object-contain" />
        </div>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(iconBtn, activePanel === 'chat' && railActiveClass)}
          aria-label={activePanel === 'chat' ? 'Close chat' : 'Open chat'}
          aria-pressed={activePanel === 'chat'}
          onClick={() => requestOpenMeetingChat(wxDetail)}
        >
          <MessageSquare className="h-4 w-4" />
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(iconBtn, activePanel === 'settings' && railActiveClass)}
          aria-label={activePanel === 'settings' ? 'Close settings' : 'Open settings'}
          aria-pressed={activePanel === 'settings'}
          onClick={() => requestOpenMeetingSettings(wxDetail)}
        >
          <Settings className="h-4 w-4" />
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(iconBtn, activePanel === 'info' && railActiveClass)}
          aria-label={activePanel === 'info' ? 'Close room info' : 'Room info'}
          aria-pressed={activePanel === 'info'}
          onClick={() => requestOpenMeetingRoomInfo(wxDetail)}
        >
          <Info className="h-4 w-4" />
        </Button>
      </div>

      <div className="min-h-2 flex-1" aria-hidden />

      <div
        className="flex w-full shrink-0 flex-col items-center gap-1.5 border-t border-border pt-2"
        aria-label="Audio and video"
      >
        {/* One voice-input menu: above mic, right-aligned (matches bottom-bar chevron role). */}
        <div className="flex w-full justify-end pe-0.5">
          <DeviceSelector
            kind="audioinput"
            menuSide="right"
            elevated
            triggerClassName="h-5 w-6 text-muted-foreground hover:text-foreground"
          />
        </div>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            iconBtn,
            micOff && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
          )}
          aria-label={micOff ? (isSelfDeafened ? 'Undeafen' : 'Unmute microphone') : micTip || 'Mute microphone'}
          aria-pressed={micOff}
          title={micOff ? (isSelfDeafened ? 'Undeafen' : 'Unmute') : 'Mute'}
          onClick={() => {
            if (isSelfDeafened) {
              toggleSelfDeafen()
              return
            }
            toggleMic()
          }}
        >
          {micOff ? <MicOff className="h-4 w-4" /> : <Mic className="h-4 w-4" />}
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            iconBtn,
            isSelfDeafened && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
          )}
          aria-label={isSelfDeafened ? 'Undeafen' : 'Deafen'}
          aria-pressed={isSelfDeafened}
          title={isSelfDeafened ? 'Undeafen' : 'Deafen'}
          onClick={() => toggleSelfDeafen()}
        >
          <RailDeafenIcon off={isSelfDeafened} />
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            iconBtn,
            !camEnabled && 'bg-destructive/15 text-destructive hover:bg-destructive/20 hover:text-destructive',
          )}
          aria-label={camEnabled ? 'Disable camera' : 'Enable camera'}
          aria-pressed={!camEnabled}
          title={camEnabled ? 'Camera off' : 'Camera on'}
          onClick={() => {
            void localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})
          }}
        >
          {camEnabled ? <Video className="h-4 w-4" /> : <VideoOff className="h-4 w-4" />}
        </Button>

        {onClose ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(iconBtn, 'text-destructive hover:bg-destructive/10 hover:text-destructive')}
            aria-label="Leave mini-app"
            title="Leave mini-app"
            onClick={leaveApp}
          >
            <LogOut className="h-4 w-4" />
          </Button>
        ) : null}
      </div>
    </aside>
  ) : null

  const titleBar = (
    <div className="flex min-w-0 items-center gap-2 border-b border-border bg-background px-2 py-1.5 sm:px-3 sm:py-2">
      {selfAvatarUrl ? (
        <img
          src={selfAvatarUrl}
          alt=""
          className="h-7 w-7 shrink-0 rounded-full object-cover"
          referrerPolicy="no-referrer"
        />
      ) : (
        <div
          className="bg-muted text-muted-foreground flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold"
          aria-hidden
        >
          {(selfName || '?').slice(0, 2).toUpperCase()}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{titleBits || appName}</div>
        <div className="text-muted-foreground truncate text-xs">
          You: {selfName || 'Guest'}
          <span className="mx-1 opacity-40">·</span>
          Untrusted mini-app · experimental
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-0.5">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          aria-label="Reload mini-app"
          title="Reload mini-app"
          onClick={() => {
            // Cross-origin iframe cannot use contentWindow.location.reload(); remount instead.
            setIframeKey((k) => k + 1)
            setDocumentTitle(null)
            setSummary(null)
          }}
        >
          <RefreshCw className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          aria-label={expanded ? 'Exit fullscreen' : 'Expand to fullscreen'}
          aria-pressed={expanded}
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
        </Button>
        {onClose ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            aria-label="Close mini-app"
            onClick={leaveApp}
          >
            <X className="h-4 w-4" />
          </Button>
        ) : null}
      </div>
    </div>
  )

  const shellStyle: CSSProperties = expanded
    ? {
        position: 'fixed',
        top: 'var(--app-offset-top, 0px)',
        left: 'var(--app-offset-left, 0px)',
        width: 'var(--app-width, 100vw)',
        height: 'var(--app-height, 100dvh)',
        zIndex: 200,
      }
    : box
      ? {
          position: 'fixed',
          top: box.top,
          left: box.left,
          width: box.width,
          height: box.height,
          zIndex: 15,
        }
      : {
          position: 'fixed',
          visibility: 'hidden',
          pointerEvents: 'none',
          zIndex: 15,
        }

  const surface =
    portalReady &&
    createPortal(
      <div
        role={expanded ? 'dialog' : undefined}
        aria-modal={expanded || undefined}
        aria-label={expanded ? `${appName} fullscreen` : undefined}
        className={cn('flex overflow-hidden border border-border bg-background shadow-2xl', expanded && 'border-0')}
        style={shellStyle}
      >
        {leftRail}

        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          {titleBar}
          <iframe
            key={iframeKey}
            ref={hostRef}
            title={`${appName} (experimental untrusted mini-app)`}
            src={iframeUrl}
            className="min-h-0 w-full flex-1 bg-white"
            // allow-pointer-lock is required for sandboxed iframes — without it
            // requestPointerLock always fails (OpenArena mouse look).
            // Still no allow-top-navigation / popups / forms (sandbox isolation).
            sandbox="allow-scripts allow-same-origin allow-pointer-lock"
            // Feature Policy for the cross-origin webxdc document (Desktop: pointer + fullscreen).
            allow="pointer-lock *; fullscreen *; autoplay *; gamepad *"
            allowFullScreen
            referrerPolicy="no-referrer"
          />
        </div>
      </div>,
      document.body,
    )

  return (
    <>
      <div ref={placeholderRef} className="h-full min-h-[12rem] w-full min-w-0" aria-hidden />
      {surface}
    </>
  )
}
