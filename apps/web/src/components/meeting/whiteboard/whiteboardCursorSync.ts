import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { syncWhiteboardToolCursor } from '@/components/meeting/whiteboard/whiteboardToolCursors'

/** Re-apply Bedrud tool cursors after Excalidraw's bundled setCursorForShape runs. */
export function attachWhiteboardCursorSync(container: HTMLElement, getApi: () => ExcalidrawImperativeAPI | null) {
  let frame = 0

  const schedule = () => {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(() => {
      syncWhiteboardToolCursor(getApi())
    })
  }

  container.addEventListener('pointermove', schedule, { passive: true })
  container.addEventListener('pointerdown', schedule, { passive: true })
  container.addEventListener('pointerup', schedule, { passive: true })

  return () => {
    cancelAnimationFrame(frame)
    container.removeEventListener('pointermove', schedule)
    container.removeEventListener('pointerdown', schedule)
    container.removeEventListener('pointerup', schedule)
  }
}