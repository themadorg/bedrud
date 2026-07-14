import { type CSSProperties, type RefObject, useEffect, useLayoutEffect, useRef, useState } from 'react'
import {
  MEETING_CHROME_STATE,
  type MeetingChromeOpenDetail,
  type MeetingChromePanel,
  type MeetingChromeStateDetail,
  requestCloseElevatedChrome,
} from './meetingChromeEvents'

type Box = { top: number; left: number; width: number; height: number }

const EXPANDED_Z = 200

type Options = {
  /**
   * `portal` (default): collapsed view is body-portaled over a layout placeholder (WebXDC).
   * `inline`: collapsed view stays in the stage tree; only expanded mode portals.
   */
  collapseMode?: 'portal' | 'inline'
}

export function useMeetingExpandChrome(
  placeholderRef: RefObject<HTMLElement | null> | null,
  source: MeetingChromeOpenDetail['source'],
  options?: Options,
) {
  const collapseMode = options?.collapseMode ?? 'portal'
  const inlineCollapsed = collapseMode === 'inline'
  const [expanded, setExpanded] = useState(false)
  const [box, setBox] = useState<Box | null>(null)
  const [portalReady, setPortalReady] = useState(false)
  const [activePanel, setActivePanel] = useState<MeetingChromePanel>(null)
  const wasExpandedRef = useRef(false)

  const chromeDetail: MeetingChromeOpenDetail = { source }

  useEffect(() => {
    setPortalReady(typeof document !== 'undefined')
  }, [])

  useLayoutEffect(() => {
    if (expanded || inlineCollapsed) return
    const el = placeholderRef?.current
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
  }, [expanded, inlineCollapsed, placeholderRef])

  useEffect(() => {
    if (!expanded) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
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

  const expandedShellStyle: CSSProperties = {
    position: 'fixed',
    top: 'var(--app-offset-top, 0px)',
    left: 'var(--app-offset-left, 0px)',
    width: 'var(--app-width, 100vw)',
    height: 'var(--app-height, 100dvh)',
    zIndex: EXPANDED_Z,
  }

  const shellStyle: CSSProperties | null = expanded
    ? expandedShellStyle
    : inlineCollapsed
      ? null
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

  const shouldPortal = portalReady && (expanded || !inlineCollapsed)

  return {
    expanded,
    setExpanded,
    toggleExpanded: () => setExpanded((v) => !v),
    collapse: () => setExpanded(false),
    portalReady,
    shouldPortal,
    shellStyle,
    activePanel,
    chromeDetail,
    expandedZ: EXPANDED_Z,
  }
}
