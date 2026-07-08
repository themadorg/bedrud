import type { AppState } from '@excalidraw/excalidraw/types'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { attachWhiteboardCursorSync } from '@/components/meeting/whiteboard/whiteboardCursorSync'
import { resetWhiteboardToolCursorState } from '@/components/meeting/whiteboard/whiteboardToolCursors'

function toolState(type: AppState['activeTool']['type'], theme: AppState['theme'] = 'dark') {
  return { activeTool: { type }, theme } as AppState
}

describe('whiteboardCursorSync', () => {
  afterEach(() => {
    resetWhiteboardToolCursorState()
    vi.restoreAllMocks()
  })

  it('re-applies eraser cursor after pointer moves', async () => {
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0)
      return 1
    })
    vi.stubGlobal('cancelAnimationFrame', () => {})

    const setCursor = vi.fn()
    const api = {
      getAppState: () => toolState('eraser'),
      setCursor,
      resetCursor: vi.fn(),
    }

    const container = document.createElement('div')
    const detach = attachWhiteboardCursorSync(container, () => api as never)

    container.dispatchEvent(new PointerEvent('pointermove', { bubbles: true }))
    expect(setCursor).toHaveBeenCalledTimes(1)
    expect(setCursor.mock.calls[0]?.[0]).toContain('10 10')

    detach()
  })
})
