import { useRoomContext } from '@livekit/components-react'
import type { RoomConnectOptions } from 'livekit-client'
import { ConnectionState, RoomEvent } from 'livekit-client'
import { useEffect } from 'react'
import {
  installLiveKitPublisherPromiseFix,
  resetLiveKitPublisherPromise,
  waitForRoomPublishReady,
} from '#/lib/livekit-publish'
import { meetingDebugLog } from '#/lib/meeting-debug-log'

/**
 * Observe data-channel readiness only. Never remount or force TURN —
 * those caused connect/disconnect loops.
 */
export function LiveKitTransportFallback({
  connectOptions: _connectOptions,
  onNeedRemount: _onNeedRemount,
  onNeedRelay: _onNeedRelay,
}: {
  connectOptions?: RoomConnectOptions
  onNeedRemount: (reason: string) => void
  onNeedRelay: (reason: string) => void
}) {
  const room = useRoomContext()

  useEffect(() => {
    installLiveKitPublisherPromiseFix(room)

    const onConnected = () => {
      installLiveKitPublisherPromiseFix(room)
      resetLiveKitPublisherPromise(room)
      void waitForRoomPublishReady(room, 45_000).then((ready) => {
        resetLiveKitPublisherPromise(room)
        meetingDebugLog('transport.fallback_watch_ready', { ready })
      })
    }

    room.on(RoomEvent.Connected, onConnected)
    room.on(RoomEvent.Reconnected, onConnected)
    if (room.state === ConnectionState.Connected) onConnected()

    return () => {
      room.off(RoomEvent.Connected, onConnected)
      room.off(RoomEvent.Reconnected, onConnected)
    }
  }, [room])

  return null
}
