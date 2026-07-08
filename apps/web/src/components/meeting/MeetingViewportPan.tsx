import {
  createContext,
  type ReactNode,
  type RefObject,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import {
  buildViewportTransformCss,
  clampViewportZoom,
  readElementContentInsets,
  type ViewportTransform,
  wheelZoomFactor,
  zoomAtClientPoint,
} from '@/components/meeting/meetingViewportTransform'
import { cn } from '@/lib/utils'

const DEFAULT_TRANSFORM: ViewportTransform = { panX: 0, panY: 0, zoom: 1 }

interface MeetingViewportPanContextValue {
  enabled: boolean
  isPanning: boolean
  transform: ViewportTransform
  transformRef: RefObject<ViewportTransform>
  panTransform: string
  shellProps: {
    onPointerDown: (e: React.PointerEvent<HTMLElement>) => void
    onPointerMove: (e: React.PointerEvent<HTMLElement>) => void
    onPointerUp: (e: React.PointerEvent<HTMLElement>) => void
    onPointerCancel: (e: React.PointerEvent<HTMLElement>) => void
    onContextMenu: (e: React.MouseEvent<HTMLElement>) => void
    onLostPointerCapture: (e: React.PointerEvent<HTMLElement>) => void
    onWheel: (e: React.WheelEvent<HTMLElement>) => void
  }
}

const MeetingViewportPanContext = createContext<MeetingViewportPanContextValue | null>(null)

export function MeetingViewportPanProvider({ children }: { children: ReactNode }) {
  const enabled = false
  const [transform, setTransform] = useState<ViewportTransform>(DEFAULT_TRANSFORM)
  const transformRef = useRef(DEFAULT_TRANSFORM)
  const dragRef = useRef<{
    pointerId: number
    startX: number
    startY: number
    originPanX: number
    originPanY: number
  } | null>(null)
  const [isPanning, setIsPanning] = useState(false)

  const endDrag = useCallback((target: HTMLElement, pointerId: number) => {
    if (dragRef.current?.pointerId === pointerId) {
      dragRef.current = null
      setIsPanning(false)
      try {
        if (target.hasPointerCapture(pointerId)) target.releasePointerCapture(pointerId)
      } catch {
        /* already released */
      }
    }
  }, [])

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLElement>) => {
      if (!enabled || e.button !== 2) return
      e.preventDefault()
      e.currentTarget.setPointerCapture(e.pointerId)
      setIsPanning(true)
      dragRef.current = {
        pointerId: e.pointerId,
        startX: e.clientX,
        startY: e.clientY,
        originPanX: transform.panX,
        originPanY: transform.panY,
      }
    },
    [enabled, transform.panX, transform.panY],
  )

  const onPointerMove = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== e.pointerId) return
    e.preventDefault()
    setTransform((prev) => {
      const next = {
        ...prev,
        panX: drag.originPanX + (e.clientX - drag.startX),
        panY: drag.originPanY + (e.clientY - drag.startY),
      }
      transformRef.current = next
      return next
    })
  }, [])

  const onPointerUp = useCallback(
    (e: React.PointerEvent<HTMLElement>) => {
      endDrag(e.currentTarget, e.pointerId)
    },
    [endDrag],
  )

  const onPointerCancel = useCallback(
    (e: React.PointerEvent<HTMLElement>) => {
      endDrag(e.currentTarget, e.pointerId)
    },
    [endDrag],
  )

  const onLostPointerCapture = useCallback(
    (e: React.PointerEvent<HTMLElement>) => {
      endDrag(e.currentTarget, e.pointerId)
    },
    [endDrag],
  )

  const onContextMenu = useCallback(
    (e: React.MouseEvent<HTMLElement>) => {
      if (enabled) e.preventDefault()
    },
    [enabled],
  )

  const onWheel = useCallback(
    (e: React.WheelEvent<HTMLElement>) => {
      if (!enabled) return
      e.preventDefault()
      const target = e.currentTarget
      const rect = target.getBoundingClientRect()
      const insets = readElementContentInsets(target)
      const nextZoom = clampViewportZoom(transform.zoom * wheelZoomFactor(e.deltaY))
      if (nextZoom === transform.zoom) return
      const next = zoomAtClientPoint(transform, rect, e.clientX, e.clientY, nextZoom, insets)
      transformRef.current = next
      setTransform(next)
    },
    [enabled, transform],
  )

  useEffect(() => {
    if (!enabled) {
      dragRef.current = null
      setIsPanning(false)
      transformRef.current = DEFAULT_TRANSFORM
      setTransform(DEFAULT_TRANSFORM)
    }
  }, [enabled])

  const panTransform = enabled ? buildViewportTransformCss(transform) : 'none'

  useEffect(() => {
    transformRef.current = transform
  }, [transform])

  const value = useMemo<MeetingViewportPanContextValue>(
    () => ({
      enabled,
      isPanning,
      transform,
      transformRef,
      panTransform,
      shellProps: {
        onPointerDown,
        onPointerMove,
        onPointerUp,
        onPointerCancel,
        onContextMenu,
        onLostPointerCapture,
        onWheel,
      },
    }),
    [
      enabled,
      isPanning,
      transform,
      transformRef,
      panTransform,
      onPointerDown,
      onPointerMove,
      onPointerUp,
      onPointerCancel,
      onContextMenu,
      onLostPointerCapture,
      onWheel,
    ],
  )

  return <MeetingViewportPanContext.Provider value={value}>{children}</MeetingViewportPanContext.Provider>
}

export function useMeetingViewportPan() {
  const ctx = useContext(MeetingViewportPanContext)
  if (!ctx) {
    throw new Error('useMeetingViewportPan must be used within MeetingViewportPanProvider')
  }
  return ctx
}

interface MeetingViewportGridProps {
  className?: string
  children: ReactNode
}

/** Shared `#meet-grid` shell with right-drag pan, wheel zoom, and overflow clipping. */
export function MeetingViewportGrid({ className, children }: MeetingViewportGridProps) {
  const layout = useMeetingUILayout()
  const { isPanning, panTransform, shellProps } = useMeetingViewportPan()

  return (
    <div
      id="meet-grid"
      className={cn(
        'absolute top-0 left-0 bottom-0 z-0 overflow-hidden',
        'pt-[calc(56px+env(safe-area-inset-top))] pb-[calc(88px+env(safe-area-inset-bottom))]',
        'transition-[right] duration-200 touch-none select-none',
        isPanning && 'cursor-grabbing',
        meetRightInsetClass(layout),
        className,
      )}
      {...shellProps}
    >
      <div className="h-full w-full origin-top-left" style={{ transform: panTransform, willChange: 'transform' }}>
        {children}
      </div>
      <div id="meet-presence-layer" className="pointer-events-none absolute inset-0 z-[1]" />
    </div>
  )
}
