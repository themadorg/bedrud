import { useLocalParticipant, useRoomContext } from '@livekit/components-react'
import { ParticipantEvent, RoomEvent, Track } from 'livekit-client'
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { isPublishUnavailableError, isRoomPublishReady, waitForRoomPublishReady } from '#/lib/livekit-publish'
import {
  encodeStageWire,
  type MeetingStage,
  parseStageWire,
  STAGE_DATA_TOPIC,
  type StageKind,
  type StageWire,
  stageOwnerLabel,
} from './stageWire'

type YoutubeStageMeta = {
  videoId: string
  playing: boolean
  currentTime: number
}

interface MeetingStageContextValue {
  stage: MeetingStage | null
  isOwner: boolean
  claimStage: (kind: StageKind, meta?: YoutubeStageMeta) => string | null
  clearStage: () => void
  updateYoutubeStage: (playing: boolean, currentTime: number) => void
  youtubeSyncNonce: number
}

const MeetingStageContext = createContext<MeetingStageContextValue | null>(null)

export function useMeetingStage(): MeetingStageContextValue {
  const ctx = useContext(MeetingStageContext)
  if (!ctx) throw new Error('useMeetingStage must be used inside MeetingStageProvider')
  return ctx
}

function buildStage(kind: StageKind, ownerIdentity: string, ownerName: string, meta?: YoutubeStageMeta): MeetingStage {
  const updatedAt = Date.now()
  if (kind === 'youtube') {
    return {
      kind: 'youtube',
      ownerIdentity,
      ownerName,
      videoId: meta?.videoId ?? '',
      playing: meta?.playing ?? false,
      currentTime: meta?.currentTime ?? 0,
      updatedAt,
    }
  }
  if (kind === 'whiteboard') {
    return { kind: 'whiteboard', ownerIdentity, ownerName, updatedAt }
  }
  return { kind: 'screenshare', ownerIdentity, ownerName, updatedAt }
}

export function MeetingStageProvider({ children }: { children: ReactNode }) {
  const room = useRoomContext()
  const { localParticipant } = useLocalParticipant()
  const [stage, setStage] = useState<MeetingStage | null>(null)
  const [youtubeSyncNonce, setYoutubeSyncNonce] = useState(0)
  const stageRef = useRef<MeetingStage | null>(null)
  const publishQueueRef = useRef<StageWire[]>([])
  const publishingRef = useRef(false)
  const drainRetryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    stageRef.current = stage
  }, [stage])

  const clearDrainRetry = useCallback(() => {
    if (drainRetryTimerRef.current != null) {
      window.clearTimeout(drainRetryTimerRef.current)
      drainRetryTimerRef.current = null
    }
  }, [])

  const drainPublishQueueRef = useRef<() => Promise<void>>(async () => {})

  const scheduleDrainRetry = useCallback((delayMs = 400) => {
    if (drainRetryTimerRef.current != null) return
    drainRetryTimerRef.current = window.setTimeout(() => {
      drainRetryTimerRef.current = null
      void drainPublishQueueRef.current()
    }, delayMs)
  }, [])

  const drainPublishQueue = useCallback(async () => {
    if (publishingRef.current || !isRoomPublishReady(room)) return
    publishingRef.current = true

    while (publishQueueRef.current.length > 0 && isRoomPublishReady(room)) {
      const payload = publishQueueRef.current.shift()
      if (!payload) break

      try {
        await room.localParticipant.publishData(encodeStageWire(payload), {
          reliable: true,
          topic: STAGE_DATA_TOPIC,
        })
      } catch (err) {
        if (isPublishUnavailableError(err)) {
          publishQueueRef.current.unshift(payload)
          break
        }
        if (import.meta.env.DEV) console.error('[MeetingStage] failed to publish stage packet:', err)
      }
    }

    publishingRef.current = false
    if (publishQueueRef.current.length > 0 && isRoomPublishReady(room)) {
      scheduleDrainRetry(1500)
    }
  }, [room, scheduleDrainRetry])

  drainPublishQueueRef.current = drainPublishQueue

  useEffect(() => () => clearDrainRetry(), [clearDrainRetry])

  const publish = useCallback(
    (payload: StageWire) => {
      publishQueueRef.current.push(payload)
      void drainPublishQueue()
    },
    [drainPublishQueue],
  )

  const stopLocalScreenShare = useCallback(() => {
    if (localParticipant?.isScreenShareEnabled) {
      void localParticipant.setScreenShareEnabled(false).catch(() => {})
    }
  }, [localParticipant])

  const clearStage = useCallback(() => {
    const active = stageRef.current
    if (!active || active.ownerIdentity !== room.localParticipant.identity) return
    const ts = Date.now()
    stageRef.current = null
    setStage(null)
    stopLocalScreenShare()
    publish({ type: 'stage_clear', ownerIdentity: room.localParticipant.identity, ts })
  }, [publish, room.localParticipant.identity, stopLocalScreenShare])

  const claimStage = useCallback(
    (kind: StageKind, meta?: YoutubeStageMeta): string | null => {
      const ownerIdentity = room.localParticipant.identity
      const ownerName = room.localParticipant.name || ownerIdentity
      const active = stageRef.current

      if (active && active.ownerIdentity !== ownerIdentity) {
        return `${stageOwnerLabel(active)} is already on stage`
      }

      if (kind === 'youtube' && !meta?.videoId) {
        return 'Enter a valid YouTube URL or video ID'
      }

      if (active?.kind !== kind) {
        stopLocalScreenShare()
      }

      const next = buildStage(kind, ownerIdentity, ownerName, meta)
      stageRef.current = next
      setStage(next)
      publish({ type: 'stage_set', stage: next })
      return null
    },
    [publish, room.localParticipant.identity, room.localParticipant.name, stopLocalScreenShare],
  )

  const updateYoutubeStage = useCallback(
    (playing: boolean, currentTime: number) => {
      const active = stageRef.current
      if (!active || active.kind !== 'youtube' || active.ownerIdentity !== room.localParticipant.identity) return
      const ts = Date.now()
      setStage((prev) => (prev?.kind === 'youtube' ? { ...prev, playing, currentTime, updatedAt: ts } : prev))
      publish({
        type: 'stage_youtube_sync',
        ownerIdentity: room.localParticipant.identity,
        playing,
        currentTime,
        ts,
      })
    },
    [publish, room.localParticipant.identity],
  )

  const applyYoutubeSync = useCallback((playing: boolean, currentTime: number) => {
    setYoutubeSyncNonce((n) => n + 1)
    setStage((prev) => (prev?.kind === 'youtube' ? { ...prev, playing, currentTime, updatedAt: Date.now() } : prev))
  }, [])

  const applyRemoteStage = useCallback(
    (next: MeetingStage | null) => {
      const localIdentity = room.localParticipant.identity
      const prev = stageRef.current

      if (next?.kind !== 'screenshare' && localParticipant?.isScreenShareEnabled) {
        stopLocalScreenShare()
      }

      if (!next) {
        stageRef.current = null
        setStage(null)
        return
      }

      if (prev && next.updatedAt < prev.updatedAt) return

      if (prev?.ownerIdentity === localIdentity && prev.kind !== next.kind && next.ownerIdentity !== localIdentity) {
        stopLocalScreenShare()
      }

      stageRef.current = next
      setStage(next)
    },
    [localParticipant, room.localParticipant.identity, stopLocalScreenShare],
  )

  useEffect(() => {
    const handler = (payload: Uint8Array, _participant: unknown, _kind: unknown, topic?: string) => {
      if (topic !== STAGE_DATA_TOPIC) return
      try {
        const wire = parseStageWire(JSON.parse(new TextDecoder().decode(payload)))
        if (!wire) return

        if (wire.type === 'stage_set') {
          applyRemoteStage(wire.stage)
          return
        }

        if (wire.type === 'stage_clear') {
          const active = stageRef.current
          if (!active || wire.ownerIdentity !== active.ownerIdentity) return
          applyRemoteStage(null)
          return
        }

        if (wire.type === 'stage_state') {
          applyRemoteStage(wire.stage)
          return
        }

        if (wire.type === 'stage_youtube_sync') {
          if (wire.ownerIdentity === room.localParticipant.identity) return
          applyYoutubeSync(wire.playing, wire.currentTime)
          return
        }

        if (wire.type === 'stage_request') {
          const active = stageRef.current
          if (!active || active.ownerIdentity !== room.localParticipant.identity) return
          publish({ type: 'stage_state', stage: active, ts: Date.now() })
        }
      } catch {
        // ignore malformed payloads
      }
    }

    room.on(RoomEvent.DataReceived, handler)
    return () => {
      room.off(RoomEvent.DataReceived, handler)
    }
  }, [applyRemoteStage, applyYoutubeSync, publish, room])

  const requestStageState = useCallback(() => {
    publish({ type: 'stage_request', ts: Date.now() })
  }, [publish])

  const pushStageToRoom = useCallback(() => {
    const active = stageRef.current
    if (!active || active.ownerIdentity !== room.localParticipant.identity) return
    publish({ type: 'stage_state', stage: active, ts: Date.now() })
  }, [publish, room.localParticipant.identity])

  useEffect(() => {
    const delays = [800, 2000, 4000]
    const timers = delays.map((ms) => window.setTimeout(requestStageState, ms))
    return () => {
      for (const timer of timers) window.clearTimeout(timer)
    }
  }, [requestStageState])

  useEffect(() => {
    const onParticipantJoined = () => {
      requestStageState()
      const active = stageRef.current
      if (!active || active.ownerIdentity !== room.localParticipant.identity) return
      const delays = [400, 1200, 2500]
      for (const ms of delays) {
        window.setTimeout(pushStageToRoom, ms)
      }
    }

    room.on(RoomEvent.ParticipantConnected, onParticipantJoined)
    return () => {
      room.off(RoomEvent.ParticipantConnected, onParticipantJoined)
    }
  }, [pushStageToRoom, requestStageState, room])

  useEffect(() => {
    const onDisconnected = (participant: { identity: string }) => {
      const active = stageRef.current
      if (active?.ownerIdentity === participant.identity) {
        stageRef.current = null
        setStage(null)
        if (participant.identity !== room.localParticipant.identity) {
          stopLocalScreenShare()
        }
      }
    }

    room.on(RoomEvent.ParticipantDisconnected, onDisconnected)
    return () => {
      room.off(RoomEvent.ParticipantDisconnected, onDisconnected)
    }
  }, [room, stopLocalScreenShare])

  const wasLocalScreenSharingRef = useRef(false)

  useEffect(() => {
    const sharing = localParticipant?.isScreenShareEnabled ?? false

    if (
      stage?.kind === 'screenshare' &&
      stage.ownerIdentity === room.localParticipant.identity &&
      wasLocalScreenSharingRef.current &&
      !sharing
    ) {
      clearStage()
    }

    wasLocalScreenSharingRef.current = sharing
  }, [stage, room.localParticipant.identity, localParticipant?.isScreenShareEnabled, clearStage])

  useEffect(() => {
    const onRoomReady = () => {
      void waitForRoomPublishReady(room).then((ready) => {
        if (!ready) return
        requestStageState()
        const active = stageRef.current
        if (active && active.ownerIdentity === room.localParticipant.identity) {
          publish({ type: 'stage_set', stage: active })
        }
        void drainPublishQueue()
      })
    }

    const onRoomNotReady = () => {
      publishingRef.current = false
      clearDrainRetry()
    }

    room.on(RoomEvent.Connected, onRoomReady)
    room.on(RoomEvent.Reconnected, onRoomReady)
    room.on(RoomEvent.Reconnecting, onRoomNotReady)
    room.on(RoomEvent.Disconnected, onRoomNotReady)
    return () => {
      room.off(RoomEvent.Connected, onRoomReady)
      room.off(RoomEvent.Reconnected, onRoomReady)
      room.off(RoomEvent.Reconnecting, onRoomNotReady)
      room.off(RoomEvent.Disconnected, onRoomNotReady)
    }
  }, [clearDrainRetry, drainPublishQueue, publish, requestStageState, room])

  useEffect(() => {
    if (stage?.kind === 'screenshare') return
    if (!localParticipant?.isScreenShareEnabled) return
    // Stage is null while the user is claiming screenshare — do not tear down the track mid-startup.
    if (!stage) return
    stopLocalScreenShare()
  }, [stage, localParticipant, stopLocalScreenShare])

  useEffect(() => {
    if (stage?.kind !== 'screenshare') return
    if (stage.ownerIdentity !== room.localParticipant.identity) return

    const republishStage = () => {
      const active = stageRef.current
      if (!active || active.kind !== 'screenshare') return
      publish({ type: 'stage_set', stage: { ...active, updatedAt: Date.now() } })
    }

    const onLocalTrackPublished = (publication: { source: Track.Source }) => {
      if (publication.source === Track.Source.ScreenShare) republishStage()
    }

    if (localParticipant?.getTrackPublication(Track.Source.ScreenShare)) {
      republishStage()
    }

    room.localParticipant.on(ParticipantEvent.LocalTrackPublished, onLocalTrackPublished)
    return () => {
      room.localParticipant.off(ParticipantEvent.LocalTrackPublished, onLocalTrackPublished)
    }
  }, [stage?.kind, stage?.ownerIdentity, room.localParticipant, localParticipant, publish])

  const isOwner = stage?.ownerIdentity === room.localParticipant.identity

  const value = useMemo(
    () => ({
      stage,
      isOwner,
      claimStage,
      clearStage,
      updateYoutubeStage,
      youtubeSyncNonce,
    }),
    [stage, isOwner, claimStage, clearStage, updateYoutubeStage, youtubeSyncNonce],
  )

  return <MeetingStageContext.Provider value={value}>{children}</MeetingStageContext.Provider>
}

export function useStageActive(kind: StageKind): boolean {
  const { stage } = useMeetingStage()
  return stage?.kind === kind
}
