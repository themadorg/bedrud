import type { AppState } from '@excalidraw/excalidraw/types'
import { describe, expect, it, vi } from 'vitest'
import { applyWhiteboardToolCursor, resetWhiteboardToolCursorState, whiteboardToolCursor } from '@/components/meeting/whiteboard/whiteboardToolCursors'

function toolState(type: AppState['activeTool']['type'], theme: AppState['theme'] = 'dark') {
  return { activeTool: { type }, theme } as Pick<AppState, 'activeTool' | 'theme'>
}

describe('whiteboardToolCursors', () => {
  it('does not override the bundled laser cursor', () => {
    expect(whiteboardToolCursor(toolState('laser', 'light'))).toBeNull()
  })

  it('anchors eraser hotspot at the circle center', () => {
    const cursor = whiteboardToolCursor(toolState('eraser'))
    expect(cursor).toContain('10 10')
    expect(cursor).toContain('circle')
  })

  it('returns null for unrelated tools', () => {
    expect(whiteboardToolCursor(toolState('selection'))).toBeNull()
  })

  it('skips redundant cursor applications', () => {
    resetWhiteboardToolCursorState()
    const setCursor = vi.fn()
    const resetCursor = vi.fn()
    const api = {
      getAppState: () => toolState('eraser'),
      setCursor,
      resetCursor,
    }

    applyWhiteboardToolCursor(api as never)
    applyWhiteboardToolCursor(api as never)

    expect(setCursor).toHaveBeenCalledTimes(1)
    expect(resetCursor).not.toHaveBeenCalled()
  })
})