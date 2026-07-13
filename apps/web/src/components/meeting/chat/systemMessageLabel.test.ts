import { describe, expect, it } from 'vitest'
import { systemMessageLabel } from './systemMessageLabel'

describe('systemMessageLabel', () => {
  it('prefers free-form message (stage share)', () => {
    expect(
      systemMessageLabel({
        type: 'system',
        event: 'stage',
        actor: 'Ada',
        message: 'Ada shared a mini-app on stage (Chess)',
        ts: 1,
      }),
    ).toBe('Ada shared a mini-app on stage (Chess)')
  })

  it('formats kick/ban when message missing', () => {
    expect(
      systemMessageLabel({ type: 'system', event: 'kick', actor: 'mod', target: 'user', ts: 1 }),
    ).toBe('user was kicked by mod')
    expect(
      systemMessageLabel({ type: 'system', event: 'ban', actor: 'mod', target: 'user', ts: 1 }),
    ).toBe('user was banned by mod')
  })
})
