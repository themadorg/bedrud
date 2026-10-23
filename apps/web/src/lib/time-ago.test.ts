import { describe, expect, it } from 'vitest'
import { timeAgo } from './time-ago'

describe('timeAgo', () => {
  const now = 1_000_000_000_000

  it('should read as just now under a minute', () => {
    expect(timeAgo(now - 30_000, now)).toBe('just now')
  })

  it('should count whole minutes, hours and days', () => {
    expect(timeAgo(now - 5 * 60_000, now)).toBe('5m ago')
    expect(timeAgo(now - 3 * 3_600_000, now)).toBe('3h ago')
    expect(timeAgo(now - 2 * 86_400_000, now)).toBe('2d ago')
  })

  it('should step up exactly at each boundary', () => {
    expect(timeAgo(now - 59_999, now)).toBe('just now')
    expect(timeAgo(now - 60_000, now)).toBe('1m ago')
    expect(timeAgo(now - 3_599_999, now)).toBe('59m ago')
    expect(timeAgo(now - 3_600_000, now)).toBe('1h ago')
    expect(timeAgo(now - 86_399_999, now)).toBe('23h ago')
    expect(timeAgo(now - 86_400_000, now)).toBe('1d ago')
  })

  it('should count a single day rather than say yesterday', () => {
    // The labels sit in dense lists, where "1d ago" lines up with its neighbours.
    expect(timeAgo(now - 86_400_000, now)).not.toBe('yesterday')
  })
})
