import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { applyWhiteboardToolCursor } from '@/components/meeting/whiteboard/whiteboardToolCursors'

type GetApi = () => ExcalidrawImperativeAPI | null

const GRABBING_CURSOR = 'grabbing'

function setPanCursor(container: HTMLElement, api: ExcalidrawImperativeAPI | null, cursor: string | null) {
  container.style.cursor = cursor ?? ''
  if (cursor === GRABBING_CURSOR) {
    api?.setCursor(GRABBING_CURSOR)
  } else {
    api?.resetCursor()
  }
}

export function attachWhiteboardRightDragPan(
  container: HTMLElement,
  getApi: GetApi,
  onViewportChange?: () => void,
) {
  const onContextMenu = (event: Event) => {
    event.preventDefault()
    event.stopImmediatePropagation()
    getApi()?.updateScene({ appState: { contextMenu: null } })
  }

  const onPointerDown = (event: PointerEvent) => {
    if (event.button !== 2) return

    const api = getApi()
    if (!api) return

    event.preventDefault()
    event.stopPropagation()

    const pointerId = event.pointerId
    const startX = event.clientX
    const startY = event.clientY
    const { scrollX: originScrollX, scrollY: originScrollY, zoom } = api.getAppState()

    setPanCursor(container, api, GRABBING_CURSOR)

    const onMove = (moveEvent: PointerEvent) => {
      if (moveEvent.pointerId !== pointerId) return
      moveEvent.preventDefault()
      const deltaX = moveEvent.clientX - startX
      const deltaY = moveEvent.clientY - startY

      api.updateScene({
        appState: {
          scrollX: originScrollX + deltaX / zoom.value,
          scrollY: originScrollY + deltaY / zoom.value,
          contextMenu: null,
        },
      })
      onViewportChange?.()
    }

    const endPan = (endEvent: PointerEvent) => {
      if (endEvent.pointerId !== pointerId) return
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', endPan)
      window.removeEventListener('pointercancel', endPan)
      setPanCursor(container, getApi(), null)
      applyWhiteboardToolCursor(getApi())
      getApi()?.updateScene({ appState: { contextMenu: null } })
    }

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', endPan)
    window.addEventListener('pointercancel', endPan)
  }

  container.addEventListener('contextmenu', onContextMenu, true)
  container.addEventListener('pointerdown', onPointerDown, true)

  return () => {
    container.removeEventListener('contextmenu', onContextMenu, true)
    container.removeEventListener('pointerdown', onPointerDown, true)
    setPanCursor(container, getApi(), null)
  }
}