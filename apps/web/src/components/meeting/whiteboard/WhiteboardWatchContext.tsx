import { useRoomContext } from '@livekit/components-react'
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type * as Y from 'yjs'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import { stageOwnerLabel } from '@/components/meeting/stage/stageWire'
import { createWhiteboardYDoc, LiveKitYjsProvider } from '@/components/meeting/whiteboard/livekitYjsProvider'
import { WhiteboardExperimentalGate } from '@/components/meeting/whiteboard/WhiteboardExperimentalGate'
import {
  type WhiteboardSession,
  WhiteboardWatchContext,
  type WhiteboardWatchContextValue,
  whiteboardSessionKey,
} from '@/components/meeting/whiteboard/whiteboard-watch-context'

export { useWhiteboardWatch } from '@/components/meeting/whiteboard/whiteboard-watch-context'
export type { WhiteboardSession, WhiteboardWatchContextValue }

export function WhiteboardWatchProvider({ children }: { children: ReactNode }) {
  const room = useRoomContext()
  const { stage, isOwner, claimStage, clearStage } = useMeetingStage()
  const [ydoc, setYdoc] = useState<Y.Doc | null>(null)
  const [pendingOpen, setPendingOpen] = useState(false)
  const [acceptedSessionKey, setAcceptedSessionKey] = useState<string | null>(null)
  const [declinedSessionKey, setDeclinedSessionKey] = useState<string | null>(null)
  const [pendingConfirm, setPendingConfirm] = useState(false)
  const providerRef = useRef<LiveKitYjsProvider | null>(null)

  const session = useMemo<WhiteboardSession | null>(() => {
    if (stage?.kind !== 'whiteboard') return null
    return {
      hostIdentity: stage.ownerIdentity,
      hostName: stage.ownerName,
      updatedAt: stage.updatedAt,
    }
  }, [stage])

  const whiteboardEnabled = useExperimentalPreferencesStore((s) => s.whiteboardEnabled)
  const disclaimerAcknowledged = useExperimentalPreferencesStore((s) => s.whiteboardDisclaimerAcknowledged)
  const sessionKey = session ? whiteboardSessionKey(session) : null
  const accepted = sessionKey != null && acceptedSessionKey === sessionKey
  const declined = sessionKey != null && declinedSessionKey === sessionKey
  const whiteboardVisible = session != null && ydoc != null && accepted

  const whiteboardOwner = stage?.kind === 'whiteboard' ? stage.ownerIdentity : null

  useEffect(() => {
    if (!whiteboardOwner) {
      providerRef.current?.destroy()
      providerRef.current = null
      setYdoc((prev) => {
        prev?.destroy()
        return null
      })
      setAcceptedSessionKey(null)
      setDeclinedSessionKey(null)
      setPendingConfirm(false)
      return
    }

    // Keep one Y.Doc per active whiteboard session. Do not recreate on stage.updatedAt
    // republishes — that caused canvas flicker while Yjs was re-syncing from scratch.
    if (providerRef.current) return

    const doc = createWhiteboardYDoc()
    const provider = new LiveKitYjsProvider(doc, room)
    providerRef.current = provider
    setYdoc(doc)

    return () => {
      provider.destroy()
      doc.destroy()
      if (providerRef.current === provider) providerRef.current = null
      setYdoc((prev) => (prev === doc ? null : prev))
    }
  }, [room, whiteboardOwner])

  useEffect(() => {
    if (pendingConfirm && sessionKey) {
      setAcceptedSessionKey(sessionKey)
      setDeclinedSessionKey(null)
      setPendingConfirm(false)
    }
  }, [pendingConfirm, sessionKey])

  // Skip the one-time disclaimer when already acknowledged, or when experimental is enabled in settings.
  useEffect(() => {
    if (pendingOpen && disclaimerAcknowledged) {
      const err = claimStage('whiteboard')
      if (err) return
      setPendingOpen(false)
      setPendingConfirm(true)
      return
    }

    if (sessionKey == null || accepted || declined || pendingConfirm) return
    if (disclaimerAcknowledged || whiteboardEnabled) {
      setAcceptedSessionKey(sessionKey)
      setDeclinedSessionKey(null)
    }
  }, [
    accepted,
    claimStage,
    declined,
    disclaimerAcknowledged,
    pendingConfirm,
    pendingOpen,
    sessionKey,
    whiteboardEnabled,
  ])

  const requestStartWhiteboard = useCallback((): string | null => {
    if (!whiteboardEnabled) {
      return 'Enable the whiteboard in Settings → Experimental'
    }
    if (stage && !isOwner) {
      return `${stageOwnerLabel(stage)} is already on stage`
    }
    setPendingOpen(true)
    return null
  }, [isOwner, stage, whiteboardEnabled])

  const confirmStartWhiteboard = useCallback((): string | null => {
    const err = claimStage('whiteboard')
    if (err) return err
    setPendingOpen(false)
    setPendingConfirm(true)
    return null
  }, [claimStage])

  const cancelStartWhiteboard = useCallback(() => {
    setPendingOpen(false)
  }, [])

  const acceptWhiteboard = useCallback(() => {
    if (!sessionKey) return
    setAcceptedSessionKey(sessionKey)
    setDeclinedSessionKey(null)
  }, [sessionKey])

  const declineWhiteboard = useCallback(() => {
    if (!sessionKey) return
    setDeclinedSessionKey(sessionKey)
  }, [sessionKey])

  const stopWhiteboard = useCallback(() => {
    if (stage?.kind !== 'whiteboard' || stage.ownerIdentity !== room.localParticipant.identity) return
    clearStage()
  }, [clearStage, room.localParticipant.identity, stage])

  const flushWhiteboardSync = useCallback(() => {
    providerRef.current?.flush()
  }, [])

  const isHost = stage?.kind === 'whiteboard' && isOwner

  const value = useMemo(
    () => ({
      session,
      isHost,
      ydoc,
      pendingOpen,
      whiteboardVisible,
      requestStartWhiteboard,
      confirmStartWhiteboard,
      cancelStartWhiteboard,
      acceptWhiteboard,
      declineWhiteboard,
      stopWhiteboard,
      flushWhiteboardSync,
    }),
    [
      session,
      isHost,
      ydoc,
      pendingOpen,
      whiteboardVisible,
      requestStartWhiteboard,
      confirmStartWhiteboard,
      cancelStartWhiteboard,
      acceptWhiteboard,
      declineWhiteboard,
      stopWhiteboard,
      flushWhiteboardSync,
    ],
  )

  const hostNeedsDisclaimer = pendingOpen
  const viewerNeedsDisclaimer = sessionKey != null && !accepted && !declined && !pendingConfirm && !whiteboardEnabled
  const showGate = !disclaimerAcknowledged && (hostNeedsDisclaimer || viewerNeedsDisclaimer)

  return (
    <WhiteboardWatchContext.Provider value={value}>
      {showGate && <WhiteboardExperimentalGate />}
      {children}
    </WhiteboardWatchContext.Provider>
  )
}
