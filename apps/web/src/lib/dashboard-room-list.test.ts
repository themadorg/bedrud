import { describe, expect, it } from 'vitest'
import { mergeDashboardRooms, serverRoomsOnly, timeAgo } from './dashboard-room-list'

const OWNED_NEVER_JOINED = { id: 'room-1', name: 'never-joined' }
const OWNED_AND_RECENT = { id: 'room-2', name: 'owned-and-recent' }
const OWNED_SECOND = { id: 'room-3', name: 'also-never-joined' }

describe('mergeDashboardRooms', () => {
  it('should order dated entries most recent first', () => {
    const entries = mergeDashboardRooms(
      [OWNED_AND_RECENT],
      [
        { name: 'recent-only-older', joinedAt: 1_000 },
        { name: 'owned-and-recent', joinedAt: 3_000 },
        { name: 'recent-only-newer', joinedAt: 5_000 },
      ],
    )

    expect(entries.map((entry) => entry.name)).toEqual(['recent-only-newer', 'owned-and-recent', 'recent-only-older'])
  })

  // The rule exists for this case. A room that is both owned and recently joined has a server
  // record and local history, and must appear once, in its recency position.
  it('should emit a room that is both owned and recent exactly once', () => {
    const entries = mergeDashboardRooms([OWNED_AND_RECENT], [{ name: 'owned-and-recent', joinedAt: 3_000 }])

    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ kind: 'server', name: 'owned-and-recent', lastJoinedAt: 3_000 })
  })

  it('should append owned rooms with no history in server order', () => {
    const entries = mergeDashboardRooms(
      [OWNED_NEVER_JOINED, OWNED_AND_RECENT, OWNED_SECOND],
      [{ name: 'owned-and-recent', joinedAt: 3_000 }],
    )

    expect(entries.map((entry) => entry.name)).toEqual(['owned-and-recent', 'never-joined', 'also-never-joined'])
  })

  it('should mark a room known only from history as recent', () => {
    const entries = mergeDashboardRooms([], [{ name: 'visited-once', joinedAt: 2_000 }])

    expect(entries[0]).toMatchObject({ kind: 'recent', name: 'visited-once', lastJoinedAt: 2_000 })
  })

  it('should give every entry a key unique across both kinds', () => {
    const entries = mergeDashboardRooms([OWNED_NEVER_JOINED], [{ name: 'visited-once', joinedAt: 2_000 }])
    const keys = entries.map((entry) => entry.key)

    expect(new Set(keys).size).toBe(keys.length)
  })

  it('should return an empty list when there is nothing to show', () => {
    expect(mergeDashboardRooms([], [])).toEqual([])
  })
})

describe('serverRoomsOnly', () => {
  it('should keep only owned rooms, in server order', () => {
    const entries = serverRoomsOnly([OWNED_NEVER_JOINED, OWNED_AND_RECENT], [{ name: 'visited-once', joinedAt: 2_000 }])

    expect(entries.map((entry) => entry.name)).toEqual(['never-joined', 'owned-and-recent'])
    expect(entries.every((entry) => entry.kind === 'server')).toBe(true)
  })

  it('should carry local history onto an owned room that has some', () => {
    const entries = serverRoomsOnly([OWNED_AND_RECENT], [{ name: 'owned-and-recent', joinedAt: 3_000 }])

    expect(entries[0].lastJoinedAt).toBe(3_000)
  })
})

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
})
