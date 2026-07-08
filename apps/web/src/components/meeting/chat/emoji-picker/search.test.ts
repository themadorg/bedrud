import { describe, expect, it } from 'vitest'
import { searchEmojis } from './search'
import type { EmojiGroup } from './types'

const sample: EmojiGroup[] = [
  {
    category: 'Smileys',
    slug: 'smileys',
    emojis: [
      { emoji: '😀', name: 'grinning face', slug: 'grinning_face' },
      { emoji: '❤️', name: 'red heart', slug: 'red_heart' },
    ],
  },
]

describe('searchEmojis', () => {
  it('returns empty for blank query', () => {
    expect(searchEmojis(sample, '')).toEqual([])
  })

  it('matches emoji name', () => {
    expect(searchEmojis(sample, 'heart')).toEqual([{ emoji: '❤️', name: 'red heart', slug: 'red_heart' }])
  })

  it('matches emoji slug', () => {
    expect(searchEmojis(sample, 'grinning')).toEqual([{ emoji: '😀', name: 'grinning face', slug: 'grinning_face' }])
  })
})
