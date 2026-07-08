import { describe, expect, it } from 'vitest'
import { isSingleEmojiMessage, isSingleEmojiText } from './chatEmojiMessage'

describe('isSingleEmojiText', () => {
  it('accepts common lone emojis', () => {
    expect(isSingleEmojiText('👍')).toBe(true)
    expect(isSingleEmojiText('❤️')).toBe(true)
    expect(isSingleEmojiText('😂')).toBe(true)
    expect(isSingleEmojiText('🇺🇸')).toBe(true)
    expect(isSingleEmojiText('👨‍👩‍👧')).toBe(true)
    expect(isSingleEmojiText('1️⃣')).toBe(true)
    expect(isSingleEmojiText('  🎉  ')).toBe(true)
  })

  it('rejects text and multi-emoji messages', () => {
    expect(isSingleEmojiText('')).toBe(false)
    expect(isSingleEmojiText('hello')).toBe(false)
    expect(isSingleEmojiText('👍👍')).toBe(false)
    expect(isSingleEmojiText('👍 👍')).toBe(false)
    expect(isSingleEmojiText('👍!')).toBe(false)
    expect(isSingleEmojiText('a')).toBe(false)
  })
})

describe('isSingleEmojiMessage', () => {
  it('requires text-only single emoji', () => {
    expect(isSingleEmojiMessage({ message: '🔥', attachments: [] })).toBe(true)
    expect(isSingleEmojiMessage({ message: '🔥', attachments: [{ kind: 'image' }], poll: undefined })).toBe(false)
    expect(isSingleEmojiMessage({ message: '🔥', attachments: [], poll: { id: 'p1' } })).toBe(false)
    expect(isSingleEmojiMessage({ message: 'hi', attachments: [] })).toBe(false)
  })
})
