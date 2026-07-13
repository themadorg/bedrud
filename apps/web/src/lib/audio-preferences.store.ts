import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { DEFAULT_PUSH_TO_TALK_KEY, normalizePushToTalkKey } from '#/lib/push-to-talk-key'

export type NoiseSuppressionMode = 'none' | 'browser' | 'rnnoise' | 'krisp'

export interface AudioPreferences {
  noiseSuppressionMode: NoiseSuppressionMode
  echoCancellation: boolean
  autoGainControl: boolean
  inputGain: number // 0–300 (percent), default 100 = unity
  noiseGate: number // 0–100 (percent), default 0 = off
  mutedBeepEnabled: boolean // play a beep when talking while muted
  mutedBeepInterval: number // ms between beeps, default 3000
  pushToTalkEnabled: boolean
  pushToTalkKey: string // KeyboardEvent.code, default Space
  /**
   * User confirmed they reviewed Krisp licensing before enabling Krisp.
   * Krisp stays off until this is set via enableKrispAfterLicenseAck().
   */
  krispLicenseAcknowledged: boolean
}

interface AudioPreferencesStore extends AudioPreferences {
  setMode: (mode: NoiseSuppressionMode) => void
  /** After the license dialog — only path that may set mode to krisp. */
  enableKrispAfterLicenseAck: () => void
  setEchoCancellation: (v: boolean) => void
  setAutoGainControl: (v: boolean) => void
  setInputGain: (v: number) => void
  setNoiseGate: (v: number) => void
  setMutedBeepEnabled: (v: boolean) => void
  setMutedBeepInterval: (v: number) => void
  setPushToTalkEnabled: (v: boolean) => void
  setPushToTalkKey: (key: string) => void
  merge: (partial: Partial<AudioPreferences>) => void
}

/** Krisp must never become active without an explicit local license acknowledgment. */
function sanitizeMode(
  mode: NoiseSuppressionMode | undefined,
  krispLicenseAcknowledged: boolean,
): NoiseSuppressionMode | undefined {
  if (mode === undefined) return undefined
  if (mode === 'krisp' && !krispLicenseAcknowledged) return 'browser'
  return mode
}

export const useAudioPreferencesStore = create<AudioPreferencesStore>()(
  persist(
    (set) => ({
      noiseSuppressionMode: 'browser',
      echoCancellation: true,
      autoGainControl: true,
      inputGain: 100,
      noiseGate: 0,
      mutedBeepEnabled: true,
      mutedBeepInterval: 3000,
      pushToTalkEnabled: false,
      pushToTalkKey: DEFAULT_PUSH_TO_TALK_KEY,
      krispLicenseAcknowledged: false,
      setMode: (noiseSuppressionMode) =>
        set((s) => {
          // Block silent krisp enable — use enableKrispAfterLicenseAck after the dialog.
          if (noiseSuppressionMode === 'krisp' && !s.krispLicenseAcknowledged) {
            return {}
          }
          return { noiseSuppressionMode }
        }),
      enableKrispAfterLicenseAck: () => set({ krispLicenseAcknowledged: true, noiseSuppressionMode: 'krisp' }),
      setEchoCancellation: (echoCancellation) => set({ echoCancellation }),
      setAutoGainControl: (autoGainControl) => set({ autoGainControl }),
      setInputGain: (inputGain) => set({ inputGain: Math.max(0, Math.min(300, inputGain)) }),
      setNoiseGate: (noiseGate) => set({ noiseGate: Math.max(0, Math.min(100, noiseGate)) }),
      setMutedBeepEnabled: (mutedBeepEnabled) => set({ mutedBeepEnabled }),
      setMutedBeepInterval: (mutedBeepInterval) => set({ mutedBeepInterval }),
      setPushToTalkEnabled: (pushToTalkEnabled) => set({ pushToTalkEnabled }),
      setPushToTalkKey: (pushToTalkKey) => set({ pushToTalkKey: normalizePushToTalkKey(pushToTalkKey) }),
      merge: (partial) =>
        set((s) => {
          const ack =
            partial.krispLicenseAcknowledged !== undefined
              ? partial.krispLicenseAcknowledged
              : s.krispLicenseAcknowledged
          const mode = sanitizeMode(
            partial.noiseSuppressionMode !== undefined ? partial.noiseSuppressionMode : undefined,
            ack,
          )
          return {
            ...(mode !== undefined && { noiseSuppressionMode: mode }),
            ...(partial.echoCancellation !== undefined && { echoCancellation: partial.echoCancellation }),
            ...(partial.autoGainControl !== undefined && { autoGainControl: partial.autoGainControl }),
            ...(partial.inputGain !== undefined && { inputGain: Math.max(0, Math.min(300, partial.inputGain)) }),
            ...(partial.noiseGate !== undefined && { noiseGate: Math.max(0, Math.min(100, partial.noiseGate)) }),
            ...(partial.mutedBeepEnabled !== undefined && { mutedBeepEnabled: partial.mutedBeepEnabled }),
            ...(partial.mutedBeepInterval !== undefined && { mutedBeepInterval: partial.mutedBeepInterval }),
            ...(partial.pushToTalkEnabled !== undefined && { pushToTalkEnabled: partial.pushToTalkEnabled }),
            ...(partial.pushToTalkKey !== undefined && {
              pushToTalkKey: normalizePushToTalkKey(partial.pushToTalkKey),
            }),
            ...(partial.krispLicenseAcknowledged !== undefined && {
              krispLicenseAcknowledged: partial.krispLicenseAcknowledged,
            }),
          }
        }),
    }),
    {
      name: 'audio-preferences',
      // Clear stale krisp-on prefs from before the license gate existed.
      onRehydrateStorage: () => (state) => {
        if (!state) return
        if (state.noiseSuppressionMode === 'krisp' && !state.krispLicenseAcknowledged) {
          state.noiseSuppressionMode = 'browser'
        }
      },
    },
  ),
)
