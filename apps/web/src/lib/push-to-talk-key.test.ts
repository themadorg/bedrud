import { describe, expect, it } from 'vitest'
import { DEFAULT_PUSH_TO_TALK_KEY, formatKeyboardCode, isModifierKey, normalizePushToTalkKey } from './push-to-talk-key'

describe('push-to-talk-key', () => {
  it('formats common key codes', () => {
    expect(formatKeyboardCode('Space')).toBe('Space')
    expect(formatKeyboardCode('KeyV')).toBe('V')
    expect(formatKeyboardCode('Digit1')).toBe('1')
    expect(formatKeyboardCode('Numpad5')).toBe('Num 5')
  })

  it('detects modifier keys', () => {
    expect(isModifierKey('ShiftLeft')).toBe(true)
    expect(isModifierKey('KeyA')).toBe(false)
  })

  it('normalizes missing keys to Space', () => {
    expect(normalizePushToTalkKey(undefined)).toBe(DEFAULT_PUSH_TO_TALK_KEY)
    expect(normalizePushToTalkKey('KeyB')).toBe('KeyB')
  })
})
