import { getVisibleSceneBounds, zoomToFitBounds } from '@excalidraw/excalidraw'
import type { ExcalidrawImperativeAPI, SocketId } from '@excalidraw/excalidraw/types'
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import { type RefObject, useCallback, useEffect, useRef } from 'react'
import { isPublishUnavailableError, isRoomConnected } from '#/lib/livekit-publish'
import {
  decodeWhiteboardFollowPacket,
  encodeWhiteboardFollowPacket,
  WHITEBOARD_FOLLOW_TOPIC,
} from '@/components/meeting/whiteboard/whiteboardFollowWire'

const VIEWPORT_SYNC_MS = 33

function throttleViewport(fn: () => void) {
  let last = 0
  let pending = false
  let timer: number | null = null

  const flush = () => {
    pending = false
    timer = null
    last = performance.now()
    fn()
  }

  return () => {
    pending = true
    const elapsed = performance.now() - last
    if (elapsed >= VIEWPORT_SYNC_MS) {
      if (timer != null) {
        window.clearTimeout(timer)
        timer = null
      }
      flush()
      return
    }
    if (timer == null) {
      timer = window.setTimeout(() => {
        if (pending) flush()
      }, VIEWPORT_SYNC_MS - elapsed)
    }
  }
}

function viewportKey(appState: ReturnType<ExcalidrawImperativeAPI['getAppState']>) {
  const bounds = getVisibleSceneBounds(appState)
  return `${bounds[0]}|${bounds[1]}|${bounds[2]}|${bounds[3]}|${appState.zoom.value}`
}

export function useWhiteboardFollowSync(apiRef: RefObject<ExcalidrawImperativeAPI | null>, enabled: boolean) {
  const room = useRoomContext()
  const teardownRef = useRef<(() => void) | null>(null)
  const relayRef = useRef<(force?: boolean) => void>(() => {})
  const applyingRemoteViewportRef = useRef(false)
  const lastRelayedViewportRef = useRef('')

  const publishFollowChange = useCallback(
    (packet: Parameters<typeof encodeWhiteboardFollowPacket>[0]) => {
      if (!isRoomConnected(room)) return
      void room.localParticipant
        .publishData(encodeWhiteboardFollowPacket(packet), { reliable: true, topic: WHITEBOARD_FOLLOW_TOPIC })
        .catch((err) => {
          if (!isPublishUnavailableError(err) && import.meta.env.DEV) {
            console.error('[useWhiteboardFollowSync] follow publish failed:', err)
          }
        })
    },
    [room],
  )

  const bindApi = useCallback(() => {
    teardownRef.current?.()
    teardownRef.current = null

    const api = apiRef.current
    if (!api || !enabled) return

    const localId = room.localParticipant.identity
    const localName = room.localParticipant.name || localId

    const relayViewport = (force = false) => {
      if (applyingRemoteViewportRef.current) return

      const appState = api.getAppState()
      if (!force && appState.followedBy.size === 0) return
      if (!isRoomConnected(room)) return

      const key = viewportKey(appState)
      if (!force && key === lastRelayedViewportRef.current) return
      lastRelayedViewportRef.current = key

      const packet = {
        type: 'viewport' as const,
        identity: localId,
        sceneBounds: getVisibleSceneBounds(appState),
      }

      void room.localParticipant
        .publishData(encodeWhiteboardFollowPacket(packet), { reliable: false, topic: WHITEBOARD_FOLLOW_TOPIC })
        .catch((err) => {
          if (!isPublishUnavailableError(err) && import.meta.env.DEV) {
            console.error('[useWhiteboardFollowSync] viewport publish failed:', err)
          }
        })
    }

    relayRef.current = relayViewport
    const throttledRelay = throttleViewport(() => relayViewport())

    const unsubFollow = api.onUserFollow((payload) => {
      publishFollowChange({
        type: 'follow-change',
        followerId: localId,
        followerName: localName,
        targetId: payload.userToFollow.socketId,
        action: payload.action,
      })
    })

    const unsubScroll = api.onScrollChange(() => {
      throttledRelay()
    })

    const unsubChange = api.onChange((_elements, appState) => {
      if (appState.followedBy.size === 0) return
      const key = viewportKey(appState)
      if (key === lastRelayedViewportRef.current) return
      throttledRelay()
    })

    teardownRef.current = () => {
      unsubFollow()
      unsubScroll()
      unsubChange()
      relayRef.current = () => {}
    }
  }, [apiRef, enabled, publishFollowChange, room])

  useEffect(() => {
    if (enabled && apiRef.current) bindApi()
  }, [apiRef, bindApi, enabled])

  useEffect(() => {
    if (!enabled) {
      teardownRef.current?.()
      teardownRef.current = null
      return
    }

    const localId = room.localParticipant.identity

    const onData = (payload: Uint8Array, _participant: unknown, _kind: unknown, topic?: string) => {
      if (topic !== WHITEBOARD_FOLLOW_TOPIC) return
      const api = apiRef.current
      if (!api) return

      const packet = decodeWhiteboardFollowPacket(payload)
      if (!packet) return

      if (packet.type === 'follow-change') {
        if (packet.targetId !== localId) return

        const appState = api.getAppState()
        const followedBy = new Set(appState.followedBy)
        const followerSocketId = packet.followerId as SocketId

        if (packet.action === 'FOLLOW') {
          followedBy.add(followerSocketId)
          api.updateScene({ appState: { followedBy } })
          relayRef.current(true)
          return
        }

        followedBy.delete(followerSocketId)
        api.updateScene({ appState: { followedBy } })
        return
      }

      if (packet.identity === localId) return

      const appState = api.getAppState()
      const leaderId = packet.identity as SocketId

      if (!appState.userToFollow || appState.userToFollow.socketId !== leaderId) return
      if (appState.followedBy.has(leaderId)) return

      applyingRemoteViewportRef.current = true
      const next = zoomToFitBounds({
        bounds: packet.sceneBounds,
        appState,
        fitToViewport: true,
        viewportZoomFactor: 1,
      })
      lastRelayedViewportRef.current = viewportKey(next.appState)
      api.updateScene({ appState: next.appState })
      applyingRemoteViewportRef.current = false
    }

    const onParticipantDisconnected = (participant: { identity: string }) => {
      const api = apiRef.current
      if (!api) return
      const appState = api.getAppState()
      const followerSocketId = participant.identity as SocketId
      const updates: Partial<typeof appState> = {}
      if (appState.followedBy.has(followerSocketId)) {
        const followedBy = new Set(appState.followedBy)
        followedBy.delete(followerSocketId)
        updates.followedBy = followedBy
      }
      if (appState.userToFollow?.socketId === followerSocketId) {
        updates.userToFollow = null
      }
      if (Object.keys(updates).length > 0) {
        api.updateScene({ appState: updates })
      }
    }

    room.on(RoomEvent.DataReceived, onData)
    room.on(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)

    return () => {
      room.off(RoomEvent.DataReceived, onData)
      room.off(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
      teardownRef.current?.()
      teardownRef.current = null
    }
  }, [apiRef, enabled, room])

  const notifyApiReady = useCallback(() => {
    bindApi()
  }, [bindApi])

  const relayViewport = useCallback((force = false) => {
    relayRef.current(force)
  }, [])

  return { notifyApiReady, relayViewport }
}
