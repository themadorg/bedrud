import { useRoomContext } from '@livekit/components-react'
import { ConnectionState } from 'livekit-client'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { useMeetingChatContext } from '@/components/meeting/MeetingContext'
import { useMeetingStage } from './MeetingStageContext'
import { stageDescription, stageSessionKey, stageShareKey } from './stageWire'

const JOIN_NOTIFY_WINDOW_MS = 20_000

/**
 * Side effects for stage changes:
 * - Chat system/event line for every new stage session (not a bubble) — host + peers
 * - Toast for late joiners who arrive while something is already on stage
 */
export function StageJoinNotifier() {
  const room = useRoomContext()
  const { stage, isOwner } = useMeetingStage()
  const { appendSystemMessage } = useMeetingChatContext()
  const joinedAtRef = useRef(0)
  const toastAnnouncedRef = useRef<string | null>(null)
  const chatAnnouncedRef = useRef<string | null>(null)

  useEffect(() => {
    if (room.state === ConnectionState.Connected) {
      joinedAtRef.current = Date.now()
      toastAnnouncedRef.current = null
      // Keep chatAnnouncedRef across reconnect so we don't re-spam the same stage line.
    }
  }, [room.state])

  // Chat event line (centered pill) when a new share starts — not on playhead/rebroadcast.
  useEffect(() => {
    if (!stage) {
      chatAnnouncedRef.current = null
      return
    }
    const key = stageShareKey(stage)
    if (chatAnnouncedRef.current === key) return
    chatAnnouncedRef.current = key

    appendSystemMessage({
      event: 'stage',
      actor: stage.ownerName || stage.ownerIdentity,
      message: stageDescription(stage),
    })
  }, [stage, appendSystemMessage])

  // Toast only for non-owners shortly after join (existing UX).
  useEffect(() => {
    if (!stage) {
      toastAnnouncedRef.current = null
      return
    }
    if (isOwner) return

    const sinceJoin = Date.now() - joinedAtRef.current
    if (sinceJoin > JOIN_NOTIFY_WINDOW_MS) return

    const key = stageSessionKey(stage)
    if (toastAnnouncedRef.current === key) return
    toastAnnouncedRef.current = key

    toast.info(stageDescription(stage), { duration: 5000 })
  }, [stage, isOwner])

  return null
}
