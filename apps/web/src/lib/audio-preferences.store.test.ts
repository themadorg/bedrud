import { beforeEach, describe, expect, it, vi } from 'vitest'

const memory = new Map<string, string>()
vi.stubGlobal('localStorage', {
  getItem: (key: string) => memory.get(key) ?? null,
  setItem: (key: string, value: string) => {
    memory.set(key, value)
  },
  removeItem: (key: string) => {
    memory.delete(key)
  },
  clear: () => memory.clear(),
  key: () => null,
  length: 0,
})

// Import after the stub so the persist middleware can use localStorage.
const { useAudioPreferencesStore } = await import('./audio-preferences.store')

describe('audio-preferences.store joinSoundMode', () => {
  beforeEach(() => {
    memory.clear()
    useAudioPreferencesStore.setState({ joinSoundMode: 'welcome' })
  })

  it('should default to playing welcome sounds', () => {
    expect(useAudioPreferencesStore.getState().joinSoundMode).toBe('welcome')
  })

  it('should take a valid mode from the synced preferences', () => {
    useAudioPreferencesStore.getState().merge({ joinSoundMode: 'off' })
    expect(useAudioPreferencesStore.getState().joinSoundMode).toBe('off')
  })

  it('should ignore an unknown mode from the synced preferences', () => {
    useAudioPreferencesStore.getState().merge({ joinSoundMode: 'loud' as never })
    expect(useAudioPreferencesStore.getState().joinSoundMode).toBe('welcome')
  })

  it('should keep the mode when the synced preferences leave it out', () => {
    useAudioPreferencesStore.getState().setJoinSoundMode('chime')
    useAudioPreferencesStore.getState().merge({ echoCancellation: false })
    expect(useAudioPreferencesStore.getState().joinSoundMode).toBe('chime')
  })
})
