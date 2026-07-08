import { describe, expect, it } from 'vitest'
import { groupReactionsByEmoji } from './chatReactions'

describe('groupReactionsByEmoji', () => {
  it('groups voters under each emoji', () => {
    const grouped = groupReactionsByEmoji(
      {
        alice: '👍',
        bob: '👍',
        carol: '❤️',
      },
      'alice',
      (id) => id,
    )

    expect(grouped).toHaveLength(2)
    expect(grouped[0]).toMatchObject({
      emoji: '👍',
      voters: [
        { identity: 'alice', name: 'alice', mine: true },
        { identity: 'bob', name: 'bob', mine: false },
      ],
    })
    expect(grouped[1]).toMatchObject({
      emoji: '❤️',
      voters: [{ identity: 'carol', name: 'carol', mine: false }],
    })
  })

  it('returns empty list when there are no reactions', () => {
    expect(groupReactionsByEmoji({}, 'alice', (id) => id)).toEqual([])
  })
})
