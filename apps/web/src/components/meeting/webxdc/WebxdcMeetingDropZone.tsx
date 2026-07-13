import { useCallback, useEffect, useRef, useState } from 'react'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { cn } from '@/lib/utils'
import { fetchWebxdcConfig } from './webxdcApi'
import { pickWebxdcFileFromDataTransfer } from './webxdcFile'
import { useOptionalWebxdcWatch } from './webxdc-watch-context'

/**
 * Full-meeting drag-and-drop target for .xdc packages.
 * Drop → upload + start + stage share for everyone.
 */
export function WebxdcMeetingDropZone() {
  const watch = useOptionalWebxdcWatch()
  const shareFile = watch?.shareFile
  const busy = watch?.busy ?? false
  const webxdcUserEnabled = useExperimentalPreferencesStore((s) => s.webxdcEnabled)
  const [serverEnabled, setServerEnabled] = useState(false)
  const [dragging, setDragging] = useState(false)
  const dragDepth = useRef(0)

  useEffect(() => {
    fetchWebxdcConfig()
      .then((c) => setServerEnabled(c.enabled === true))
      .catch(() => setServerEnabled(false))
  }, [])

  const enabled = webxdcUserEnabled && serverEnabled && !!shareFile

  const onDragEnter = useCallback(
    (e: DragEvent) => {
      if (!enabled || busy || !shareFile) return
      if (![...e.dataTransfer!.types].includes('Files')) return
      e.preventDefault()
      dragDepth.current += 1
      setDragging(true)
    },
    [enabled, busy, shareFile],
  )

  const onDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault()
    dragDepth.current = Math.max(0, dragDepth.current - 1)
    if (dragDepth.current === 0) setDragging(false)
  }, [])

  const onDragOver = useCallback(
    (e: DragEvent) => {
      if (!enabled || busy) return
      if (![...e.dataTransfer!.types].includes('Files')) return
      e.preventDefault()
      e.dataTransfer!.dropEffect = 'copy'
    },
    [enabled, busy],
  )

  const onDrop = useCallback(
    (e: DragEvent) => {
      if (!enabled || !shareFile) return
      e.preventDefault()
      e.stopPropagation()
      dragDepth.current = 0
      setDragging(false)
      const file = pickWebxdcFileFromDataTransfer(e.dataTransfer)
      if (!file) return
      void shareFile(file)
    },
    [enabled, shareFile],
  )

  useEffect(() => {
    if (!enabled) return
    // Capture on document so drops work anywhere in the meeting (stage, chrome, …).
    document.addEventListener('dragenter', onDragEnter)
    document.addEventListener('dragleave', onDragLeave)
    document.addEventListener('dragover', onDragOver)
    document.addEventListener('drop', onDrop)
    return () => {
      document.removeEventListener('dragenter', onDragEnter)
      document.removeEventListener('dragleave', onDragLeave)
      document.removeEventListener('dragover', onDragOver)
      document.removeEventListener('drop', onDrop)
    }
  }, [enabled, onDragEnter, onDragLeave, onDragOver, onDrop])

  if (!enabled || !dragging) return null

  return (
    <div
      className={cn(
        'pointer-events-none fixed z-[80] flex items-center justify-center',
        'left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)]',
        'h-[var(--app-height,100svh)] w-[var(--app-width,100svw)]',
        'bg-black/55 backdrop-blur-[2px]',
      )}
      aria-hidden
    >
      <div className="max-w-sm rounded-xl border-2 border-dashed border-primary bg-card/95 px-8 py-10 text-center shadow-2xl">
        <p className="text-base font-semibold text-foreground">Drop WebXDC package</p>
        <p className="mt-2 text-sm text-muted-foreground">
          Release to upload and share <code className="text-xs">.xdc</code> on stage for everyone
        </p>
        {busy ? <p className="mt-3 text-xs text-primary">Uploading…</p> : null}
      </div>
    </div>
  )
}
