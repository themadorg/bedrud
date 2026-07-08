import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'

function hasSelection(appState: ReturnType<ExcalidrawImperativeAPI['getAppState']>) {
  return Object.keys(appState.selectedElementIds).length > 0
}

function isInTextEditor(target: EventTarget | null) {
  return target instanceof HTMLElement && Boolean(target.closest('.excalidraw-wysiwyg'))
}

export function deselectWhiteboard(api: ExcalidrawImperativeAPI) {
  api.updateScene({
    appState: {
      selectedElementIds: {},
      selectedGroupIds: {},
      editingTextElement: null,
      selectedLinearElement: null,
      selectionElement: null,
      showHyperlinkPopup: false,
      activeEmbeddable: null,
    },
  })
}

/** Escape deselects text/elements; one press also clears selection after exiting text edit. */
export function handleWhiteboardEscapeKey(event: KeyboardEvent, api: ExcalidrawImperativeAPI) {
  if (event.key !== 'Escape') return

  const appState = api.getAppState()
  const inTextEditor = isInTextEditor(event.target) || appState.editingTextElement != null

  if (inTextEditor) {
    queueMicrotask(() => {
      const next = api.getAppState()
      if (!next.editingTextElement && hasSelection(next)) {
        deselectWhiteboard(api)
      }
    })
    return
  }

  if (!hasSelection(appState)) return

  event.preventDefault()
  event.stopPropagation()
  deselectWhiteboard(api)
}
