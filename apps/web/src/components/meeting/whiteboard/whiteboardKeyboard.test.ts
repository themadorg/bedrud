import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { describe, expect, test, vi } from 'vitest'
import { deselectWhiteboard, handleWhiteboardEscapeKey } from './whiteboardKeyboard'

function mockApi(
  appState: Partial<ReturnType<ExcalidrawImperativeAPI['getAppState']>> = {},
): ExcalidrawImperativeAPI {
  let state = {
    selectedElementIds: {},
    editingTextElement: null,
    ...appState,
  } as ReturnType<ExcalidrawImperativeAPI['getAppState']>

  return {
    getAppState: () => state,
    updateScene: vi.fn((scene) => {
      if (scene.appState) {
        state = { ...state, ...scene.appState }
      }
    }),
  } as unknown as ExcalidrawImperativeAPI
}

describe('whiteboardKeyboard', () => {
  test('deselects selected elements on escape', () => {
    const api = mockApi({ selectedElementIds: { 'text-1': true } })
    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })

    handleWhiteboardEscapeKey(event, api)

    expect(event.defaultPrevented).toBe(true)
    expect(api.updateScene).toHaveBeenCalledWith(
      expect.objectContaining({
        appState: expect.objectContaining({
          selectedElementIds: {},
          editingTextElement: null,
        }),
      }),
    )
  })

  test('deselects after exiting text editor on escape', async () => {
    let state = {
      editingTextElement: { id: 'text-1' },
      selectedElementIds: {},
    } as ReturnType<ExcalidrawImperativeAPI['getAppState']>

    const api = {
      getAppState: () => state,
      updateScene: vi.fn((scene) => {
        if (scene.appState) state = { ...state, ...scene.appState }
      }),
    } as unknown as ExcalidrawImperativeAPI

    const editable = document.createElement('textarea')
    editable.className = 'excalidraw-wysiwyg'
    document.body.appendChild(editable)

    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
    Object.defineProperty(event, 'target', { value: editable })

    handleWhiteboardEscapeKey(event, api)

    state = {
      ...state,
      editingTextElement: null,
      selectedElementIds: { 'text-1': true },
    }

    await Promise.resolve()

    expect(api.updateScene).toHaveBeenCalledWith(
      expect.objectContaining({
        appState: expect.objectContaining({
          selectedElementIds: {},
        }),
      }),
    )

    editable.remove()
  })

  test('deselectWhiteboard clears selection state', () => {
    const api = mockApi({ selectedElementIds: { a: true, b: true } })
    deselectWhiteboard(api)
    expect(api.updateScene).toHaveBeenCalled()
  })
})