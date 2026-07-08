import { bindExcalidrawToYDoc } from '@/components/meeting/whiteboard/excalidrawYjsBinding'
import { handleWhiteboardEscapeKey } from '@/components/meeting/whiteboard/whiteboardKeyboard'
import { alignRtlTextElements } from '@/components/meeting/whiteboard/whiteboardTextDirection'
import { WhiteboardMainMenu } from '@/components/meeting/whiteboard/WhiteboardMainMenu'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import type { CollaboratorPointer, Gesture } from '@excalidraw/excalidraw/types'
import { X } from 'lucide-react'
import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import type * as Y from 'yjs'
import { attachWhiteboardCursorSync } from '@/components/meeting/whiteboard/whiteboardCursorSync'
import { attachWhiteboardRightDragPan } from '@/components/meeting/whiteboard/whiteboardRightDragPan'
import { releaseWhiteboardCursors } from '@/components/meeting/whiteboard/whiteboardTeardown'
import { applyWhiteboardToolCursor } from '@/components/meeting/whiteboard/whiteboardToolCursors'
import '@/vendor/excalidraw/packages/excalidraw/css/app.scss'
import '@/vendor/excalidraw/packages/excalidraw/css/styles.scss'
import '@/vendor/excalidraw/packages/excalidraw/fonts/fonts.css'

const Excalidraw = lazy(() =>
  import('@excalidraw/excalidraw').then((m) => ({
    default: m.Excalidraw,
  })),
)

interface MeetingSharedWhiteboardProps {
  ydoc: Y.Doc
  localIdentity: string
  getLocks: () => ReadonlyMap<string, { identity: string; username: string }>
  onApiReady: (api: ExcalidrawImperativeAPI) => void
  onClose?: () => void
  onSyncFlush?: () => void
  onPointerUpdate?: (payload: {
    pointer: CollaboratorPointer
    button: 'up' | 'down'
    pointersMap: Gesture['pointers']
  }) => void
  onPointerUp?: () => void
  onViewportChange?: () => void
}

export function MeetingSharedWhiteboard({
  ydoc,
  localIdentity,
  getLocks,
  onApiReady,
  onClose,
  onSyncFlush,
  onPointerUpdate,
  onPointerUp,
  onViewportChange,
}: MeetingSharedWhiteboardProps) {
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)
  const shellRef = useRef<HTMLDivElement>(null)
  const bindingRef = useRef<ReturnType<typeof bindExcalidrawToYDoc> | null>(null)
  const [panSurface, setPanSurface] = useState<HTMLElement | null>(null)

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const api = apiRef.current
      if (!api) return
      handleWhiteboardEscapeKey(event, api)
    }

    document.addEventListener('keydown', onKeyDown, true)
    return () => {
      document.removeEventListener('keydown', onKeyDown, true)
      bindingRef.current?.destroy()
      bindingRef.current = null
      releaseWhiteboardCursors(apiRef.current, shellRef.current)
      apiRef.current = null
    }
  }, [])

  useEffect(() => {
    const api = apiRef.current
    if (!api) return

    bindingRef.current?.destroy()
    bindingRef.current = bindExcalidrawToYDoc(api, ydoc, { localIdentity, getLocks })
  }, [getLocks, localIdentity, ydoc])

  const handleViewportChange = useCallback(() => {
    onViewportChange?.()
    const api = apiRef.current
    if (!api) return
    api.refresh()
    window.dispatchEvent(new Event('resize'))
  }, [onViewportChange])

  useEffect(() => {
    if (!panSurface) return
    const detachPan = attachWhiteboardRightDragPan(panSurface, () => apiRef.current, handleViewportChange)
    const detachCursor = attachWhiteboardCursorSync(panSurface, () => apiRef.current)
    return () => {
      detachPan()
      detachCursor()
    }
  }, [handleViewportChange, panSurface])

  useEffect(() => {
    const shell = shellRef.current
    const api = apiRef.current
    if (!shell || !api) return

    let timer: ReturnType<typeof setTimeout> | null = null
    const scheduleRefresh = () => {
      if (timer != null) return
      timer = setTimeout(() => {
        timer = null
        api.refresh()
        applyWhiteboardToolCursor(api)
      }, 150)
    }

    const observer = new ResizeObserver(scheduleRefresh)
    observer.observe(shell)

    return () => {
      if (timer != null) clearTimeout(timer)
      observer.disconnect()
    }
  }, [panSurface])

  return (
    <Suspense
      fallback={
        <div className="flex h-full w-full items-center justify-center text-sm text-white/50">Loading whiteboard…</div>
      }
    >
      <div
        ref={shellRef}
        className="bedrud-whiteboard-shell h-full w-full [&_.excalidraw]:h-full [&_.context-menu]:hidden [&_.excalidraw-contextMenuContainer]:hidden"
      >
        <Excalidraw
          autoFocus
          handleKeyboardGlobally
          isCollaborating
          onPointerUpdate={onPointerUpdate}
          excalidrawAPI={(api) => {
            apiRef.current = api
            onApiReady(api)
            bindingRef.current?.destroy()
            bindingRef.current = bindExcalidrawToYDoc(api, ydoc, { localIdentity, getLocks })
            requestAnimationFrame(() => {
              const container = shellRef.current?.querySelector('.excalidraw')
              if (container instanceof HTMLElement) setPanSurface(container)
              api.refresh()
              applyWhiteboardToolCursor(api)
            })
          }}
          theme="dark"
          onChange={(elements, appState, files) => {
            const aligned = alignRtlTextElements(elements)
            const sceneElements = aligned ?? elements
            if (aligned) {
              apiRef.current?.updateScene({ elements: aligned, captureUpdate: 'NEVER' })
            }
            bindingRef.current?.onExcalidrawChange(sceneElements, appState, files)
            applyWhiteboardToolCursor(apiRef.current, appState)
          }}
          onPointerUp={() => {
            const api = apiRef.current
            const binding = bindingRef.current
            if (!api || !binding) return
            binding.onExcalidrawChange(api.getSceneElementsIncludingDeleted(), api.getAppState(), api.getFiles())
            binding.flush()
            onSyncFlush?.()
            onPointerUp?.()
          }}
          renderTopRightUI={() =>
            onClose ? (
              <button
                type="button"
                onClick={onClose}
                aria-label="Close shared whiteboard"
                title="Close whiteboard"
                className="flex h-9 w-9 items-center justify-center rounded-lg border-none cursor-pointer"
                style={{
                  background: 'var(--island-bg-color)',
                  color: 'var(--icon-fill-color)',
                }}
              >
                <X size={18} />
              </button>
            ) : null
          }
          UIOptions={{
            canvasActions: {
              toggleTheme: false,
              export: false,
              loadScene: false,
              saveToActiveFile: false,
              saveAsImage: false,
              clearCanvas: false,
            },
          }}
        >
          <WhiteboardMainMenu apiRef={apiRef} />
        </Excalidraw>
      </div>
    </Suspense>
  )
}