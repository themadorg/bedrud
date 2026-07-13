import { beforeEach, describe, expect, it, vi } from 'vitest'

const memory = new Map<string, string>()
vi.stubGlobal('localStorage', {
  getItem: (k: string) => memory.get(k) ?? null,
  setItem: (k: string, v: string) => {
    memory.set(k, v)
  },
  removeItem: (k: string) => {
    memory.delete(k)
  },
  clear: () => memory.clear(),
  key: () => null,
  length: 0,
})

// Import after stub so persist middleware can use localStorage
const { useExperimentalPreferencesStore } = await import('./experimental-preferences.store')

describe('experimental-preferences.store', () => {
  beforeEach(() => {
    memory.clear()
    useExperimentalPreferencesStore.setState({
      whiteboardEnabled: false,
      youtubeEnabled: false,
      webxdcEnabled: false,
      whiteboardDisclaimerAcknowledged: false,
    })
  })

  it('toggles webxdcEnabled', () => {
    useExperimentalPreferencesStore.getState().setWebxdcEnabled(true)
    expect(useExperimentalPreferencesStore.getState().webxdcEnabled).toBe(true)
    useExperimentalPreferencesStore.getState().setWebxdcEnabled(false)
    expect(useExperimentalPreferencesStore.getState().webxdcEnabled).toBe(false)
  })

  it('merges webxdcEnabled from server prefs', () => {
    useExperimentalPreferencesStore.getState().merge({ webxdcEnabled: true, youtubeEnabled: true })
    const s = useExperimentalPreferencesStore.getState()
    expect(s.webxdcEnabled).toBe(true)
    expect(s.youtubeEnabled).toBe(true)
    expect(s.whiteboardEnabled).toBe(false)
  })
})
