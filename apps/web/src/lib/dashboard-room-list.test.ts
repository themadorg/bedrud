import { describe, expect, it } from 'vitest'
import { mergeDashboardRooms, serverRoomsOnly } from './dashboard-room-list'

const OWNED_NEVER_JOINED = { id: 'room-1', name: 'never-joined' }
const OWNED_AND_RECENT = { id: 'room-2', name: 'owned-and-recent' }
const OWNED_SECOND = { id: 'room-3', name: 'also-never-joined' }

/** Returns the RFC 3339 instant the server sends for a room's last activity. */
function serverTime(milliseconds: number): string {
  return new Date(milliseconds).toISOString()
}

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
    expect(entries[0]).toMatchObject({ kind: 'server', name: 'owned-and-recent', lastActivityAt: 3_000 })
  })

  it('should append owned rooms with no activity in server order', () => {
    const entries = mergeDashboardRooms(
      [OWNED_NEVER_JOINED, OWNED_AND_RECENT, OWNED_SECOND],
      [{ name: 'owned-and-recent', joinedAt: 3_000 }],
    )

    expect(entries.map((entry) => entry.name)).toEqual(['owned-and-recent', 'never-joined', 'also-never-joined'])
  })

  // A room somebody else used minutes ago must not sit below one this device opened weeks ago.
  it('should date an owned room by its server activity even when this device never joined it', () => {
    const entries = mergeDashboardRooms(
      [{ ...OWNED_NEVER_JOINED, lastActivityAt: serverTime(9_000) }],
      [{ name: 'visited-once', joinedAt: 2_000 }],
    )

    expect(entries.map((entry) => entry.name)).toEqual(['never-joined', 'visited-once'])
    expect(entries[0].lastActivityAt).toBe(9_000)
  })

  it("should prefer the server's activity over this device's visit", () => {
    const entries = mergeDashboardRooms(
      [{ ...OWNED_AND_RECENT, lastActivityAt: serverTime(7_000) }],
      [{ name: 'owned-and-recent', joinedAt: 3_000 }],
    )

    expect(entries[0].lastActivityAt).toBe(7_000)
  })

  it("should fall back to this device's visit when the server's activity cannot be read", () => {
    const entries = mergeDashboardRooms(
      [
        { ...OWNED_AND_RECENT, lastActivityAt: 'not a date' },
        { ...OWNED_SECOND, lastActivityAt: null },
      ],
      [{ name: 'owned-and-recent', joinedAt: 3_000 }],
    )

    expect(entries.map((entry) => entry.lastActivityAt)).toEqual([3_000, null])
  })

  it('should mark a room known only from history as recent', () => {
    const entries = mergeDashboardRooms([], [{ name: 'visited-once', joinedAt: 2_000 }])

    expect(entries[0]).toMatchObject({ kind: 'recent', name: 'visited-once', lastActivityAt: 2_000 })
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
  it('should keep only owned rooms, in server order when none can be dated', () => {
    const entries = serverRoomsOnly([OWNED_NEVER_JOINED, OWNED_AND_RECENT], [{ name: 'visited-once', joinedAt: 2_000 }])

    expect(entries.map((entry) => entry.name)).toEqual(['never-joined', 'owned-and-recent'])
    expect(entries.every((entry) => entry.kind === 'server')).toBe(true)
  })

  it('should carry local history onto an owned room that has some', () => {
    const entries = serverRoomsOnly([OWNED_AND_RECENT], [{ name: 'owned-and-recent', joinedAt: 3_000 }])

    expect(entries[0].lastActivityAt).toBe(3_000)
  })

  // The chip's order has to agree with the activity its cards print.
  it('should order owned rooms by activity, undated rooms last in server order', () => {
    const entries = serverRoomsOnly(
      [
        OWNED_NEVER_JOINED,
        { ...OWNED_SECOND, lastActivityAt: serverTime(4_000) },
        OWNED_AND_RECENT,
        { id: 'room-4', name: 'last-undated' },
      ],
      [{ name: 'owned-and-recent', joinedAt: 6_000 }],
    )

    expect(entries.map((entry) => entry.name)).toEqual([
      'owned-and-recent',
      'also-never-joined',
      'never-joined',
      'last-undated',
    ])
  })
})
