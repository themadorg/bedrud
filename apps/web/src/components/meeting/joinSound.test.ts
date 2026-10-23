import { describe, expect, it } from 'vitest'
import { chooseJoinSound, type JoinSoundInput, WELCOME_SOUND_COOLDOWN_MS, WELCOME_SOUND_LATE_MS } from './joinSound'

const NOW = 1_000_000

function input(overrides: Partial<JoinSoundInput> = {}): JoinSoundInput {
  return {
    mode: 'welcome',
    deafened: false,
    welcomeSoundsEnabled: true,
    welcomeSoundUrl: '/uploads/welcome-sounds/peer.mp3?v=1',
    lastPlayedAt: undefined,
    welcomePlaying: false,
    waitedMs: 200,
    now: NOW,
    ...overrides,
  }
}

describe('chooseJoinSound', () => {
  it('should play the welcome sound for a peer who has one', () => {
    expect(chooseJoinSound(input())).toBe('welcome')
  })

  it('should play nothing while the listener is deafened', () => {
    expect(chooseJoinSound(input({ deafened: true }))).toBe('none')
  })

  it('should play nothing when the listener turned join sounds off', () => {
    expect(chooseJoinSound(input({ mode: 'off' }))).toBe('none')
  })

  it('should play the chime when the listener chose chime only', () => {
    expect(chooseJoinSound(input({ mode: 'chime' }))).toBe('chime')
  })

  it('should play the chime while welcome sounds are off on the server', () => {
    expect(chooseJoinSound(input({ welcomeSoundsEnabled: false }))).toBe('chime')
  })

  it('should play the chime for a peer without a welcome sound', () => {
    expect(chooseJoinSound(input({ welcomeSoundUrl: undefined }))).toBe('chime')
  })

  it('should play the chime while another welcome sound is still playing', () => {
    expect(chooseJoinSound(input({ welcomePlaying: true }))).toBe('chime')
  })

  describe('late profile', () => {
    it('should still play the welcome sound right at the limit', () => {
      expect(chooseJoinSound(input({ waitedMs: WELCOME_SOUND_LATE_MS }))).toBe('welcome')
    })

    it('should fall back to the chime one millisecond after the limit', () => {
      expect(chooseJoinSound(input({ waitedMs: WELCOME_SOUND_LATE_MS + 1 }))).toBe('chime')
    })
  })

  describe('reconnect cooldown', () => {
    it('should play the chime when the same peer rejoins inside the cooldown', () => {
      const lastPlayedAt = NOW - WELCOME_SOUND_COOLDOWN_MS + 1
      expect(chooseJoinSound(input({ lastPlayedAt }))).toBe('chime')
    })

    it('should play the welcome sound again once the cooldown has passed', () => {
      const lastPlayedAt = NOW - WELCOME_SOUND_COOLDOWN_MS
      expect(chooseJoinSound(input({ lastPlayedAt }))).toBe('welcome')
    })
  })
})
