import { describe, expect, it } from 'vitest'
import { groupReactionsByEmoji, restoreVoterChoice } from './chatReactions'

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

describe('restoreVoterChoice', () => {
  it('should put back the choice a failed send replaced', () => {
    expect(restoreVoterChoice({ alice: '🎉', bob: '👍' }, 'alice', '❤️')).toEqual({ alice: '❤️', bob: '👍' })
  })

  it('should remove a choice that did not exist before the failed send', () => {
    expect(restoreVoterChoice({ alice: '🎉', bob: '👍' }, 'alice', undefined)).toEqual({ bob: '👍' })
  })

  it('should leave other voters untouched', () => {
    const choices = { bob: '👍' }
    expect(restoreVoterChoice(choices, 'alice', undefined)).toEqual({ bob: '👍' })
  })
})
