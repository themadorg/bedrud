import { Copy, Mail, MessageCircle, QrCode, Send, Share2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useMemo, useState } from 'react'
import type { InviteShareTarget, InviteShareTargetId } from './inviteShareTargets'
import { inviteShareTargets } from './inviteShareTargets'
import { copyMeetingLink } from './meetingLink'

/**
 * The QR's own coordinate resolution, which is not a size on screen: the code is stretched to the box
 * `--meet-invite-qr-size` gives it. A power of two keeps the modules on whole pixels at the common
 * display scales.
 */
const QR_RESOLUTION = 256

const TARGET_ICONS: Record<InviteShareTargetId, typeof Share2> = {
  share: Share2,
  copy: Copy,
  qr: QrCode,
  email: Mail,
  telegram: Send,
  whatsapp: MessageCircle,
}

const TARGET_CIRCLE_CLASS =
  'flex items-center justify-center rounded-full bg-[var(--meet-control)] text-[var(--meet-fg-strong)] transition-[background] duration-150 hover:bg-[var(--meet-control-hover)]'

const TARGET_LABEL_CLASS =
  'w-16 overflow-hidden text-ellipsis whitespace-nowrap text-center text-[11px] text-[var(--meet-fg-muted)]'

const TARGET_SIZE_STYLE = {
  width: 'var(--meet-invite-target-size)',
  height: 'var(--meet-invite-target-size)',
}

interface MeetingInviteTargetsProps {
  roomLink: string
  /** Asks the sheet for its full height, which the QR needs to fit without hiding the row below it. */
  onExpandSheet: () => void
}

/** The share row, the QR it toggles above itself, and the room link in full underneath. */
export function MeetingInviteTargets({ roomLink, onExpandSheet }: MeetingInviteTargetsProps) {
  const [canShare, setCanShare] = useState(false)
  const [qrVisible, setQrVisible] = useState(false)

  // `navigator.share` cannot be read during the server render, and reading it in the first client
  // render would make the markup disagree with the server's. It is read once the sheet is mounted.
  useEffect(() => {
    setCanShare(typeof navigator !== 'undefined' && typeof navigator.share === 'function')
  }, [])

  const targets = useMemo(() => inviteShareTargets(roomLink, canShare), [roomLink, canShare])

  const activateTarget = (target: InviteShareTarget) => {
    if (target.id === 'share') {
      // A share the participant dismisses rejects, and a dismissal is not an error worth reporting.
      void navigator.share({ url: roomLink }).catch(() => {})
      return
    }
    if (target.id === 'copy') {
      void copyMeetingLink()
      return
    }
    if (target.id === 'qr') {
      // The next visibility is computed here rather than inside the updater, so the sheet is only
      // expanded when the QR is being opened and the updater stays free of side effects.
      const nextVisible = !qrVisible
      setQrVisible(nextVisible)
      if (nextVisible) onExpandSheet()
    }
  }

  return (
    <div className="flex shrink-0 flex-col gap-3">
      {qrVisible && (
        <div className="flex justify-center">
          {/*
            The plate is white in both themes. A scanner needs the contrast, and a QR drawn in the
            dark theme's colours is a QR that does not scan.
          */}
          <div className="rounded-2xl bg-white p-3" style={{ width: 'var(--meet-invite-qr-size)' }}>
            <QRCodeSVG value={roomLink} size={QR_RESOLUTION} className="h-auto w-full" />
          </div>
        </div>
      )}

      {/*
        No scrollbar under the row. The meeting's scrollbar is 5px of accent colour, which reads as an
        underline beneath four labels rather than as a scrollbar, and Android's target row has none.
        This is the filmstrip's treatment in `FocusLayout`, which is the same shape of surface.
      */}
      <div className="meet-scroll-none flex gap-4 overflow-x-auto pb-1 [scrollbar-width:none]">
        {targets.map((target) => {
          const Icon = TARGET_ICONS[target.id]
          const icon = <Icon size={24} />

          if (target.href !== undefined) {
            return (
              <a
                key={target.id}
                href={target.href}
                target="_blank"
                rel="noreferrer"
                className="flex shrink-0 flex-col items-center gap-1.5"
              >
                <span className={TARGET_CIRCLE_CLASS} style={TARGET_SIZE_STYLE}>
                  {icon}
                </span>
                <span className={TARGET_LABEL_CLASS}>{target.label}</span>
              </a>
            )
          }

          return (
            <button
              key={target.id}
              type="button"
              onClick={() => activateTarget(target)}
              className="flex shrink-0 cursor-pointer flex-col items-center gap-1.5 border-none bg-transparent p-0"
              aria-pressed={target.id === 'qr' ? qrVisible : undefined}
            >
              <span className={TARGET_CIRCLE_CLASS} style={TARGET_SIZE_STYLE}>
                {icon}
              </span>
              <span className={TARGET_LABEL_CLASS}>{target.label}</span>
            </button>
          )
        })}
      </div>

      <button
        type="button"
        onClick={() => void copyMeetingLink()}
        className="flex w-full cursor-pointer items-center gap-2 rounded-xl border border-[var(--meet-border-subtle)] bg-[var(--meet-control)] px-3 py-2.5 text-start"
        aria-label="Copy room link"
      >
        <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-[var(--meet-fg-muted)]">
          {roomLink}
        </span>
        <Copy size={14} className="shrink-0 text-[var(--meet-fg-muted)]" />
      </button>
    </div>
  )
}
