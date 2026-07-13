import { useConnectionState, useLocalParticipant } from '@livekit/components-react'
import type { LocalAudioTrack } from 'livekit-client'
import { ConnectionState, Track } from 'livekit-client'
import { useEffect, useState } from 'react'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { audioProcessorService } from '#/lib/audio-processor.service'
import { getPublicSettings, refreshPublicSettings } from '#/lib/use-public-settings'

/**
 * Null-rendering component that manages the AudioProcessor lifecycle inside
 * the meeting room. Must be placed inside <LiveKitRoom> so hooks work.
 *
 * Never loads RNNoise/Krisp SDKs unless admin enabled them for the instance.
 */
export function AudioProcessorManager() {
  const { localParticipant } = useLocalParticipant()
  const connectionState = useConnectionState()
  const mode = useAudioPreferencesStore((s) => s.noiseSuppressionMode)
  const krispLicenseAcknowledged = useAudioPreferencesStore((s) => s.krispLicenseAcknowledged)
  const echoCancellation = useAudioPreferencesStore((s) => s.echoCancellation)
  const autoGainControl = useAudioPreferencesStore((s) => s.autoGainControl)
  const [rnnoiseAllowed, setRnnoiseAllowed] = useState(false)
  const [krispAllowed, setKrispAllowed] = useState(false)

  useEffect(() => {
    refreshPublicSettings()
    void getPublicSettings().then((s) => {
      const rn = !!s.rnnoiseEnabled
      const kr = !!s.krispEnabled
      setRnnoiseAllowed(rn)
      setKrispAllowed(kr)
      audioProcessorService.setNoisePackageAllowed({ rnnoise: rn, krisp: kr })
    })
  }, [])

  let effectiveMode = mode
  if (mode === 'rnnoise' && !rnnoiseAllowed) effectiveMode = 'browser'
  if (mode === 'krisp' && (!krispAllowed || !krispLicenseAcknowledged)) effectiveMode = 'browser'

  // biome-ignore lint/correctness/useExhaustiveDependencies: only attach once on connect; mode changes are handled by the next effect
  useEffect(() => {
    if (connectionState !== ConnectionState.Connected) return
    const pub = localParticipant?.getTrackPublication(Track.Source.Microphone)
    const track = pub?.track
    if (track) {
      audioProcessorService.attach(track as LocalAudioTrack, effectiveMode)
    }
  }, [connectionState, localParticipant])

  useEffect(() => {
    if (connectionState !== ConnectionState.Connected) return
    audioProcessorService.switchMode(effectiveMode, { echoCancellation, autoGainControl })
  }, [effectiveMode, connectionState, echoCancellation, autoGainControl])

  useEffect(() => {
    if (connectionState !== ConnectionState.Connected) return
    audioProcessorService.setEchoCancellation(echoCancellation)
  }, [echoCancellation, connectionState])

  useEffect(
    () => () => {
      audioProcessorService.detach()
    },
    [],
  )

  return null
}
