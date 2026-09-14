import type { RecentRoom } from '#/lib/recent-rooms.store'

/** The fields the dashboard ordering needs from a server room. The API's room type is a superset. */
export interface NamedRoom {
  id: string
  name: string
}

/**
 * One row of the dashboard list. A `server` entry has a room record behind it and can offer the
 * owner's controls; a `recent` entry is known only from this device's history, so it carries a name
 * and a timestamp and nothing else.
 */
export type DashboardEntry<TRoom extends NamedRoom> =
  | { kind: 'server'; key: string; name: string; room: TRoom; lastJoinedAt: number | null }
  | { kind: 'recent'; key: string; name: string; lastJoinedAt: number }

const MILLISECONDS_PER_MINUTE = 60_000
const MINUTES_PER_HOUR = 60
const HOURS_PER_DAY = 24

/** Indexes local history by room name, so a server room can find its own last visit. */
function lastJoinedByName(recents: readonly RecentRoom[]): Map<string, number> {
  return new Map(recents.map((recent) => [recent.name, recent.joinedAt]))
}

/** Wraps a server room as an entry, carrying its last visit when this device has one. */
function toServerEntry<TRoom extends NamedRoom>(room: TRoom, lastJoinedAt: number | null): DashboardEntry<TRoom> {
  return { kind: 'server', key: `server:${room.id}`, name: room.name, room, lastJoinedAt }
}

/** Wraps a room known only from local history as an entry. */
function toRecentEntry<TRoom extends NamedRoom>(recent: RecentRoom): DashboardEntry<TRoom> {
  return { kind: 'recent', key: `recent:${recent.name}`, name: recent.name, lastJoinedAt: recent.joinedAt }
}

/**
 * Builds the list behind the "All" chip: every room this device knows about, from either source,
 * in one order.
 *
 * Rooms with a last visit come first, most recent first, whether the record is a server room or
 * local history alone — the question the ordering answers is "what were you in most recently", and
 * ownership does not change that. Owned rooms never joined from this device follow, in the order
 * the server returned them, because there is nothing to date them by. A room that is both owned and
 * recently joined appears once, as a server entry, in its recency position.
 *
 * This mirrors the Android client's rule in `DashboardScreen.kt`, minus its per-server filtering.
 */
export function mergeDashboardRooms<TRoom extends NamedRoom>(
  rooms: readonly TRoom[],
  recents: readonly RecentRoom[],
): DashboardEntry<TRoom>[] {
  const lastJoined = lastJoinedByName(recents)
  const serverRoomNames = new Set(rooms.map((room) => room.name))

  const datedEntries: { entry: DashboardEntry<TRoom>; lastJoinedAt: number }[] = [
    ...recents
      .filter((recent) => !serverRoomNames.has(recent.name))
      .map((recent) => ({ entry: toRecentEntry<TRoom>(recent), lastJoinedAt: recent.joinedAt })),
    ...rooms
      .filter((room) => lastJoined.has(room.name))
      .map((room) => {
        const lastJoinedAt = lastJoined.get(room.name) as number
        return { entry: toServerEntry(room, lastJoinedAt), lastJoinedAt }
      }),
  ]

  const neverJoined = rooms.filter((room) => !lastJoined.has(room.name)).map((room) => toServerEntry(room, null))

  return [
    ...datedEntries.sort((left, right) => right.lastJoinedAt - left.lastJoinedAt).map(({ entry }) => entry),
    ...neverJoined,
  ]
}

/**
 * Builds the list behind the "My Rooms" chip: owned rooms only, in the order the server returned
 * them, each carrying its last visit so the card can show the same label it shows under "All".
 */
export function serverRoomsOnly<TRoom extends NamedRoom>(
  rooms: readonly TRoom[],
  recents: readonly RecentRoom[],
): DashboardEntry<TRoom>[] {
  const lastJoined = lastJoinedByName(recents)
  return rooms.map((room) => toServerEntry(room, lastJoined.get(room.name) ?? null))
}

/**
 * Formats a last-visit timestamp as the card's presence label, the web's reading of the Android
 * card's presence line. `now` is a parameter so the label can be tested without faking the clock.
 */
export function timeAgo(timestamp: number, now: number = Date.now()): string {
  const minutes = Math.floor((now - timestamp) / MILLISECONDS_PER_MINUTE)
  if (minutes < 1) return 'just now'
  if (minutes < MINUTES_PER_HOUR) return `${minutes}m ago`

  const hours = Math.floor(minutes / MINUTES_PER_HOUR)
  if (hours < HOURS_PER_DAY) return `${hours}h ago`

  return `${Math.floor(hours / HOURS_PER_DAY)}d ago`
}
