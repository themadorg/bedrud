import { useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { useMeetingStage } from './MeetingStageContext'
import { stageDescription, stageSessionKey } from './stageWire'

const JOIN_NOTIFY_WINDOW_MS = 20_000

export function StageJoinNotifier() {
  const room = useRoomContext()
  const { stage, isOwner } = useMeetingStage()
  const joinedAtRef = useRef(0)
  const announcedRef = useRef<string | null>(null)

  useEffect(() => {
    if (room.state === ConnectionState.Connected) {
      joinedAtRef.current = Date.now()
      announcedRef.current = null
    }
  }, [room.state])

  useEffect(() => {
    if (!stage) {
      announcedRef.current = null
      return
    }
    if (isOwner) return

    const sinceJoin = Date.now() - joinedAtRef.current
    if (sinceJoin > JOIN_NOTIFY_WINDOW_MS) return

    const key = stageSessionKey(stage)
    if (announcedRef.current === key) return
    announcedRef.current = key

    toast.info(stageDescription(stage), { duration: 5000 })
  }, [stage, isOwner])

  return null
}
