import { describe, expect, it } from 'vitest'
import type { ChatPoll } from '../MeetingContext'
import { computePollResults } from './chatPollUtils'

const poll: ChatPoll = {
  id: 'p1',
  question: 'Lunch?',
  options: [
    { id: 'a', text: 'Pizza' },
    { id: 'b', text: 'Salad' },
  ],
  votes: {
    alice: 'a',
    bob: 'a',
    carol: 'b',
  },
}

describe('computePollResults', () => {
  it('computes counts and percentages', () => {
    const results = computePollResults(poll, (id) => id)
    expect(results[0]).toMatchObject({ text: 'Pizza', count: 2, pct: 67, voterNames: ['alice', 'bob'] })
    expect(results[1]).toMatchObject({ text: 'Salad', count: 1, pct: 33, voterNames: ['carol'] })
  })

  it('handles zero votes', () => {
    const empty = { ...poll, votes: {} }
    const results = computePollResults(empty, (id) => id)
    expect(results.every((r) => r.count === 0 && r.pct === 0)).toBe(true)
  })
})
