import { useCallback } from 'react'
import { type NoiseSuppressionMode, useAudioPreferencesStore } from '#/lib/audio-preferences.store'

/**
 * Request a noise-suppression mode.
 * RNNoise / Krisp only when instance admin enabled them (public settings).
 * Heavy SDKs load only after a successful mode switch in AudioProcessorService.
 */
export function useRequestNoiseMode(opts?: { rnnoiseAllowed?: boolean; krispAllowed?: boolean }) {
  const setMode = useAudioPreferencesStore((s) => s.setMode)
  const enableKrispAfterLicenseAck = useAudioPreferencesStore((s) => s.enableKrispAfterLicenseAck)
  const rnnoiseAllowed = opts?.rnnoiseAllowed ?? false
  const krispAllowed = opts?.krispAllowed ?? false

  const requestMode = useCallback(
    (mode: NoiseSuppressionMode) => {
      if (mode === 'rnnoise') {
        if (!rnnoiseAllowed) return
        setMode('rnnoise')
        return
      }
      if (mode === 'krisp') {
        if (!krispAllowed) return
        // Local store flag so setMode('krisp') is accepted; admin already licensed.
        enableKrispAfterLicenseAck()
        return
      }
      setMode(mode)
    },
    [setMode, enableKrispAfterLicenseAck, rnnoiseAllowed, krispAllowed],
  )

  return { requestMode }
}
