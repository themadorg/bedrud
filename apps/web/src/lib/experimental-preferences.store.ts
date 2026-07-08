import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface ExperimentalPreferences {
  whiteboardEnabled: boolean
  youtubeEnabled: boolean
  /** One-time experimental whiteboard disclaimer (localStorage via persist). */
  whiteboardDisclaimerAcknowledged: boolean
}

interface ExperimentalPreferencesState extends ExperimentalPreferences {
  setWhiteboardEnabled: (enabled: boolean) => void
  setYoutubeEnabled: (enabled: boolean) => void
  acknowledgeWhiteboardDisclaimer: () => void
  merge: (partial: Partial<ExperimentalPreferences>) => void
}

export const useExperimentalPreferencesStore = create<ExperimentalPreferencesState>()(
  persist(
    (set) => ({
      whiteboardEnabled: false,
      youtubeEnabled: false,
      whiteboardDisclaimerAcknowledged: false,
      setWhiteboardEnabled: (whiteboardEnabled) => set({ whiteboardEnabled }),
      setYoutubeEnabled: (youtubeEnabled) => set({ youtubeEnabled }),
      acknowledgeWhiteboardDisclaimer: () => set({ whiteboardDisclaimerAcknowledged: true }),
      merge: (partial) =>
        set({
          ...(partial.whiteboardEnabled !== undefined && { whiteboardEnabled: partial.whiteboardEnabled }),
          ...(partial.youtubeEnabled !== undefined && { youtubeEnabled: partial.youtubeEnabled }),
          ...(partial.whiteboardDisclaimerAcknowledged !== undefined && {
            whiteboardDisclaimerAcknowledged: partial.whiteboardDisclaimerAcknowledged,
          }),
        }),
    }),
    { name: 'experimental-preferences' },
  ),
)
