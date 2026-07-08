import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { resetWhiteboardToolCursorState } from '@/components/meeting/whiteboard/whiteboardToolCursors'

/** Reset custom whiteboard cursors and stray Excalidraw DOM after the overlay unmounts. */
export function releaseWhiteboardCursors(
  api?: ExcalidrawImperativeAPI | null,
  shell?: HTMLElement | null,
) {
  api?.resetCursor()
  resetWhiteboardToolCursorState()

  if (shell) {
    shell.style.cursor = ''
    const excalidraw = shell.querySelector('.excalidraw')
    if (excalidraw instanceof HTMLElement) excalidraw.style.cursor = ''
  }

  for (const el of document.querySelectorAll('.excalidraw-wysiwyg')) {
    el.remove()
  }

  document.body.classList.remove('excalidraw-cursor-resize')
  document.body.classList.remove('excalidraw-animations-disabled')
}