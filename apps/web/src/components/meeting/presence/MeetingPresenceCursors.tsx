import { useRoomContext } from '@livekit/components-react'
import { ConnectionState, RoomEvent } from 'livekit-client'
import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { avatarColor } from '@/components/meeting/chat/chatGrouping'
import { contentNormToGridLocal } from '@/components/meeting/meetingViewportTransform'
import { useMeetingViewportPan } from '@/components/meeting/MeetingViewportPan'
import { isPublishUnavailableError, isRoomConnected } from '#/lib/livekit-publish'
import { textDirectionFor } from '#/lib/text-direction'
import {
  clientToSurfaceNorm,
  decodeMeetingPointerPacket,
  encodeMeetingPointerPacket,
  getMeetGridSurface,
  MEETING_POINTER_TOPIC,
  type MeetingPointerPacket,
} from './meetingPointerWire'

const SYNC_MS = 40
const STALE_MS = 5_000

type RemoteCursor = {
  identity: string
  username: string
  x: number
  y: number
  visible: boolean
  updatedAt: number
}

/** Arrow tip at (0,0) — matches OS cursor hotspot and Excalidraw remote pointers. */
function PresenceMarker({ color, username }: { color: string; username: string }) {
  return (
    <div className="relative" style={{ filter: 'drop-shadow(0 1px 3px rgba(0,0,0,0.5))' }}>
      <svg width="11" height="14" viewBox="0 0 11 14" aria-hidden className="block overflow-visible">
        <path
          d="M0,0 L0,14 L4,9 L11,8 Z"
          fill="#fff"
          stroke="#fff"
          strokeWidth="6"
          strokeLinejoin="round"
        />
        <path
          d="M0,0 L0,14 L4,9 L11,8 Z"
          fill={color}
          stroke={color}
          strokeWidth="2"
          strokeLinejoin="round"
        />
      </svg>
      <span
        dir={textDirectionFor(username)}
        className="meet-rtl-text absolute left-[3px] top-[15px] whitespace-nowrap px-1.5 py-0.5 text-[10px] font-semibold leading-none text-[#1b1b1f]"
        style={{
          background: color,
          border: '1px solid rgba(255,255,255,0.9)',
        }}
      >
        {username}
      </span>
    </div>
  )
}

export function MeetingPresenceCursors() {
  const room = useRoomContext()
  const { transform, transformRef } = useMeetingViewportPan()
  const [cursors, setCursors] = useState<RemoteCursor[]>([])
  const [layoutEpoch, setLayoutEpoch] = useState(0)
  const [portalTarget, setPortalTarget] = useState<HTMLElement | null>(null)
  const cursorsRef = useRef(new Map<string, RemoteCursor>())
  const publishRef = useRef<(packet: MeetingPointerPacket) => void>(() => {})
  const localVisibleRef = useRef(false)
  const enabled = false

  useEffect(() => {
    if (!enabled) {
      setPortalTarget(null)
      return
    }
    setPortalTarget(document.getElementById('meet-presence-layer'))
  }, [enabled, layoutEpoch])

  useEffect(() => {
    if (!enabled) return
    const grid = document.getElementById('meet-grid')
    if (!grid) return

    const bump = () => setLayoutEpoch((n) => n + 1)
    const observer = new ResizeObserver(bump)
    observer.observe(grid)
    window.addEventListener('resize', bump)
    window.addEventListener('scroll', bump, true)

    return () => {
      observer.disconnect()
      window.removeEventListener('resize', bump)
      window.removeEventListener('scroll', bump, true)
    }
  }, [enabled])

  const upsertRemote = useCallback((packet: MeetingPointerPacket) => {
    const next: RemoteCursor = {
      identity: packet.identity,
      username: packet.username,
      x: packet.x,
      y: packet.y,
      visible: packet.visible,
      updatedAt: Date.now(),
    }
    cursorsRef.current.set(packet.identity, next)
    setCursors(Array.from(cursorsRef.current.values()).filter((c) => c.visible))
  }, [])

  const removeRemote = useCallback((identity: string) => {
    if (!cursorsRef.current.delete(identity)) return
    setCursors(Array.from(cursorsRef.current.values()).filter((c) => c.visible))
  }, [])

  useEffect(() => {
    if (!enabled) {
      cursorsRef.current.clear()
      setCursors([])
      return
    }

    const localId = room.localParticipant.identity
    const localName = room.localParticipant.name || localId
    let lastSent = 0
    let pending: MeetingPointerPacket | null = null
    let timer: ReturnType<typeof setTimeout> | null = null

    const gridSurface = () => getMeetGridSurface()

    const flush = () => {
      if (!pending || !isRoomConnected(room)) return
      const packet = pending
      pending = null
      timer = null
      lastSent = performance.now()
      void room.localParticipant
        .publishData(encodeMeetingPointerPacket(packet), { reliable: false, topic: MEETING_POINTER_TOPIC })
        .catch((err) => {
          if (!isPublishUnavailableError(err) && import.meta.env.DEV) {
            console.error('[MeetingPresenceCursors] publish failed:', err)
          }
        })
    }

    publishRef.current = (packet: MeetingPointerPacket) => {
      pending = packet
      const elapsed = performance.now() - lastSent
      if (elapsed >= SYNC_MS) {
        if (timer != null) {
          clearTimeout(timer)
          timer = null
        }
        flush()
        return
      }
      if (timer == null) {
        timer = setTimeout(flush, SYNC_MS - elapsed)
      }
    }

    const publishLocal = (clientX: number, clientY: number, visible: boolean) => {
      if (!visible) {
        publishRef.current({
          identity: localId,
          username: localName,
          x: 0,
          y: 0,
          visible: false,
        })
        return
      }

      const surface = gridSurface()
      if (!surface) return
      const norm = clientToSurfaceNorm(
        clientX,
        clientY,
        surface.rect,
        transformRef.current,
        surface.insets,
      )
      if (!norm) {
        if (localVisibleRef.current) {
          localVisibleRef.current = false
          publishRef.current({
            identity: localId,
            username: localName,
            x: 0,
            y: 0,
            visible: false,
          })
        }
        return
      }

      localVisibleRef.current = true
      publishRef.current({
        identity: localId,
        username: localName,
        x: norm.x,
        y: norm.y,
        visible: true,
      })
    }

    const hideLocal = () => {
      localVisibleRef.current = false
      if (timer != null) {
        clearTimeout(timer)
        timer = null
      }
      pending = null
      if (!isRoomConnected(room)) return
      void room.localParticipant
        .publishData(
          encodeMeetingPointerPacket({
            identity: localId,
            username: localName,
            x: 0,
            y: 0,
            visible: false,
          }),
          { reliable: false, topic: MEETING_POINTER_TOPIC },
        )
        .catch(() => {})
    }

    const onPointerMove = (e: PointerEvent) => {
      if (e.pointerType === 'touch') return
      publishLocal(e.clientX, e.clientY, true)
    }

    const onBlur = () => hideLocal()
    const onVisibility = () => {
      if (document.visibilityState !== 'visible') hideLocal()
    }

    const onData = (payload: Uint8Array, _participant: unknown, _kind: unknown, topic?: string) => {
      if (topic !== MEETING_POINTER_TOPIC) return
      const packet = decodeMeetingPointerPacket(payload)
      if (!packet || packet.identity === localId) return
      if (!packet.visible) {
        removeRemote(packet.identity)
        return
      }
      upsertRemote(packet)
    }

    const onParticipantDisconnected = (participant: { identity: string }) => {
      removeRemote(participant.identity)
    }

    const onRoomState = () => {
      if (room.state !== ConnectionState.Connected) hideLocal()
    }

    window.addEventListener('pointermove', onPointerMove, { passive: true })
    window.addEventListener('blur', onBlur)
    document.addEventListener('visibilitychange', onVisibility)
    room.on(RoomEvent.DataReceived, onData)
    room.on(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
    room.on(RoomEvent.Disconnected, onRoomState)
    room.on(RoomEvent.Reconnecting, onRoomState)

    const staleTimer = window.setInterval(() => {
      const now = Date.now()
      let changed = false
      for (const [id, cursor] of cursorsRef.current) {
        if (now - cursor.updatedAt > STALE_MS) {
          cursorsRef.current.delete(id)
          changed = true
        }
      }
      if (changed) {
        setCursors(Array.from(cursorsRef.current.values()).filter((c) => c.visible))
      }
    }, 1_000)

    return () => {
      hideLocal()
      if (timer != null) clearTimeout(timer)
      window.clearInterval(staleTimer)
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('blur', onBlur)
      document.removeEventListener('visibilitychange', onVisibility)
      room.off(RoomEvent.DataReceived, onData)
      room.off(RoomEvent.ParticipantDisconnected, onParticipantDisconnected)
      room.off(RoomEvent.Disconnected, onRoomState)
      room.off(RoomEvent.Reconnecting, onRoomState)
      cursorsRef.current.clear()
      setCursors([])
    }
  }, [enabled, removeRemote, room, transformRef, upsertRemote])

  if (!enabled || !portalTarget) return null

  const surface = getMeetGridSurface()
  if (!surface) return null

  // layoutEpoch forces recompute when the grid moves or resizes
  void layoutEpoch

  return createPortal(
    <>
      {cursors.map((cursor) => {
        if (cursor.identity === room.localParticipant.identity) return null
        const point = contentNormToGridLocal(
          { x: cursor.x, y: cursor.y },
          surface.rect,
          transform,
          surface.insets,
        )
        const color = avatarColor(cursor.identity)
        return (
          <div
            key={cursor.identity}
            className="pointer-events-none absolute will-change-[left,top]"
            style={{
              left: point.x,
              top: point.y,
            }}
          >
            <PresenceMarker color={color} username={cursor.username} />
          </div>
        )
      })}
    </>,
    portalTarget,
  )
}