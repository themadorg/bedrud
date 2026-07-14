import { Maximize2, Minimize2, RefreshCw, X } from 'lucide-react'
import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { MeetingExpandLeftRail } from '@/components/meeting/MeetingExpandLeftRail'
import { useMeetingExpandChrome } from '@/components/meeting/useMeetingExpandChrome'
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
  const [iframeKey, setIframeKey] = useState(0)

  const expand = useMeetingExpandChrome(placeholderRef, 'webxdc-expand')

  // Force sandbox tokens on the DOM — some browsers keep a stale sandbox if only
  // the React prop changes after first mount (pointer-lock must be present).
  useLayoutEffect(() => {
    const el = hostRef.current
    if (!el) return
    el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-pointer-lock')
    el.setAttribute('allow', 'pointer-lock *; fullscreen *; autoplay *; gamepad *')
    el.setAttribute('allowfullscreen', '')
  }, [iframeUrl, iframeKey])

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

  const titleBits = [appName, documentTitle, summary].filter(Boolean).join(' · ')

  const leaveApp = () => {
    expand.collapse()
    onClose?.()
  }

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
          aria-label={expand.expanded ? 'Exit fullscreen' : 'Expand to fullscreen'}
          aria-pressed={expand.expanded}
          onClick={expand.toggleExpanded}
        >
          {expand.expanded ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
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

  const surface =
    expand.shouldPortal &&
    expand.shellStyle &&
    createPortal(
      <div
        {...(expand.expanded
          ? {
              role: 'dialog' as const,
              'aria-modal': true as const,
              'aria-label': `${appName} fullscreen`,
            }
          : {})}
        className={cn(
          'flex overflow-hidden border border-border bg-background shadow-2xl',
          expand.expanded && 'border-0',
        )}
        style={expand.shellStyle}
      >
        {expand.expanded ? (
          <MeetingExpandLeftRail
            activePanel={expand.activePanel}
            chromeDetail={expand.chromeDetail}
            onLeave={onClose ? leaveApp : undefined}
            leaveLabel="Leave mini-app"
          />
        ) : null}

        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          {titleBar}
          <iframe
            key={iframeKey}
            ref={hostRef}
            title={`${appName} (experimental untrusted mini-app)`}
            src={iframeUrl}
            className="min-h-0 w-full flex-1 bg-white"
            sandbox="allow-scripts allow-same-origin allow-pointer-lock"
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
