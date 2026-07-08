import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import { type RefObject, useCallback, useEffect, useRef } from 'react'
import type * as Y from 'yjs'
import {
  acquireElementLocks,
  type ElementLockSnapshot,
  heldLockElementIds,
  readLockSnapshot,
  releaseAllLocksForIdentity,
  releaseElementLocks,
} from '@/components/meeting/whiteboard/whiteboardElementLocks'

export function useWhiteboardElementLocks(
  apiRef: RefObject<ExcalidrawImperativeAPI | null>,
  ydoc: Y.Doc | null,
  enabled: boolean,
) {
  const room = useRoomContext()
  const locksRef = useRef<ElementLockSnapshot>(new Map())
  const heldRef = useRef<Set<string>>(new Set())
  const drawingIdsRef = useRef<Set<string>>(new Set())
  const previousElementIdsRef = useRef<Set<string>>(new Set())
  const teardownRef = useRef<(() => void) | null>(null)

  const localId = room.localParticipant.identity
  const localName = room.localParticipant.name || localId

  const syncHeldLocks = useCallback(
    (heldIds: Iterable<string>) => {
      if (!ydoc) return
      const nextHeld = new Set(heldIds)
      const prevHeld = heldRef.current
      const toAcquire: string[] = []
      const toRelease: string[] = []

      for (const id of nextHeld) {
        if (!prevHeld.has(id)) toAcquire.push(id)
      }
      for (const id of prevHeld) {
        if (!nextHeld.has(id)) toRelease.push(id)
      }

      if (toAcquire.length > 0) acquireElementLocks(ydoc, localId, localName, toAcquire)
      if (toRelease.length > 0) releaseElementLocks(ydoc, localId, toRelease)
      heldRef.current = nextHeld
    },
    [localId, localName, ydoc],
  )

  const bindApi = useCallback(() => {
    teardownRef.current?.()
    teardownRef.current = null

    const api = apiRef.current
    if (!api || !ydoc || !enabled) return

    const refreshLocks = () => {
      locksRef.current = readLockSnapshot(ydoc)
    }
    refreshLocks()

    const onLocksChange = () => refreshLocks()
    const yLocks = ydoc.getMap('locks')
    yLocks.observe(onLocksChange)

    const unsubChange = api.onChange((elements, appState) => {
      const currentIds = new Set(elements.filter((el) => !el.isDeleted).map((el) => el.id))
      for (const id of currentIds) {
        if (!previousElementIdsRef.current.has(id)) {
          drawingIdsRef.current.add(id)
        }
      }
      previousElementIdsRef.current = currentIds

      const held = heldLockElementIds(appState, drawingIdsRef.current)
      syncHeldLocks(held)
    })

    teardownRef.current = () => {
      yLocks.unobserve(onLocksChange)
      unsubChange()
    }
  }, [apiRef, enabled, syncHeldLocks, ydoc])

  useEffect(() => {
    if (enabled && apiRef.current && ydoc) bindApi()
  }, [apiRef, bindApi, enabled, ydoc])

  useEffect(() => {
    if (!enabled || !ydoc) {
      teardownRef.current?.()
      teardownRef.current = null
      locksRef.current = new Map()
      heldRef.current = new Set()
      drawingIdsRef.current = new Set()
      previousElementIdsRef.current = new Set()
      return
    }

    const onParticipantDisconnected = (participant: { identity: string }) => {
      releaseAllLocksForIdentity(ydoc, participant.identity)
    }

    room.on(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)

    return () => {
      room.off(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
      releaseAllLocksForIdentity(ydoc, localId)
      teardownRef.current?.()
      teardownRef.current = null
    }
  }, [enabled, localId, room, ydoc])

  const onPointerUp = useCallback(() => {
    drawingIdsRef.current.clear()
    const api = apiRef.current
    if (!api || !ydoc) return
    const held = heldLockElementIds(api.getAppState(), drawingIdsRef.current)
    syncHeldLocks(held)
  }, [apiRef, syncHeldLocks, ydoc])

  const getLocks = useCallback(() => locksRef.current, [])

  const notifyApiReady = useCallback(() => {
    bindApi()
  }, [bindApi])

  return { getLocks, localIdentity: localId, onPointerUp, notifyApiReady }
}
