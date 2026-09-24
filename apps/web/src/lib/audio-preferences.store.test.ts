import { describe, expect, it, vi } from 'vitest'

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

// Import after the stub so the persist middleware can reach localStorage.
const { useAudioPreferencesStore } = await import('./audio-preferences.store')

// Read on nothing stored, before any test mutates the store, so this stays the shipped default
// rather than whatever a previous test left behind.
const shippedDefaults = useAudioPreferencesStore.getState()

describe('audio-preferences.store', () => {
  it('ships with the muted-mic alert off', () => {
    expect(shippedDefaults.mutedBeepEnabled).toBe(false)
  })

  it('still ships the 3s reminder interval, so turning the alert on needs no second step', () => {
    expect(shippedDefaults.mutedBeepInterval).toBe(3000)
  })

  it('keeps the alert on once the user turns it on', () => {
    useAudioPreferencesStore.getState().setMutedBeepEnabled(true)
    expect(useAudioPreferencesStore.getState().mutedBeepEnabled).toBe(true)
    useAudioPreferencesStore.getState().setMutedBeepEnabled(false)
    expect(useAudioPreferencesStore.getState().mutedBeepEnabled).toBe(false)
  })

  it('accepts the alert from synced preferences', () => {
    useAudioPreferencesStore.getState().merge({ mutedBeepEnabled: true })
    expect(useAudioPreferencesStore.getState().mutedBeepEnabled).toBe(true)
  })
})
