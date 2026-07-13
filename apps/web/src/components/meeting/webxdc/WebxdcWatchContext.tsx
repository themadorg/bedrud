import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { useOptionalMeetingStage } from '@/components/meeting/stage/MeetingStageContext'
import {
  closeWebxdcInstance,
  fetchWebxdcConfig,
  mintWebxdcTicket,
  startWebxdcInstance,
  uploadWebxdcPackage,
} from './webxdcApi'
import {
  startResponseToSession,
  ticketResponseToSession,
  WebxdcWatchContext,
  type WebxdcSession,
} from './webxdc-watch-context'

export { useOptionalWebxdcWatch, useWebxdcWatch } from './webxdc-watch-context'

/**
 * Must sit under MeetingProvider + MeetingStageProvider (see m.$meetId.tsx).
 * Uses optional stage so a mis-nested tree does not hard-crash the meeting.
 */
export function WebxdcWatchProvider({ children }: { children: ReactNode }) {
  const { roomId } = useMeetingRoomContext()
  const stageApi = useOptionalMeetingStage()
  const webxdcUserEnabled = useExperimentalPreferencesStore((s) => s.webxdcEnabled)

  const [session, setSession] = useState<WebxdcSession | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [serverEnabled, setServerEnabled] = useState(false)
  const joinedInstanceRef = useRef<string | null>(null)

  const stage = stageApi?.stage ?? null
  const isOwner = stageApi?.isOwner ?? false
  const claimStage = stageApi?.claimStage
  const clearStage = stageApi?.clearStage

  useEffect(() => {
    fetchWebxdcConfig()
      .then((c) => setServerEnabled(c.enabled === true))
      .catch(() => setServerEnabled(false))
  }, [])

  // Host/upload/start: needs server + user experimental toggle.
  const webxdcReady = webxdcUserEnabled && serverEnabled
  // Viewing a stage share: server only (guests should see shared apps without the toggle).
  const canViewShared = serverEnabled

  // When someone puts a webxdc on stage, mint a ticket and open the iframe for all peers.
  useEffect(() => {
    if (!canViewShared || !roomId || !stageApi) {
      if (stage?.kind !== 'webxdc') {
        setSession(null)
        joinedInstanceRef.current = null
      }
      return
    }
    if (stage?.kind !== 'webxdc') {
      setSession(null)
      joinedInstanceRef.current = null
      return
    }

    const instanceId = stage.instanceId
    if (!instanceId) return
    if (joinedInstanceRef.current === instanceId && session?.instanceId === instanceId) return

    let cancelled = false
    joinedInstanceRef.current = instanceId
    setBusy(true)
    setError(null)

    mintWebxdcTicket(roomId, instanceId)
      .then((t) => {
        if (cancelled) return
        setSession(
          ticketResponseToSession(t, {
            instanceId,
            packageId: stage.packageId,
            name: stage.name || 'WebXDC',
          }),
        )
      })
      .catch((e) => {
        if (cancelled) return
        joinedInstanceRef.current = null
        setSession(null)
        setError(e instanceof Error ? e.message : String(e))
        toast.error('Could not open shared mini-app', {
          description: e instanceof Error ? e.message : 'Ticket mint failed',
        })
      })
      .finally(() => {
        if (!cancelled) setBusy(false)
      })

    return () => {
      cancelled = true
    }
  }, [canViewShared, stage, roomId, session?.instanceId, stageApi])

  const sharePackage = useCallback(
    async (packageId: string, name?: string) => {
      if (!webxdcReady) {
        toast.error('WebXDC is not enabled', {
          description: 'Enable server webxdc + Settings → Experimental → WebXDC mini-apps',
        })
        return
      }
      if (!claimStage) {
        toast.error('Stage is not ready', { description: 'Reload the meeting and try again.' })
        return
      }
      setBusy(true)
      setError(null)
      try {
        const res = await startWebxdcInstance(roomId, packageId)
        const next = startResponseToSession(res)
        setSession(next)
        joinedInstanceRef.current = next.instanceId
        const err = claimStage('webxdc', {
          instanceId: next.instanceId,
          packageId: next.packageId,
          name: name || next.name,
        })
        if (err) {
          toast.error('Could not share on stage', { description: err })
        } else {
          toast.success('Mini-app shared on stage')
        }
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e)
        setError(msg)
        toast.error('Failed to start mini-app', { description: msg })
      } finally {
        setBusy(false)
      }
    },
    [webxdcReady, roomId, claimStage],
  )

  const shareFile = useCallback(
    async (file: File) => {
      if (!webxdcReady) {
        toast.error('WebXDC is not enabled', {
          description: 'Enable server webxdc + Settings → Experimental → WebXDC mini-apps',
        })
        return
      }
      setBusy(true)
      setError(null)
      try {
        const pkg = await uploadWebxdcPackage(roomId, file)
        setBusy(false)
        await sharePackage(pkg.id, pkg.name)
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e)
        setError(msg)
        toast.error('Failed to upload mini-app', { description: msg })
        setBusy(false)
      }
    },
    [webxdcReady, roomId, sharePackage],
  )

  const stopShare = useCallback(async () => {
    const inst = session?.instanceId
    if (isOwner && stage?.kind === 'webxdc' && clearStage) {
      clearStage()
    }
    if (inst && isOwner) {
      try {
        await closeWebxdcInstance(roomId, inst)
      } catch {
        /* ignore */
      }
    }
    setSession(null)
    joinedInstanceRef.current = null
  }, [session?.instanceId, isOwner, stage?.kind, clearStage, roomId])

  const leaveLocal = useCallback(() => {
    setSession(null)
    joinedInstanceRef.current = null
  }, [])

  const value = useMemo(
    () => ({
      session,
      isHost: isOwner && stage?.kind === 'webxdc',
      busy,
      error,
      shareFile,
      sharePackage,
      stopShare,
      leaveLocal,
    }),
    [session, isOwner, stage?.kind, busy, error, shareFile, sharePackage, stopShare, leaveLocal],
  )

  return <WebxdcWatchContext.Provider value={value}>{children}</WebxdcWatchContext.Provider>
}
