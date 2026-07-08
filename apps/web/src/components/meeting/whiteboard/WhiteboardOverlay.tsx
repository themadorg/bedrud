import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { useCallback, useEffect, useRef } from 'react'
import { meetRightInsetClass, useMeetingUILayout } from '@/components/meeting/MeetingUILayoutContext'
import { MeetingSharedWhiteboard } from '@/components/meeting/whiteboard/MeetingSharedWhiteboard'
import { useWhiteboardElementLocks } from '@/components/meeting/whiteboard/useWhiteboardElementLocks'
import { useWhiteboardFollowSync } from '@/components/meeting/whiteboard/useWhiteboardFollowSync'
import { useWhiteboardPointerSync } from '@/components/meeting/whiteboard/useWhiteboardPointerSync'
import { useWhiteboardWatch } from '@/components/meeting/whiteboard/whiteboard-watch-context'
import { releaseWhiteboardCursors } from '@/components/meeting/whiteboard/whiteboardTeardown'
import { cn } from '@/lib/utils'

export function WhiteboardOverlay() {
  const layout = useMeetingUILayout()
  const { session, isHost, ydoc, whiteboardVisible, stopWhiteboard, flushWhiteboardSync } = useWhiteboardWatch()
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null)
  const { onPointerUpdate, notifyApiReady: notifyPointerApiReady } = useWhiteboardPointerSync(apiRef, whiteboardVisible)
  const { notifyApiReady: notifyFollowApiReady, relayViewport } = useWhiteboardFollowSync(apiRef, whiteboardVisible)
  const {
    getLocks,
    localIdentity,
    onPointerUp: onLockPointerUp,
    notifyApiReady: notifyLocksApiReady,
  } = useWhiteboardElementLocks(apiRef, ydoc, whiteboardVisible)
  const handleViewportChange = useCallback(() => relayViewport(), [relayViewport])

  useEffect(() => {
    if (session && ydoc) return
    releaseWhiteboardCursors(apiRef.current)
    apiRef.current = null
  }, [session, ydoc])

  if (!whiteboardVisible || !session || !ydoc) return null

  return (
    <div
      className={cn(
        'absolute top-[calc(56px+env(safe-area-inset-top))] left-0 bottom-[calc(88px+env(safe-area-inset-bottom))] z-[5] flex flex-col p-3 transition-[right] duration-200',
        meetRightInsetClass(layout),
      )}
    >
      {/* Blur on a backdrop layer only — filter on an ancestor breaks Excalidraw's fixed SVGLayer (laser/eraser trails). */}
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-white/[0.08] shadow-2xl">
        <div className="pointer-events-none absolute inset-0 rounded-xl bg-[#030308]/95 backdrop-blur-md" aria-hidden />
        <div className="relative min-h-0 flex-1 bg-[#12121f]">
          <MeetingSharedWhiteboard
            ydoc={ydoc}
            onApiReady={(api) => {
              apiRef.current = api
              notifyPointerApiReady()
              notifyFollowApiReady()
              notifyLocksApiReady()
            }}
            localIdentity={localIdentity}
            getLocks={getLocks}
            onPointerUp={onLockPointerUp}
            onClose={isHost ? () => stopWhiteboard() : undefined}
            onSyncFlush={flushWhiteboardSync}
            onPointerUpdate={onPointerUpdate}
            onViewportChange={handleViewportChange}
          />
        </div>
      </div>
    </div>
  )
}
