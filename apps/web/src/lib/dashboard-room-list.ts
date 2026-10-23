import type { RecentRoom } from '#/lib/recent-rooms.store'

/** The fields the dashboard ordering needs from a server room. The API's room type is a superset. */
export interface NamedRoom {
  id: string
  name: string
  /** When anyone last joined the room, as the RFC 3339 instant the server stamps on every join. */
  lastActivityAt?: string | null
}

/**
 * One row of the dashboard list. A `server` entry has a room record behind it and can offer the
 * owner's controls; a `recent` entry is known only from this device's history, so it carries a name
 * and a timestamp and nothing else.
 */
export type DashboardEntry<TRoom extends NamedRoom> =
  | { kind: 'server'; key: string; name: string; room: TRoom; lastActivityAt: number | null }
  | { kind: 'recent'; key: string; name: string; lastActivityAt: number }

/** Indexes local history by room name, so a server room can find its own last visit. */
function lastJoinedByName(recents: readonly RecentRoom[]): Map<string, number> {
  return new Map(recents.map((recent) => [recent.name, recent.joinedAt]))
}

/**
 * Returns when a server room was last active. The server's record wins, since it counts every
 * participant's join where local history only knows this device's visits; the visit answers when
 * the server reports nothing, or something that does not parse.
 */
function resolveActivityAt(room: NamedRoom, lastVisit: number | null): number | null {
  const serverActivity = room.lastActivityAt ? Date.parse(room.lastActivityAt) : Number.NaN
  return Number.isNaN(serverActivity) ? lastVisit : serverActivity
}

/** Wraps a server room as an entry, dated by its own activity. */
function toServerEntry<TRoom extends NamedRoom>(room: TRoom, lastVisit: number | null): DashboardEntry<TRoom> {
  return {
    kind: 'server',
    key: `server:${room.id}`,
    name: room.name,
    room,
    lastActivityAt: resolveActivityAt(room, lastVisit),
  }
}

/** Wraps a room known only from local history as an entry. */
function toRecentEntry<TRoom extends NamedRoom>(recent: RecentRoom): DashboardEntry<TRoom> {
  return { kind: 'recent', key: `recent:${recent.name}`, name: recent.name, lastActivityAt: recent.joinedAt }
}

/**
 * Orders entries most recently active first. Entries nothing can date follow, in the order they came
 * in: the sort is stable, so they keep the server's order.
 */
function sortByActivity<TRoom extends NamedRoom>(entries: DashboardEntry<TRoom>[]): DashboardEntry<TRoom>[] {
  return entries.sort((left, right) => {
    if (left.lastActivityAt === null || right.lastActivityAt === null) {
      return (left.lastActivityAt === null ? 1 : 0) - (right.lastActivityAt === null ? 1 : 0)
    }
    return right.lastActivityAt - left.lastActivityAt
  })
}

/**
 * Builds the list behind the "All" chip: every room this device knows about, from either source,
 * in one order.
 *
 * Rooms that can be dated come first, most recently active first, whether the record is a server
 * room or local history alone. A server room is dated by the server's last activity, falling back
 * to this device's visit. Owned rooms nothing can date follow, in the order the server returned
 * them. A room that is both owned and recently joined appears once, as a server entry.
 *
 * This mirrors the Android client's rule in `DashboardScreen.kt`, minus its per-server filtering.
 */
export function mergeDashboardRooms<TRoom extends NamedRoom>(
  rooms: readonly TRoom[],
  recents: readonly RecentRoom[],
): DashboardEntry<TRoom>[] {
  const lastJoined = lastJoinedByName(recents)
  const serverRoomNames = new Set(rooms.map((room) => room.name))

  return sortByActivity([
    ...recents.filter((recent) => !serverRoomNames.has(recent.name)).map((recent) => toRecentEntry<TRoom>(recent)),
    ...rooms.map((room) => toServerEntry(room, lastJoined.get(room.name) ?? null)),
  ])
}

/**
 * Builds the list behind the "My Rooms" chip: owned rooms only, in the same activity order as
 * "All", so the order agrees with the time each card prints.
 */
export function serverRoomsOnly<TRoom extends NamedRoom>(
  rooms: readonly TRoom[],
  recents: readonly RecentRoom[],
): DashboardEntry<TRoom>[] {
  const lastJoined = lastJoinedByName(recents)
  return sortByActivity(rooms.map((room) => toServerEntry(room, lastJoined.get(room.name) ?? null)))
}
