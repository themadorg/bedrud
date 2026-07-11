import { Download, Minus, Plus, X } from 'lucide-react'
import { type PointerEvent, useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useAuthStore } from '#/lib/auth.store'

const MIN_ZOOM = 1
const MAX_ZOOM = 4
const ZOOM_STEP = 0.25
/** Higher zoom → slower pan so fine positioning is easier. */
function panDragScale(zoom: number): number {
  return 1 / (zoom * 0.65)
}

interface Props {
  url: string | null
  onClose: () => void
}

export function ChatImageLightbox({ url, onClose }: Props) {
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const dragging = useRef(false)
  const pointerMoved = useRef(false)
  const dragStart = useRef({ x: 0, y: 0, panX: 0, panY: 0, zoom: 1 })
  const viewportRef = useRef<HTMLDivElement>(null)
  const [downloading, setDownloading] = useState(false)

  // biome-ignore lint/correctness/useExhaustiveDependencies: url is intentional trigger to reset zoom/pan on image change
  useEffect(() => {
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }, [url])

  useEffect(() => {
    if (!url) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
    }
  }, [url, onClose])

  const clampZoom = useCallback((value: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value)), [])

  const zoomIn = useCallback(() => setZoom((z) => clampZoom(z + ZOOM_STEP)), [clampZoom])
  const zoomOut = useCallback(() => {
    setZoom((z) => {
      const next = clampZoom(z - ZOOM_STEP)
      if (next <= 1) setPan({ x: 0, y: 0 })
      return next
    })
  }, [clampZoom])

  const resetZoom = useCallback(() => {
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }, [])

  const downloadImage = useCallback(async () => {
    if (!url || downloading) return
    setDownloading(true)
    try {
      let blob: Blob
      let filename = 'chat-image'

      if (url.startsWith('data:')) {
        const res = await fetch(url)
        blob = await res.blob()
        const mimeMatch = url.match(/^data:image\/([\w+.-]+)/)
        if (mimeMatch) {
          const ext = mimeMatch[1] === 'jpeg' ? 'jpg' : mimeMatch[1]
          filename = `chat-image.${ext}`
        }
      } else {
        const fullUrl = url.startsWith('http') ? url : `${window.location.origin}${url}`
        const headers: Record<string, string> = {}
        const token = useAuthStore.getState().tokens?.accessToken
        if (token) headers.Authorization = `Bearer ${token}`
        const res = await fetch(fullUrl, { credentials: 'include', headers })
        if (!res.ok) throw new Error('Download failed')
        blob = await res.blob()
        const ext = url.split('.').pop()?.split('?')[0] || 'png'
        filename = `chat-image.${ext}`
      }

      const objectUrl = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = objectUrl
      anchor.download = filename
      anchor.click()
      URL.revokeObjectURL(objectUrl)
    } catch {
      // Best-effort fallback: open in a new tab when download is blocked.
      window.open(url, '_blank', 'noopener,noreferrer')
    } finally {
      setDownloading(false)
    }
  }, [url, downloading])

  const handleWheel = useCallback(
    (e: WheelEvent) => {
      e.preventDefault()
      const delta = e.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP
      setZoom((z) => {
        const next = clampZoom(z + delta)
        if (next <= 1) setPan({ x: 0, y: 0 })
        return next
      })
    },
    [clampZoom],
  )

  useEffect(() => {
    const el = viewportRef.current
    if (!url || !el) return
    el.addEventListener('wheel', handleWheel, { passive: false })
    return () => el.removeEventListener('wheel', handleWheel)
  }, [url, handleWheel])

  const handlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    pointerMoved.current = false
    if (zoom <= 1) return
    dragging.current = true
    dragStart.current = { x: e.clientX, y: e.clientY, panX: pan.x, panY: pan.y, zoom }
    e.currentTarget.setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging.current) return
    pointerMoved.current = true
    const scale = panDragScale(dragStart.current.zoom)
    setPan({
      x: dragStart.current.panX + (e.clientX - dragStart.current.x) * scale,
      y: dragStart.current.panY + (e.clientY - dragStart.current.y) * scale,
    })
  }

  const handlePointerUp = (e: PointerEvent<HTMLDivElement>) => {
    dragging.current = false
    e.currentTarget.releasePointerCapture(e.pointerId)
  }

  if (!url) return null

  return createPortal(
    <div
      className="app-fixed-viewport z-[100] flex flex-col bg-black/90"
      role="dialog"
      aria-modal="true"
      aria-label="Image preview"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onClose()
      }}
    >
      <div className="absolute top-[max(12px,calc(12px+env(safe-area-inset-top,0px)))] end-[max(12px,calc(12px+env(safe-area-inset-right,0px)))] z-20 flex items-center gap-2">
        <button
          type="button"
          onClick={() => void downloadImage()}
          disabled={downloading}
          className="flex h-11 w-11 items-center justify-center rounded-xl border border-white/15 bg-black/50 text-white/90 backdrop-blur-sm transition-colors hover:bg-black/70 hover:text-white disabled:opacity-50"
          aria-label="Download image"
        >
          <Download size={18} />
        </button>
        <button
          type="button"
          onClick={onClose}
          className="flex h-11 w-11 items-center justify-center rounded-xl border border-white/15 bg-black/50 text-white/90 backdrop-blur-sm transition-colors hover:bg-black/70 hover:text-white"
          aria-label="Close image preview"
        >
          <X size={20} />
        </button>
      </div>

      {/* biome-ignore lint/a11y/noStaticElementInteractions: interactive image viewport for pan/zoom/click-to-close */}
      <div
        ref={viewportRef}
        className={`flex flex-1 items-center justify-center overflow-hidden touch-none ${zoom > 1 ? 'cursor-grab active:cursor-grabbing' : 'cursor-default'}`}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        // biome-ignore lint/a11y/noNoninteractiveTabindex: interactive image viewport for pan/zoom
        tabIndex={0}
        onClick={(e) => {
          if (e.target !== e.currentTarget || pointerMoved.current) {
            pointerMoved.current = false
            return
          }
          onClose()
        }}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose()
        }}
      >
        <img
          src={url}
          alt="Chat attachment"
          draggable={false}
          className="max-h-[calc(var(--app-height,100svh)*0.85)] max-w-[calc(var(--app-width,100svw)*0.92)] select-none object-contain"
          style={{ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})` }}
        />
      </div>

      <div className="absolute bottom-[max(16px,calc(16px+env(safe-area-inset-bottom,0px)))] left-1/2 z-20 flex -translate-x-1/2 items-center gap-1 rounded-xl border border-white/15 bg-black/50 p-1 backdrop-blur-sm">
        <button
          type="button"
          onClick={zoomOut}
          disabled={zoom <= MIN_ZOOM}
          className="flex h-9 w-9 items-center justify-center rounded-lg text-white/90 transition-colors hover:bg-white/10 disabled:cursor-default disabled:opacity-35"
          aria-label="Zoom out"
        >
          <Minus size={18} />
        </button>
        <button
          type="button"
          onClick={resetZoom}
          className="min-w-[52px] px-2 text-center text-[12px] font-medium tabular-nums text-white/80 transition-colors hover:text-white"
          aria-label="Reset zoom"
        >
          {Math.round(zoom * 100)}%
        </button>
        <button
          type="button"
          onClick={zoomIn}
          disabled={zoom >= MAX_ZOOM}
          className="flex h-9 w-9 items-center justify-center rounded-lg text-white/90 transition-colors hover:bg-white/10 disabled:cursor-default disabled:opacity-35"
          aria-label="Zoom in"
        >
          <Plus size={18} />
        </button>
      </div>
    </div>,
    document.body,
  )
}
