import type {
  Collaborator,
  CollaboratorPointer,
  ExcalidrawImperativeAPI,
  Gesture,
  SocketId,
} from '@excalidraw/excalidraw/types'
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import { type RefObject, useCallback, useEffect, useRef } from 'react'
import { isPublishUnavailableError, isRoomConnected } from '#/lib/livekit-publish'
import { avatarColor } from '@/components/meeting/chat/chatGrouping'
import { selectedElementIds } from '@/components/meeting/whiteboard/whiteboardElementLocks'
import {
  decodeWhiteboardPointerPacket,
  encodeWhiteboardPointerPacket,
  WHITEBOARD_POINTER_TOPIC,
  type WhiteboardPointerPacket,
} from '@/components/meeting/whiteboard/whiteboardPointerWire'

const POINTER_SYNC_MS = 33

function toSharedPointer(pointer: CollaboratorPointer): WhiteboardPointerPacket['pointer'] {
  if (pointer.tool === 'laser') {
    return { x: pointer.x, y: pointer.y, tool: 'laser' }
  }
  return {
    x: pointer.x,
    y: pointer.y,
    tool: 'pointer',
    renderCursor: true,
  }
}

function toCollaboratorPointer(packet: WhiteboardPointerPacket['pointer']): CollaboratorPointer {
  if (packet.tool === 'laser') {
    return { x: packet.x, y: packet.y, tool: 'laser' }
  }
  return {
    x: packet.x,
    y: packet.y,
    tool: 'pointer',
    renderCursor: packet.renderCursor ?? true,
  }
}

function throttlePointerSync(
  fn: (pointer: CollaboratorPointer, button: 'up' | 'down', selectedIds?: string[]) => void,
): (pointer: CollaboratorPointer, button: 'up' | 'down', selectedIds?: string[]) => void {
  let last = 0
  let pending: { pointer: CollaboratorPointer; button: 'up' | 'down'; selectedIds?: string[] } | null = null
  let timer: number | null = null

  const flush = () => {
    if (!pending) return
    const payload = pending
    pending = null
    timer = null
    last = performance.now()
    fn(payload.pointer, payload.button, payload.selectedIds)
  }

  return (pointer, button, selectedIds) => {
    pending = { pointer, button, selectedIds }
    const now = performance.now()
    const elapsed = now - last

    if (elapsed >= POINTER_SYNC_MS) {
      if (timer != null) {
        window.clearTimeout(timer)
        timer = null
      }
      flush()
      return
    }

    if (timer == null) {
      timer = window.setTimeout(flush, POINTER_SYNC_MS - elapsed)
    }
  }
}

function toCollaboratorSelection(ids?: string[]): Record<string, boolean> | undefined {
  if (!ids?.length) return undefined
  return Object.fromEntries(ids.map((id) => [id, true]))
}

export function useWhiteboardPointerSync(apiRef: RefObject<ExcalidrawImperativeAPI | null>, enabled: boolean) {
  const room = useRoomContext()
  const collaboratorsRef = useRef(new Map<SocketId, Collaborator>())
  const publishRef = useRef<(pointer: CollaboratorPointer, button: 'up' | 'down', selectedIds?: string[]) => void>(
    () => {},
  )
  const teardownRef = useRef<(() => void) | null>(null)
  const lastPointerRef = useRef<CollaboratorPointer>({ x: 0, y: 0, tool: 'pointer', renderCursor: false })
  const lastSelectionKeyRef = useRef('')

  const pushCollaborators = useCallback(() => {
    const api = apiRef.current
    if (!api) return

    const collaborators = new Map<SocketId, Collaborator>()
    for (const [socketId, collaborator] of collaboratorsRef.current) {
      if (collaborator.isCurrentUser) {
        collaborators.set(socketId, {
          username: collaborator.username,
          isCurrentUser: true,
        })
        continue
      }
      collaborators.set(socketId, collaborator)
    }

    api.updateScene({ collaborators })
  }, [apiRef])

  const toSocketId = (identity: string) => identity as SocketId

  // biome-ignore lint/correctness/useExhaustiveDependencies: toSocketId is stable local function
  const setCollaborator = useCallback(
    (identity: string, updates: Partial<Collaborator>) => {
      const socketId = toSocketId(identity)
      const existing = collaboratorsRef.current.get(socketId)
      collaboratorsRef.current.set(socketId, {
        ...existing,
        ...updates,
        socketId,
        isCurrentUser: identity === room.localParticipant.identity,
      })
      pushCollaborators()
    },
    [pushCollaborators, room.localParticipant.identity],
  )

  // biome-ignore lint/correctness/useExhaustiveDependencies: toSocketId is stable local function
  const removeCollaborator = useCallback(
    (identity: string) => {
      if (!collaboratorsRef.current.delete(toSocketId(identity))) return
      pushCollaborators()
    },
    [pushCollaborators],
  )

  // biome-ignore lint/correctness/useExhaustiveDependencies: toSocketId is stable local function
  const syncParticipants = useCallback(() => {
    const localId = room.localParticipant.identity
    const localName = room.localParticipant.name || localId

    collaboratorsRef.current.set(toSocketId(localId), {
      socketId: toSocketId(localId),
      isCurrentUser: true,
      username: localName,
      color: avatarColor(localId),
    })

    for (const participant of room.remoteParticipants.values()) {
      const socketId = toSocketId(participant.identity)
      if (collaboratorsRef.current.has(socketId)) continue
      collaboratorsRef.current.set(socketId, {
        socketId,
        isCurrentUser: false,
        username: participant.name || participant.identity,
        color: avatarColor(participant.identity),
      })
    }

    pushCollaborators()
  }, [pushCollaborators, room.localParticipant.identity, room.localParticipant.name, room.remoteParticipants])

  useEffect(() => {
    const localId = room.localParticipant.identity
    const localName = room.localParticipant.name || localId

    const publishHidden = () => {
      if (!isRoomConnected(room)) return
      const packet = encodeWhiteboardPointerPacket({
        identity: localId,
        username: localName,
        pointer: { x: 0, y: 0, tool: 'pointer', renderCursor: false },
        button: 'up',
      })
      void room.localParticipant
        .publishData(packet, { reliable: false, topic: WHITEBOARD_POINTER_TOPIC })
        .catch(() => {})
    }

    if (!enabled) {
      const api = apiRef.current
      if (api) api.updateScene({ collaborators: new Map() })
      collaboratorsRef.current.clear()
      publishHidden()
      return
    }

    const publish = (pointer: CollaboratorPointer, button: 'up' | 'down', selectedIds?: string[]) => {
      if (!isRoomConnected(room)) return

      const packet = encodeWhiteboardPointerPacket({
        identity: localId,
        username: localName,
        color: avatarColor(localId),
        pointer: toSharedPointer(pointer),
        button,
        selectedElementIds: selectedIds,
      })

      void room.localParticipant
        .publishData(packet, { reliable: false, topic: WHITEBOARD_POINTER_TOPIC })
        .catch((err) => {
          if (!isPublishUnavailableError(err) && import.meta.env.DEV) {
            console.error('[useWhiteboardPointerSync] publish failed:', err)
          }
        })
    }

    publishRef.current = throttlePointerSync(publish)
    syncParticipants()

    teardownRef.current?.()
    const api = apiRef.current
    if (api) {
      const unsubChange = api.onChange((_elements, appState) => {
        const ids = selectedElementIds(appState)
        const key = ids.join('|')
        if (key === lastSelectionKeyRef.current) return
        lastSelectionKeyRef.current = key
        publish(lastPointerRef.current, 'up', ids)
      })
      teardownRef.current = unsubChange
    }

    const onData = (payload: Uint8Array, _participant: unknown, _kind: unknown, topic?: string) => {
      if (topic !== WHITEBOARD_POINTER_TOPIC) return

      const packet = decodeWhiteboardPointerPacket(payload)
      if (!packet || packet.identity === localId) return

      setCollaborator(packet.identity, {
        pointer: toCollaboratorPointer(packet.pointer),
        button: packet.button,
        username: packet.username,
        color: packet.color ?? avatarColor(packet.identity),
        selectedElementIds: toCollaboratorSelection(packet.selectedElementIds),
      })
    }

    const onParticipantConnected = () => syncParticipants()
    const onParticipantDisconnected = (participant: { identity: string }) => {
      removeCollaborator(participant.identity)
    }

    room.on(RoomEvent.DataReceived, onData)
    room.on(RoomEvent.ParticipantConnected, onParticipantConnected)
    room.on(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)

    return () => {
      teardownRef.current?.()
      teardownRef.current = null
      room.off(RoomEvent.DataReceived, onData)
      room.off(RoomEvent.ParticipantConnected, onParticipantConnected)
      room.off(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
      const api = apiRef.current
      if (api) api.updateScene({ collaborators: new Map() })
      collaboratorsRef.current.clear()
      lastSelectionKeyRef.current = ''
      publishHidden()
    }
  }, [apiRef, enabled, removeCollaborator, room, setCollaborator, syncParticipants])

  const onPointerUpdate = useCallback(
    (payload: { pointer: CollaboratorPointer; button: 'up' | 'down'; pointersMap: Gesture['pointers'] }) => {
      if (payload.pointersMap.size >= 2) return
      lastPointerRef.current = payload.pointer
      const api = apiRef.current
      const selected = api ? selectedElementIds(api.getAppState()) : undefined
      publishRef.current(payload.pointer, payload.button, selected)
    },
    [apiRef],
  )

  const notifyApiReady = useCallback(() => {
    syncParticipants()
  }, [syncParticipants])

  return { onPointerUpdate, notifyApiReady }
}
