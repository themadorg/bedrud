import { exportToClipboard, MIME_TYPES } from '@excalidraw/excalidraw'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'

function isMac() {
  return typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform)
}

export function pasteToWhiteboard() {
  const event = new KeyboardEvent('keydown', {
    key: 'v',
    code: 'KeyV',
    ctrlKey: !isMac(),
    metaKey: isMac(),
    bubbles: true,
    cancelable: true,
  })
  document.dispatchEvent(event)
}

export function selectAllWhiteboardElements(api: ExcalidrawImperativeAPI) {
  const selectedElementIds = Object.fromEntries(
    api.getSceneElements().map((element) => [element.id, true as const]),
  ) as Record<string, true>
  api.updateScene({ appState: { selectedElementIds } })
}

export async function copyWhiteboardAsPng(api: ExcalidrawImperativeAPI) {
  const elements = api.getSceneElements()
  const appState = api.getAppState()
  await exportToClipboard({
    elements,
    appState,
    files: api.getFiles(),
    type: 'png',
    mimeType: MIME_TYPES.png,
  })
}

export async function copyWhiteboardAsSvg(api: ExcalidrawImperativeAPI) {
  const elements = api.getSceneElements()
  const appState = api.getAppState()
  await exportToClipboard({
    elements,
    appState,
    files: api.getFiles(),
    type: 'svg',
  })
}

export function toggleWhiteboardGrid(api: ExcalidrawImperativeAPI) {
  const { gridModeEnabled } = api.getAppState()
  api.updateScene({ appState: { gridModeEnabled: !gridModeEnabled } })
}

export function toggleWhiteboardSnap(api: ExcalidrawImperativeAPI) {
  const { objectsSnapModeEnabled } = api.getAppState()
  api.updateScene({ appState: { objectsSnapModeEnabled: !objectsSnapModeEnabled } })
}

export function toggleWhiteboardZenMode(api: ExcalidrawImperativeAPI) {
  const { zenModeEnabled } = api.getAppState()
  api.updateScene({ appState: { zenModeEnabled: !zenModeEnabled } })
}

export function toggleWhiteboardViewMode(api: ExcalidrawImperativeAPI) {
  const { viewModeEnabled } = api.getAppState()
  api.updateScene({ appState: { viewModeEnabled: !viewModeEnabled } })
}

export function toggleWhiteboardStats(api: ExcalidrawImperativeAPI) {
  const { stats } = api.getAppState()
  api.updateScene({ appState: { stats: { ...stats, open: !stats.open } } })
}