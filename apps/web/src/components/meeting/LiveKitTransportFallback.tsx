import { useRoomContext } from '@livekit/components-react'
import type { RoomConnectOptions } from 'livekit-client'
import { ConnectionState, RoomEvent } from 'livekit-client'
import { useEffect, useRef } from 'react'
import { isLiveKitRelayConnectOptions, waitForRoomPublishReady } from '#/lib/livekit-publish'

const P2P_DATA_CHANNEL_TIMEOUT_MS = 12_000

/**
 * When P2P ICE connects but SCTP data channels never open, remount on TURN/TLS relay
 * so chat/audio share a working peer connection.
 */
export function LiveKitTransportFallback({
  connectOptions,
  onNeedRelay,
}: {
  connectOptions?: RoomConnectOptions
  onNeedRelay: () => void
}) {
  const room = useRoomContext()
  const escalatedRef = useRef(false)

  useEffect(() => {
    escalatedRef.current = false
  }, [connectOptions])

  useEffect(() => {
    if (isLiveKitRelayConnectOptions(connectOptions)) return

    const escalate = () => {
      if (escalatedRef.current) return
      escalatedRef.current = true
      console.warn('[livekit-transport] P2P ICE ok but data channels closed — switching to TURN/TLS relay for chat')
      onNeedRelay()
    }

    const checkDataChannels = () => {
      void waitForRoomPublishReady(room, P2P_DATA_CHANNEL_TIMEOUT_MS).then((ready) => {
        if (!ready && room.state === ConnectionState.Connected) {
          escalate()
        }
      })
    }

    const onConnected = () => {
      checkDataChannels()
    }

    room.on(RoomEvent.Connected, onConnected)
    if (room.state === ConnectionState.Connected) {
      checkDataChannels()
    }

    return () => {
      room.off(RoomEvent.Connected, onConnected)
    }
  }, [room, connectOptions, onNeedRelay])

  return null
}
