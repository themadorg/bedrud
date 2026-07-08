import type { AppState, ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'

const ERASER_CURSOR_SIZE = 20
const ERASER_HOTSPOT = ERASER_CURSOR_SIZE / 2

function svgCursorDataUrl(svg: string) {
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

const eraserCanvasCache = new Map<AppState['theme'], string>()

function eraserCursor(theme: AppState['theme']) {
  let dataUrl = eraserCanvasCache.get(theme)
  if (!dataUrl) {
    const isDark = theme === 'dark'
    const fill = isDark ? '#000000' : '#ffffff'
    const stroke = isDark ? '#ffffff' : '#000000'
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${ERASER_CURSOR_SIZE}" height="${ERASER_CURSOR_SIZE}"><circle cx="${ERASER_HOTSPOT}" cy="${ERASER_HOTSPOT}" r="5" fill="${fill}" stroke="${stroke}" stroke-width="1"/></svg>`
    dataUrl = svgCursorDataUrl(svg)
    eraserCanvasCache.set(theme, dataUrl)
  }
  return `url("${dataUrl}") ${ERASER_HOTSPOT} ${ERASER_HOTSPOT}, auto`
}

const cursorCache = new Map<string, string>()
let lastAppliedKey: string | null = null
let hadCustomCursor = false

function cachedEraserCursor(theme: AppState['theme']) {
  const key = `eraser:${theme}`
  let cursor = cursorCache.get(key)
  if (!cursor) {
    cursor = eraserCursor(theme)
    cursorCache.set(key, cursor)
  }
  return cursor
}

export function whiteboardToolCursor(appState: Pick<AppState, 'activeTool' | 'theme'>): string | null {
  if (appState.activeTool.type === 'eraser') return cachedEraserCursor(appState.theme)
  return null
}

function setWhiteboardToolCursor(
  api: ExcalidrawImperativeAPI,
  state: Pick<AppState, 'activeTool' | 'theme'>,
) {
  const cursor = whiteboardToolCursor(state)
  const nextKey = cursor ? `${state.activeTool.type}:${state.theme}` : null

  if (cursor) {
    api.setCursor(cursor)
    hadCustomCursor = true
    lastAppliedKey = nextKey
    return
  }

  if (hadCustomCursor) {
    api.resetCursor()
    hadCustomCursor = false
  }
  lastAppliedKey = null
}

export function applyWhiteboardToolCursor(
  api: ExcalidrawImperativeAPI | null,
  appState?: Pick<AppState, 'activeTool' | 'theme'>,
) {
  if (!api) return

  const state = appState ?? api.getAppState()
  const nextKey = whiteboardToolCursor(state)
    ? `${state.activeTool.type}:${state.theme}`
    : null
  if (nextKey === lastAppliedKey) return

  setWhiteboardToolCursor(api, state)
}

export function syncWhiteboardToolCursor(
  api: ExcalidrawImperativeAPI | null,
  appState?: Pick<AppState, 'activeTool' | 'theme'>,
) {
  if (!api) return
  setWhiteboardToolCursor(api, appState ?? api.getAppState())
}

/** Clear module state — for tests only. */
export function resetWhiteboardToolCursorState() {
  lastAppliedKey = null
  hadCustomCursor = false
  eraserCanvasCache.clear()
  cursorCache.clear()
}