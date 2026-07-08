export type ViewportTransform = {
  panX: number
  panY: number
  zoom: number
}

export type ContentInsets = {
  top: number
  right: number
  bottom: number
  left: number
}

export const ZERO_CONTENT_INSETS: ContentInsets = { top: 0, right: 0, bottom: 0, left: 0 }

export function readElementContentInsets(element: HTMLElement): ContentInsets {
  const style = getComputedStyle(element)
  return {
    top: Number.parseFloat(style.paddingTop) || 0,
    right: Number.parseFloat(style.paddingRight) || 0,
    bottom: Number.parseFloat(style.paddingBottom) || 0,
    left: Number.parseFloat(style.paddingLeft) || 0,
  }
}

export const MIN_VIEWPORT_ZOOM = 0.5
export const MAX_VIEWPORT_ZOOM = 2.5

export function clampViewportZoom(zoom: number): number {
  return Math.min(MAX_VIEWPORT_ZOOM, Math.max(MIN_VIEWPORT_ZOOM, zoom))
}

/** Map client coords to normalized content space (0–1), accounting for shell padding, pan, and zoom. */
export function clientToContentNorm(
  clientX: number,
  clientY: number,
  rect: DOMRect,
  transform: ViewportTransform,
  insets: ContentInsets = ZERO_CONTENT_INSETS,
): { x: number; y: number } | null {
  const contentWidth = rect.width - insets.left - insets.right
  const contentHeight = rect.height - insets.top - insets.bottom
  if (contentWidth <= 0 || contentHeight <= 0) return null

  const contentLeft = rect.left + insets.left
  const contentTop = rect.top + insets.top
  const contentRight = rect.right - insets.right
  const contentBottom = rect.bottom - insets.bottom

  if (clientX < contentLeft || clientX > contentRight || clientY < contentTop || clientY > contentBottom) {
    return null
  }

  const localX = clientX - contentLeft
  const localY = clientY - contentTop
  const contentX = (localX - transform.panX) / transform.zoom
  const contentY = (localY - transform.panY) / transform.zoom

  return {
    x: Math.min(1, Math.max(0, contentX / contentWidth)),
    y: Math.min(1, Math.max(0, contentY / contentHeight)),
  }
}

function contentNormToLocal(
  norm: { x: number; y: number },
  rect: DOMRect,
  transform: ViewportTransform,
  insets: ContentInsets,
): { x: number; y: number } {
  const contentWidth = rect.width - insets.left - insets.right
  const contentHeight = rect.height - insets.top - insets.bottom

  return {
    x: insets.left + transform.panX + norm.x * contentWidth * transform.zoom,
    y: insets.top + transform.panY + norm.y * contentHeight * transform.zoom,
  }
}

/** Inverse of clientToContentNorm — maps normalized content coords back to viewport pixels. */
export function contentNormToClient(
  norm: { x: number; y: number },
  rect: DOMRect,
  transform: ViewportTransform,
  insets: ContentInsets = ZERO_CONTENT_INSETS,
): { x: number; y: number } {
  const local = contentNormToLocal(norm, rect, transform, insets)
  return {
    x: rect.left + local.x,
    y: rect.top + local.y,
  }
}

/** Maps normalized content coords to pixels inside `#meet-grid` (for absolute positioning). */
export function contentNormToGridLocal(
  norm: { x: number; y: number },
  rect: DOMRect,
  transform: ViewportTransform,
  insets: ContentInsets = ZERO_CONTENT_INSETS,
): { x: number; y: number } {
  return contentNormToLocal(norm, rect, transform, insets)
}

export function buildViewportTransformCss({ panX, panY, zoom }: ViewportTransform): string {
  return `translate(${panX}px, ${panY}px) scale(${zoom})`
}

/** Zoom toward a viewport point while keeping that point anchored under the cursor. */
export function zoomAtClientPoint(
  transform: ViewportTransform,
  rect: DOMRect,
  clientX: number,
  clientY: number,
  nextZoom: number,
  insets: ContentInsets = ZERO_CONTENT_INSETS,
): ViewportTransform {
  const localX = clientX - rect.left - insets.left
  const localY = clientY - rect.top - insets.top
  const ratio = nextZoom / transform.zoom

  return {
    panX: localX - (localX - transform.panX) * ratio,
    panY: localY - (localY - transform.panY) * ratio,
    zoom: nextZoom,
  }
}

export function wheelZoomFactor(deltaY: number): number {
  return deltaY < 0 ? 1.08 : 1 / 1.08
}
