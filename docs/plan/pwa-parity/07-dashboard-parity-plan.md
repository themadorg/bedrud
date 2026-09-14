# Dashboard parity implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the web dashboard the Android dashboard's shape on phones — a quick-join bar that is always present and accepts a pasted link, two filter chips over one merged room list — and move the tree onto the app's single 1024px breakpoint.

**Architecture:** Two pure helpers in `src/lib/` carry all the new logic — parsing a join input, and merging server rooms with local history into one ordered list — so the behaviour is testable without rendering. The route file wires them up; `RoomCard` grows a second entry kind and `RecentRoomRow` disappears. A small `FilterChip` reuses `badgeVariants` so the chip's corner stays single-sourced.

**Tech Stack:** React 19, TanStack Router and Query, TailwindCSS v4, shadcn/ui, Zustand, Vitest 4, Biome, Bun.

**Spec:** [06 — Dashboard parity](./06-dashboard-parity.md). Read it before Task 1.

## Global Constraints

- One breakpoint. Phone layout is below **1024px**, expressed as `lg:` and `max-lg:`. No `sm:`, `max-sm:`, `md:` or `max-md:` prefix may remain anywhere in the dashboard tree when this unit is done.
- Exactly **one `rounded-*` class per element**. `cn` is `twMerge(clsx(...))` and keeps the last radius it sees, so a second one is silently dead. A chip's corner is `rounded-sm` (8px) and comes from `badgeVariants`, never re-declared.
- **Biome owns final layout.** Write code in the shape that reads best; run `bunx biome check --write` on touched files before committing and take its output verbatim. Its import sort is case-insensitive.
- **Every commit compiles on its own.** A changed signature and the call sites that feed it belong in one commit, along with whatever the change deletes.
- **No abbreviated identifiers.** `index`, not `i`; `entry`, not `e`. Lambda parameters included.
- **Comments sit above what they describe**, capitalised, a full sentence, explaining why rather than restating the code.
- Tests are written first and seen failing before the code that satisfies them.
- The repo's test convention is **pure logic or source-reading**. There are no React render tests and this unit adds none; a claim about rendered structure is pinned by reading the source file, the way `settings.route.test.ts` does.
- Test descriptions are imperative and begin with `should`.
- The one combined verify command is `bun run check && bun run test`, run from `apps/web`.
- No attribution of any kind in commits.

---

## File Structure

| File | Responsibility |
|---|---|
| `src/lib/join-input.ts` | **Create.** Resolves whatever is typed or pasted into the quick-join field to a room name, or null. |
| `src/lib/join-input.test.ts` | **Create.** Covers every input form the parser accepts and rejects. |
| `src/lib/dashboard-room-list.ts` | **Create.** Merges server rooms with local history into one ordered entry list, exposes the server-only list, and owns the `timeAgo` label. |
| `src/lib/dashboard-room-list.test.ts` | **Create.** Covers the ordering rule, the both-owned-and-recent case, and the label. |
| `src/components/dashboard/FilterChip.tsx` | **Create.** One filter chip: a real `<button>` wearing `badgeVariants`, with a leading check when selected. |
| `src/components/dashboard/RoomCard.tsx` | **Modify.** Takes a `DashboardEntry` instead of a `Room`, and renders the recent-only kind. |
| `src/routes/dashboard.index.tsx` | **Modify.** Quick-join always visible and parsing; chips replace tabs; one merged list; `RecentRoomRow` deleted. |
| `src/routes/dashboard.index.route.test.ts` | **Create.** Pins the structural claims: the bar cannot be hidden, no stale breakpoint prefixes. |
| `src/components/dashboard/dialog-breakpoints.test.ts` | **Create.** Pins the two dashboard dialogs to the shared breakpoint. |
| `src/components/dashboard/CreateRoomDialog.tsx` | **Modify.** `sm:max-w-md` → `lg:max-w-md`. |
| `src/components/dashboard/RoomSettingsDialog.tsx` | **Modify.** `sm:max-w-sm` → `lg:max-w-sm`. |
| `apps/web/AGENTS.md` | **Modify.** A "Phone dashboard" section beside the existing "Phone settings" one. |
| `docs/plan/pwa-parity/01-overview-and-units.md` | **Modify.** Unit 6 row, and the component-vocabulary table. |

### Task order

Tasks 1 to 3 add new files and touch nothing that exists, so each stands alone. Task 4 changes the
quick-join bar only. Task 5 changes `RoomCard`'s signature, rewrites the call site and deletes
`RecentRoomRow` in one commit, because a commit that changed the signature without its call site
would not compile. Tasks 6 and 7 are the sweep and the docs.

---

### Task 1: The join-input parser

**Files:**
- Create: `apps/web/src/lib/join-input.ts`
- Test: `apps/web/src/lib/join-input.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `parseJoinInput(rawInput: string): string | null`.

Mirrors `BedrudURLParser.parseJoinInput` in the Android client, minus the multi-server resolution the web has no concept of.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/lib/join-input.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { parseJoinInput } from './join-input'

describe('parseJoinInput', () => {
  it('should take the room from a full meeting URL', () => {
    expect(parseJoinInput('https://bedrud.example/m/team-standup')).toBe('team-standup')
  })

  it('should take the room from the legacy call path', () => {
    expect(parseJoinInput('https://bedrud.example/c/team-standup')).toBe('team-standup')
  })

  it('should accept a URL with no scheme', () => {
    expect(parseJoinInput('bedrud.example/m/team-standup')).toBe('team-standup')
  })

  it('should accept a URL carrying a port', () => {
    expect(parseJoinInput('https://bedrud.example:8080/m/team-standup')).toBe('team-standup')
  })

  it('should ignore a query string and a fragment', () => {
    expect(parseJoinInput('https://bedrud.example/m/team-standup?from=email#top')).toBe('team-standup')
  })

  it('should accept a bare path', () => {
    expect(parseJoinInput('/m/team-standup')).toBe('team-standup')
  })

  // A plain name is the common case: someone reading a room name aloud over a call.
  it('should slugify a plain room name', () => {
    expect(parseJoinInput('Team Standup')).toBe('team-standup')
    expect(parseJoinInput('  Team   Standup  ')).toBe('team-standup')
  })

  // A URL that names no room must not fall through to slugification: the old code turned the whole
  // URL into a room name and navigated to a room that could never exist.
  it('should reject a URL with no room segment', () => {
    expect(parseJoinInput('https://bedrud.example/dashboard')).toBeNull()
    expect(parseJoinInput('https://bedrud.example/m/')).toBeNull()
  })

  it('should reject blank input', () => {
    expect(parseJoinInput('')).toBeNull()
    expect(parseJoinInput('   ')).toBeNull()
  })
})
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
cd apps/web && bunx vitest run src/lib/join-input.test.ts
```

Expected: fails to resolve `./join-input`.

- [ ] **Step 3: Write the implementation**

Create `apps/web/src/lib/join-input.ts`:

```ts
/** The canonical share path segment, `{server}/m/{room}`. */
const MEETING_PATH_SEGMENT = 'm'

/** The alternate call path segment, `{server}/c/{room}`, kept for links shared before the rename. */
const CALL_PATH_SEGMENT = 'c'

const ROOM_PATH_PATTERN = new RegExp(`(?:^|/)(?:${MEETING_PATH_SEGMENT}|${CALL_PATH_SEGMENT})/([^/?#]+)`)

/** Reports whether the input looks like a URL or a path rather than a room name typed by hand. */
function looksLikeLocation(trimmedInput: string): boolean {
  return trimmedInput.includes('/')
}

/** Turns a name a person typed into the lowercase hyphenated form room names take. */
function slugify(trimmedInput: string): string {
  return trimmedInput.toLowerCase().replace(/\s+/g, '-')
}

/**
 * Resolves whatever was typed or pasted into the quick-join field to a room name.
 *
 * Accepts a full meeting URL on any host, with or without a scheme or a port, a bare `/m/` or `/c/`
 * path, or a plain room name. Returns null when the input names no room, so the caller can say so
 * rather than navigating to a room that cannot exist — which is what slugifying a whole URL did.
 */
export function parseJoinInput(rawInput: string): string | null {
  const trimmedInput = rawInput.trim()
  if (!trimmedInput) return null

  if (looksLikeLocation(trimmedInput)) {
    const roomSegment = ROOM_PATH_PATTERN.exec(trimmedInput)?.[1]
    return roomSegment ? decodeURIComponent(roomSegment) : null
  }

  return slugify(trimmedInput)
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
cd apps/web && bunx vitest run src/lib/join-input.test.ts
```

Expected: 9 passing.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/join-input.ts apps/web/src/lib/join-input.test.ts
git commit -m "add a join-input parser that accepts pasted meeting links"
```

---

### Task 2: The merged room list

**Files:**
- Create: `apps/web/src/lib/dashboard-room-list.ts`
- Test: `apps/web/src/lib/dashboard-room-list.test.ts`

**Interfaces:**
- Consumes: `RecentRoom` from `#/lib/recent-rooms.store`.
- Produces: `NamedRoom`, `DashboardEntry<TRoom>`, `mergeDashboardRooms`, `serverRoomsOnly`, `timeAgo`.

Implements the ordering rule from `DashboardScreen.kt:538`, simplified for a single server and for the web's `RecentRoom`, which carries only `name` and `joinedAt`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/lib/dashboard-room-list.test.ts`:

```ts
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

    expect(entries.map((entry) => entry.name)).toEqual([
      'recent-only-newer',
      'owned-and-recent',
      'recent-only-older',
    ])
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

    expect(entries.map((entry) => entry.name)).toEqual([
      'owned-and-recent',
      'never-joined',
      'also-never-joined',
    ])
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
    const entries = serverRoomsOnly(
      [OWNED_NEVER_JOINED, OWNED_AND_RECENT],
      [{ name: 'visited-once', joinedAt: 2_000 }],
    )

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
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
cd apps/web && bunx vitest run src/lib/dashboard-room-list.test.ts
```

Expected: fails to resolve `./dashboard-room-list`.

- [ ] **Step 3: Write the implementation**

Create `apps/web/src/lib/dashboard-room-list.ts`:

```ts
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
 * local history alone — the question the ordering answers is "what were you in most recently",
 * and ownership does not change that. Owned rooms never joined from this device follow, in the
 * order the server returned them, because there is nothing to date them by. A room that is both
 * owned and recently joined appears once, as a server entry, in its recency position.
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
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
cd apps/web && bunx vitest run src/lib/dashboard-room-list.test.ts
```

Expected: 11 passing.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/lib/dashboard-room-list.ts apps/web/src/lib/dashboard-room-list.test.ts
git commit -m "add the merged dashboard room list and its ordering rule"
```

---

### Task 3: The filter chip

**Files:**
- Create: `apps/web/src/components/dashboard/FilterChip.tsx`

**Interfaces:**
- Consumes: `badgeVariants` from `@/components/ui/badge`.
- Produces: `FilterChip`, props `{ label: string; selected: boolean; onSelect: () => void }`.

`DESIGN.md:123` says the badge is the chip, not a pill, and `badgeVariants` already carries the 8px corner. `Badge` itself renders a `<div>`, which cannot take focus or fire on Enter, so the chip is its own `<button>` wearing the same variant string rather than a `Badge` forced into a button role.

- [ ] **Step 1: Write the component**

Create `apps/web/src/components/dashboard/FilterChip.tsx`:

```tsx
import { Check } from 'lucide-react'

import { badgeVariants } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  label: string
  selected: boolean
  onSelect: () => void
}

/**
 * One filter chip, matching the Android client's `FilterChip`: a leading check appears on the
 * selected chip only, so the active filter reads as "this one is on" rather than relying on the
 * fill colour alone.
 *
 * The corner and typography come from `badgeVariants`, which owns the chip shape for the whole app.
 * No radius class is added here — `cn` keeps only the last one it sees, so a second would silently
 * replace the token.
 */
export function FilterChip({ label, selected, onSelect }: Props) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        badgeVariants({ variant: selected ? 'default' : 'outline' }),
        'h-8 cursor-pointer gap-1.5 px-3',
        selected ? 'border-transparent' : 'border-input hover:bg-accent',
      )}
    >
      {selected && <Check className="h-3.5 w-3.5" />}
      {label}
    </button>
  )
}
```

- [ ] **Step 2: Verify it compiles and is formatted**

```bash
cd apps/web && bunx biome check --write src/components/dashboard/FilterChip.tsx && bunx tsc --noEmit
```

Expected: no errors. There is no test here: the repo has no render tests, and a source-reading test asserting a component's own class string would restate the file rather than pin a claim. The chip's corner is covered by the existing design-token test, which sweeps every file.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/dashboard/FilterChip.tsx
git commit -m "add a filter chip built on the shared badge shape"
```

---

### Task 4: The quick-join bar on every width

**Files:**
- Modify: `apps/web/src/routes/dashboard.index.tsx`
- Create: `apps/web/src/routes/dashboard.index.route.test.ts`

**Interfaces:**
- Consumes: `parseJoinInput` from Task 1.
- Produces: `dashboard.index.route.test.ts`, which Task 6 extends.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/routes/dashboard.index.route.test.ts`:

```ts
// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dashboardRouteSource = readFileSync(new URL('./dashboard.index.tsx', import.meta.url), 'utf8')

describe('the dashboard route', () => {
  // The bar was hidden below 768px, which left a phone with no way to reach a room it did not
  // already own. No source-reading test can observe a viewport, but it can pin the absence of every
  // prefix that could hide the bar again.
  it('should carry no breakpoint prefix that hides content', () => {
    expect(dashboardRouteSource).not.toMatch(/max-(sm|md):hidden/)
  })

  it('should resolve the join field through the shared parser', () => {
    expect(dashboardRouteSource).toContain('parseJoinInput')
  })

  // The old code slugified whatever was in the field, so a pasted URL became a room name that could
  // never resolve. The parser returns null for that input and the bar has to handle the null rather
  // than pass it on.
  it('should report input that names no room rather than navigating', () => {
    expect(dashboardRouteSource).toMatch(/parseJoinInput\(value\)\s*\n\s*if \(!roomName\)/)
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bunx vitest run src/routes/dashboard.index.route.test.ts
```

Expected: `should carry no breakpoint prefix that hides content` fails on the `max-md:hidden` still in the file, and the other two fail on the missing parser.

- [ ] **Step 3: Rewrite the bar**

Replace `QuickJoinBar` at `dashboard.index.tsx:59-96` with:

```tsx
function QuickJoinBar({ onJoin, onCreate }: { onJoin: (name: string) => void; onCreate: () => void }) {
  const [value, setValue] = useState('')

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const roomName = parseJoinInput(value)
    if (!roomName) {
      toast.error('That does not look like a room name or a meeting link')
      return
    }
    onJoin(roomName)
  }

  return (
    <div className="flex items-center gap-2">
      <form
        onSubmit={handleSubmit}
        className="flex h-9 flex-1 items-center gap-2 rounded-lg border border-input bg-background px-3 focus-within:ring-2 focus-within:ring-ring"
      >
        <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        {/* Room names are lowercase and have no spaces, so the phone keyboard should not
            capitalise, correct or spell-check what is typed. This mirrors the Android field's
            `KeyboardType.Uri` with capitalisation and auto-correct off. */}
        <Input
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="Join by room name or invite link..."
          inputMode="url"
          autoComplete="off"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          className="h-full flex-1 border-none focus-visible:ring-0 px-0"
        />
        {/* Disabled rather than hidden while the field is empty: a button that appears as you type
            shifts the row under your thumb. Android disables it for the same reason. */}
        <Button type="submit" size="sm" className="gap-1" disabled={!value.trim()}>
          Join <ArrowRight className="h-3 w-3" />
        </Button>
      </form>
      {/* Desktop only: phones create a room from the floating button in the bottom navigation,
          which is where the Android client puts it too. */}
      <Button type="button" variant="default" size="sm" onClick={onCreate} className="max-lg:hidden">
        <Plus className="h-3.5 w-3.5" />
        New room
      </Button>
    </div>
  )
}
```

Add `import { parseJoinInput } from '#/lib/join-input'` to the imports.

- [ ] **Step 4: Run the test and watch it pass**

```bash
cd apps/web && bunx vitest run src/routes/dashboard.index.route.test.ts
```

Expected: 3 passing.

- [ ] **Step 5: Verify the tree still compiles**

```bash
cd apps/web && bunx biome check --write src/routes/dashboard.index.tsx && bunx tsc --noEmit
```

Expected: clean. Nothing else changed yet.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/routes/dashboard.index.tsx apps/web/src/routes/dashboard.index.route.test.ts
git commit -m "show the quick join bar on phones and accept pasted meeting links"
```

---

### Task 5: Chips over one merged list

**Files:**
- Modify: `apps/web/src/components/dashboard/RoomCard.tsx`
- Modify: `apps/web/src/routes/dashboard.index.tsx`

**Interfaces:**
- Consumes: `DashboardEntry`, `mergeDashboardRooms`, `serverRoomsOnly`, `timeAgo` from Task 2; `FilterChip` from Task 3.
- Produces: `RoomCard` taking `{ entry, onJoin, onDelete?, onSettings?, onRemove? }`.

This is one commit, not two. It changes `RoomCard`'s signature, rewrites the call site that feeds
it, and deletes `RecentRoomRow` — splitting those leaves a commit that does not compile.

- [ ] **Step 1: Change the card's props**

Replace the `Room` interface and `Props` at `RoomCard.tsx:22-42` with:

`RoomCard.tsx` imports through `@/` throughout, so its new import uses `@/` too, even though the
route file reaches the same directory through `#/`. Both aliases resolve to `./src/*`; match the
file you are in.

```tsx
import { type DashboardEntry, timeAgo } from '@/lib/dashboard-room-list'

interface Room {
  id: string
  name: string
  isPublic: boolean
  maxParticipants: number
  isActive: boolean
  settings: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee?: boolean
  }
}

interface Props {
  entry: DashboardEntry<Room>
  onJoin: () => void
  onDelete?: () => void
  onSettings?: () => void
  onRemove?: () => void
}
```

- [ ] **Step 2: Branch the card body on the entry kind**

Derive the room once and guard every server-only block on it:

```tsx
export function RoomCard({ entry, onJoin, onDelete, onSettings, onRemove }: Props) {
  const [copied, setCopied] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  // A recent entry has no server record, so it has no capacity, no visibility and no capabilities
  // to show. Everything below that reads `room` is skipped for it.
  const room = entry.kind === 'server' ? entry.room : null

  const capabilities = room
    ? [
        room.settings.allowAudio ? { icon: Mic, label: 'Audio' } : null,
        room.settings.allowVideo ? { icon: Video, label: 'Video' } : null,
        room.settings.allowChat ? { icon: MessageSquare, label: 'Chat' } : null,
      ].filter((item): item is { icon: typeof Mic; label: string } => Boolean(item))
    : []

  function copyLink() {
    void navigator.clipboard.writeText(`${window.location.origin}/m/${entry.name}`)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const capacityLabel = room && room.maxParticipants > 0 ? `${room.maxParticipants}` : 'Open'
  const isActive = room?.isActive ?? false
```

In the badge row, guard the three server-only badges and add the presence label:

```tsx
          <div className="flex flex-wrap items-center gap-1">
            {isActive && (
              <Badge
                variant="outline"
                className="h-5 gap-1 border-emerald-500/30 bg-emerald-500/10 px-1.5 text-[10px] text-emerald-600 dark:text-emerald-400"
              >
                <span className="h-1 w-1 rounded-full bg-emerald-500" />
                Live
              </Badge>
            )}
            {room && (
              <Badge variant="outline" className="h-5 gap-1 px-1.5 text-[10px]">
                {room.isPublic ? <Globe className="h-2.5 w-2.5" /> : <Lock className="h-2.5 w-2.5" />}
                {room.isPublic ? 'Public' : 'Private'}
              </Badge>
            )}
            {room?.settings.e2ee && (
              <Badge className="h-5 gap-1 px-1.5 text-[10px]">
                <ShieldCheck className="h-2.5 w-2.5" />
                E2EE
              </Badge>
            )}
            {room?.settings.requireApproval && (
              <Badge variant="outline" className="h-5 gap-1 px-1.5 text-[10px]">
                <UserCheck className="h-2.5 w-2.5" />
                Approval
              </Badge>
            )}
          </div>
          <h3 className="mt-1.5 truncate font-mono text-[13px] font-semibold leading-tight">{entry.name}</h3>
          {entry.lastJoinedAt !== null && (
            <p className="mt-0.5 text-[11px] text-muted-foreground/70">{timeAgo(entry.lastJoinedAt)}</p>
          )}
```

The capacity and capability row renders only for a server entry:

```tsx
      {room && (
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <Users className="h-3 w-3 shrink-0" />
            {capacityLabel}
          </span>
          {capabilities.map(({ icon: Icon, label }) => (
            <span key={label} className="inline-flex items-center gap-0.5" title={label}>
              <Icon className="h-3 w-3 shrink-0" />
              {label}
            </span>
          ))}
        </div>
      )}
```

- [ ] **Step 3: Add the Remove control**

In the action row, the Join button now reads `isActive`; keep Settings and Delete as they are and add Remove after them:

```tsx
        {onRemove && (
          <Button
            variant="outline"
            size="icon"
            onClick={onRemove}
            className="h-8 w-8"
            aria-label="Remove from recent rooms"
            title="Remove from recent rooms"
          >
            <X className="h-3.5 w-3.5 text-muted-foreground" />
          </Button>
        )}
```

Add `X` to the `lucide-react` import. Removing a room from local history is not destructive — it drops a row from this device's list and nothing else — so it takes the plain outline treatment rather than the destructive styling Delete carries, and it asks for no confirmation. This matches the Android client, where the recent card's swipe action is `secondaryContainer` and fires immediately.

- [ ] **Step 4: Delete `RecentRoomRow` and the local `timeAgo`**

In `dashboard.index.tsx`, remove `timeAgo` at `:46-55` and `RecentRoomRow` at `:100-129`. `timeAgo` now lives in `dashboard-room-list.ts` and the card calls it. Drop `Clock` and `X` from the `lucide-react` import if nothing else uses them.

- [ ] **Step 5: Swap the tabs for chips**

Replace the `tab` state with a filter:

```tsx
  const [activeFilter, setActiveFilter] = useState<'all' | 'mine'>('all')
```

and the `Tabs` block at `:246-257` with the chip row:

```tsx
        <div className="flex items-center gap-2">
          <FilterChip label="All" selected={activeFilter === 'all'} onSelect={() => setActiveFilter('all')} />
          <FilterChip label="My Rooms" selected={activeFilter === 'mine'} onSelect={() => setActiveFilter('mine')} />
        </div>
```

Drop the `Tabs`, `TabsList` and `TabsTrigger` import; add `FilterChip`.

- [ ] **Step 6: Build one list**

Replace the two filtered lists at `:221-231` with one:

```tsx
  const normalizedQuery = query.trim().toLowerCase()
  const entries = (
    activeFilter === 'all' ? mergeDashboardRooms(rooms ?? [], recentRooms) : serverRoomsOnly(rooms ?? [], recentRooms)
  ).filter((entry) => !normalizedQuery || entry.name.toLowerCase().includes(normalizedQuery))
```

The previous code sorted server rooms active-first, then by name. That sort goes: the merged list is ordered by recency, which is the order the Android client uses and the one this unit adopts.

- [ ] **Step 7: Render one grid**

Replace both the `tab === 'rooms'` and `tab === 'recent'` blocks with:

```tsx
      <div className="rounded-xl border bg-card/50">
        {isLoading ? (
          <div className="p-2">
            <SkeletonRows />
          </div>
        ) : entries.length > 0 ? (
          <div className="grid grid-cols-1 gap-2 p-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {entries.map((entry) => (
              <RoomCard
                key={entry.key}
                entry={entry}
                onJoin={() => handleJoin(entry.name)}
                onDelete={entry.kind === 'server' ? () => deleteRoom.mutate(entry.room.id) : undefined}
                onSettings={entry.kind === 'server' ? () => setSettingsRoom(entry.room) : undefined}
                onRemove={entry.kind === 'recent' ? () => removeRecent(entry.name) : undefined}
              />
            ))}
          </div>
        ) : (
          <div className="px-4 py-12 text-center">
            {(rooms?.length ?? 0) > 0 || recentRooms.length > 0 ? (
              <>
                <p className="text-sm font-medium">No rooms match "{query}"</p>
                <Button variant="link" type="button" onClick={() => setQuery('')} className="mt-2 text-sm">
                  Clear filter
                </Button>
              </>
            ) : (
              <>
                <p className="text-sm font-medium">No rooms yet</p>
                <p className="mt-1 text-xs text-muted-foreground">Create your first room to get started.</p>
                <Button type="button" variant="default" size="sm" onClick={() => setCreateOpen(true)} className="mt-3">
                  <Plus className="h-3.5 w-3.5" />
                  New room
                </Button>
              </>
            )}
          </div>
        )}
      </div>
```

The room handed to `RoomCard` is the API room itself now, rather than the hard-coded capability object the old call site built at `:282-295`. That object claimed every room allowed audio, video and chat and required no approval, whatever the room's real settings were, so the capability row was decorative. Passing the real room makes it true.

- [ ] **Step 8: Verify**

```bash
cd apps/web && bunx biome check --write src/components/dashboard/RoomCard.tsx src/routes/dashboard.index.tsx && bunx tsc --noEmit && bun run test
```

Expected: clean typecheck, every test passing.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/components/dashboard/RoomCard.tsx apps/web/src/routes/dashboard.index.tsx
git commit -m "replace the room tabs with filter chips over one merged list"
```

---

### Task 6: The breakpoint sweep

**Files:**
- Modify: `apps/web/src/routes/dashboard.index.tsx`, `apps/web/src/routes/dashboard.index.route.test.ts`
- Modify: `apps/web/src/components/dashboard/CreateRoomDialog.tsx`, `apps/web/src/components/dashboard/RoomSettingsDialog.tsx`
- Create: `apps/web/src/components/dashboard/dialog-breakpoints.test.ts`

Tasks 4 and 5 already removed `max-md:hidden`, the three `sm:` hover-reveal prefixes and `sm:grid-cols-2`. What remains is the header's `md:block` and the two dialogs.

- [ ] **Step 1: Extend the guard test**

Add to `dashboard.index.route.test.ts`:

```ts
  // Unit 1 made 1024px the app's one phone breakpoint. The dashboard tree was never swept.
  it('should carry no breakpoint prefix outside the shared scale', () => {
    expect(dashboardRouteSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
```

Create `apps/web/src/components/dashboard/dialog-breakpoints.test.ts`:

```ts
// @vitest-environment node
//
// These files are read from disk, so this runs in Node rather than jsdom: jsdom's URL resolves
// `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const DIALOG_FILE_NAMES = ['CreateRoomDialog.tsx', 'RoomSettingsDialog.tsx']

describe('the dashboard dialogs', () => {
  it.each(DIALOG_FILE_NAMES)('should switch %s at the shared phone breakpoint', (fileName) => {
    const source = readFileSync(new URL(`./${fileName}`, import.meta.url), 'utf8')
    expect(source).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})
```

- [ ] **Step 2: Run both and watch them fail**

```bash
cd apps/web && bunx vitest run src/routes/dashboard.index.route.test.ts src/components/dashboard/dialog-breakpoints.test.ts
```

Expected: the new assertions fail on `md:block`, `sm:max-w-md` and `sm:max-w-sm`.

- [ ] **Step 3: Move the three prefixes**

- `dashboard.index.tsx:237`: `hidden md:block` → `hidden lg:block`
- `CreateRoomDialog.tsx`: `sm:max-w-md` → `lg:max-w-md`
- `RoomSettingsDialog.tsx`: `sm:max-w-sm` → `lg:max-w-sm`

Prefix-only. No class is added, removed or reordered.

- [ ] **Step 4: Run them and watch them pass**

```bash
cd apps/web && bunx vitest run src/routes/dashboard.index.route.test.ts src/components/dashboard/dialog-breakpoints.test.ts
```

Expected: all passing.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/routes/dashboard.index.tsx apps/web/src/routes/dashboard.index.route.test.ts apps/web/src/components/dashboard/CreateRoomDialog.tsx apps/web/src/components/dashboard/RoomSettingsDialog.tsx apps/web/src/components/dashboard/dialog-breakpoints.test.ts
git commit -m "switch the dashboard tree at the shared phone breakpoint"
```

---

### Task 7: Documentation

**Files:**
- Modify: `apps/web/AGENTS.md`, `docs/plan/pwa-parity/01-overview-and-units.md`

- [ ] **Step 1: Describe the dashboard in `apps/web/AGENTS.md`**

Add a "Phone dashboard" section immediately after the "Mobile detection" section.

This branch is cut from unit 1, not unit 5, so the "Phone settings" section unit 5 adds is not here
yet and cannot be the anchor. Both sections land after "Mobile detection"; when the two branches
meet, their order in the file does not matter.

```markdown
### Phone dashboard

`/dashboard` is one list at every width. The quick-join bar is always visible and resolves its
field through `parseJoinInput` in `src/lib/join-input.ts`, which accepts a full meeting URL, a bare
`/m/` or `/c/` path, or a plain room name, and returns null for anything else. The "New room"
button beside it is desktop-only; phones create from the floating button in `MobileBottomNav`.

Two filter chips, All and My Rooms, replace the tabs the page used to carry. `FilterChip` wears
`badgeVariants` so the chip corner stays single-sourced. Under All the list holds server rooms and
rooms known only from this device's history, merged and ordered by `mergeDashboardRooms` in
`src/lib/dashboard-room-list.ts`; under My Rooms it holds server rooms only. `RoomCard` renders
both kinds: a locally known room has no capacity, visibility or capability row, and carries Remove
where an owned room carries Settings and Delete.

The admin tree has no phone layout, so `MobileBottomNav` deliberately has no Admin tab even though
the Android client shows one.
```

- [ ] **Step 2: Update the plan overview**

In `docs/plan/pwa-parity/01-overview-and-units.md`:

- Add a unit 6 row to the units table:
  `| 6 | Dashboard parity | quick-join bar on phones with pasted-link support, filter chips over one merged room list, and the dashboard tree moved to the shared breakpoint | done, [06](./06-dashboard-parity.md), [07](./07-dashboard-parity-plan.md) |`
- Update the status line at the top to name 06 alongside 02 and 04.
- Add a row to the component-vocabulary table:
  ``| `QuickJoinBar` + `FilterRow` (dashboard) | quick-join bar + filter chips | unit 6 |``

- [ ] **Step 3: Run the full verify**

```bash
cd apps/web && bun run check && bun run test
```

Expected: 395+ files checked with the three known `noDocumentCookie` warnings in `api.test.ts`, and every test file passing.

- [ ] **Step 4: Commit**

```bash
git add apps/web/AGENTS.md docs/plan/pwa-parity/01-overview-and-units.md docs/plan/pwa-parity/06-dashboard-parity.md docs/plan/pwa-parity/07-dashboard-parity-plan.md
git commit -m "document the phone dashboard surface"
```

---

## Verification checklist

Run before calling the unit done. A green suite is not verification.

- [ ] `bun run check && bun run test` from `apps/web`, both clean.
- [ ] Dev server at 390 × 844 in both themes: the quick-join bar is present; typing a name and pressing Go joins; pasting `http://localhost:7070/m/<room>` joins the same room; pasting `http://localhost:7070/dashboard` shows the error toast and does not navigate.
- [ ] Both chips filter the list, and the check appears on the active one only.
- [ ] A room joined but not owned appears under All, above an older owned room, with a Remove button and no Settings or Delete.
- [ ] Remove takes it out of the list and it does not return on reload.
- [ ] The same room does not appear twice when it is both owned and recently joined.
- [ ] At 800px wide the grid is one column and both dialogs use the phone width.
- [ ] Android emulator Chrome against the dev server, installed to the home screen: the bar clears the top safe area and the list scrolls under the bottom navigation.
- [ ] Before and after screenshots captured at 390 × 844 in both themes, on the base branch and on this one.

## Known leftovers

Not this unit's work; file as tracked issues when the unit lands.

- `Room` is declared twice, once in `routes/dashboard.index.tsx` and once in `RoomCard.tsx`, with `e2ee` required in one and optional in the other. They are structurally compatible, so this compiles, but there should be one type.
- The dashboard FAB uses `bg-primary` and `text-primary-foreground` where Android uses `primaryContainer` and `onPrimaryContainer`.
- `// TODO oncoming feature` at `dashboard.index.tsx:1` names nothing and predates this plan.
- `tsconfig.json` maps both `#/*` and `@/*` to `./src/*`, and files mix the two, sometimes within one
  import block. One alias should win.
- Pull-to-refresh, and the Admin tab with a phone layout for the admin tree, each deferred to their own unit by the spec.
