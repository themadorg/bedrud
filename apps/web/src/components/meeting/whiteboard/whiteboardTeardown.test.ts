import { describe, expect, it, vi } from 'vitest'
import { releaseWhiteboardCursors } from '@/components/meeting/whiteboard/whiteboardTeardown'

describe('whiteboardTeardown', () => {
  it('resets api cursor and clears body classes', () => {
    document.body.classList.add('excalidraw-cursor-resize')

    const resetCursor = vi.fn()
    const shell = document.createElement('div')
    const excalidraw = document.createElement('div')
    excalidraw.className = 'excalidraw'
    excalidraw.style.cursor = 'grabbing'
    shell.appendChild(excalidraw)

    releaseWhiteboardCursors({ resetCursor } as never, shell)

    expect(resetCursor).toHaveBeenCalledTimes(1)
    expect(excalidraw.style.cursor).toBe('')
    expect(document.body.classList.contains('excalidraw-cursor-resize')).toBe(false)
  })
})