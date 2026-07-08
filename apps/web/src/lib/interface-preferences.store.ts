import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface InterfacePreferences {
  showWelcomeScreen: boolean
}

interface InterfacePreferencesState extends InterfacePreferences {
  setShowWelcomeScreen: (enabled: boolean) => void
  merge: (partial: Partial<InterfacePreferences>) => void
}

export const useInterfacePreferencesStore = create<InterfacePreferencesState>()(
  persist(
    (set) => ({
      showWelcomeScreen: true,
      setShowWelcomeScreen: (showWelcomeScreen) => set({ showWelcomeScreen }),
      merge: (partial) =>
        set({
          ...(partial.showWelcomeScreen !== undefined && { showWelcomeScreen: partial.showWelcomeScreen }),
        }),
    }),
    { name: 'interface-preferences' },
  ),
)
