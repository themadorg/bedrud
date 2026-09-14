# 06 — Dashboard parity

Unit 6 of the PWA parity plan. Builds on unit 1, which set the shape scale and the single 1024px
phone breakpoint, and follows unit 5, which was the settings surface. Scope is the dashboard room
list: the quick-join bar, the filter control, the room list itself, and the breakpoints that tree
still uses.

The comparison this unit acts on was made by reading
`apps/android/.../screens/dashboard/DashboardScreen.kt` and `screens/main/MainScreen.kt` against
`apps/web/src/routes/dashboard.index.tsx` and `components/dashboard/MobileBottomNav.tsx`.

## The quick-join bar

### Today

`routes/dashboard.index.tsx:70` gives the quick-join row `max-md:hidden`, so it disappears below
768px. On a phone the web dashboard therefore offers no way to join a room by name or by a pasted
invite link. The only path into a room is tapping a card for a room you already own.

Android puts `QuickJoinBar` at the very top of the dashboard column, above the filter chips, at
every width. It is a text field and a Join button on one row.

### Target

The bar renders at every width, directly above the filter row, as the first element in the
dashboard column.

The field follows Android's input configuration, which exists because room slugs are lowercase and
have no spaces: `inputMode="url"`, `autoCapitalize="none"`, `autoCorrect="off"`,
`spellCheck={false}`, and submitting the form joins, which is the web reading of Android's
`ImeAction.Go`.

The Join button is disabled while the field is blank, matching Android's `enabled = value.isNotBlank()`,
rather than the current behaviour of hiding the button until text is typed. A control that appears
and disappears as you type moves the layout under your thumb.

The "New room" button stays beside the bar on desktop only. Phones already have the floating action
button in `MobileBottomNav`, which is exactly how Android does it — a FAB on the dashboard scaffold
and no create button in the bar.

### Pasted links have to work

Android resolves the field's contents through `BedrudURLParser.parseJoinInput`, which accepts a full
meeting URL on any host, a bare `/m/<room>` or `/c/<room>` path, or a plain room name.

The web does none of this. `dashboard.index.tsx:64` lowercases the input and replaces whitespace
with hyphens, so pasting `https://bedrud.example/m/team-standup` produces a room name of
`https://bedrud.example/m/team-standup` and the join fails. Since the bar is the phone's only way
into a room that is not already listed, and since pasting a shared link is the common case on a
phone, the parse is part of making the bar work rather than a separate feature.

A `parseJoinInput` helper in `apps/web/src/lib/` mirrors the Android rules, minus the multi-server
handling the web has no concept of:

- A string containing `/m/<segment>` or `/c/<segment>`, with or without a scheme or a port, yields
  that segment.
- Anything else is treated as a room name and slugified as it is today.
- A string that yields no usable segment returns null, and the bar reports that it is not a room
  rather than navigating to a broken URL.

This is pure string logic, so it is tested directly, in the repo's existing style.

## Filter chips replace the tabs

### Today

The dashboard has a `Tabs` control with two triggers, "My Rooms" and "Recent", each carrying a
count, and a separate free-text "Filter…" input to its right. Choosing a tab swaps the panel
beneath: My Rooms renders a grid of `RoomCard`, Recent renders a list of `RecentRoomRow`, which is
a different component with a different shape.

Android has a `FilterRow` of two Material 3 filter chips, All and My Rooms, with a leading check
icon on the selected chip only. There is no text filter. Recents are not a separate view: under the
All chip they are merged into the single room list.

### Target

Two chips, All and My Rooms, with a leading check on the active one. The chips replace the tabs at
every width, so there is one filter control and one implementation rather than a phone control and
a desktop control that drift apart.

Chips use the `rounded-sm` token unit 1 assigned to them.

### One merged list

Under the All chip the list holds both server rooms and rooms known only from local history,
ordered by Android's rule (`DashboardScreen.kt:538`), simplified for a single server and for the
web's `RecentRoom`, which carries only a name and a `joinedAt`:

1. Recents whose name does not appear in the API list become recent-only entries, dated by
   `joinedAt`.
2. Server rooms whose name does appear in the recents are dated by that same `joinedAt`.
3. Those two groups are sorted together, most recent first.
4. Server rooms with no local history follow, in the order the server returned them.

Under the My Rooms chip the list holds server rooms only, in server order.

The ordering is pure logic over two arrays, so it is tested directly, including the case that makes
the rule worth writing down: a room that is both owned and recently joined appears once, in its
recency position, not twice.

### The free-text filter stays

The "Filter…" input stays, and narrows whatever the active chip selected.

This is a deliberate departure from Android, which has no equivalent. Removing a working search
from a list that can hold any number of rooms would be a regression for the sake of a match, and
the parity goal is the same shapes and placement, not the same feature set. The chips carry the
parity; the filter box is the web's own.

## The card for a recent-only room

Merging the lists means one card component renders two kinds of entry, so `RecentRoomRow`
disappears and `RoomCard` covers both.

A recent-only entry has a name and a `joinedAt` and nothing else. There is no server record behind
it, so there is no capacity, no public or private state, no capability set, and no ownership. Its
card therefore renders:

- the room name, and the existing `timeAgo` label, which is the web's reading of Android's presence
  line;
- Join, and Copy link, both of which need only the name;
- Remove, in the position the owner-only Settings and Delete buttons occupy on a server-backed card.

Server-backed cards are unchanged.

This is the second deliberate departure. Android's cards are minimal — a title, a presence label and
a trailing chevron — while the web's card carries badges, a capability row, a copy-link button and
an inline delete confirmation. Flattening the web card to Android's would strip working affordances
from every room on the dashboard. The merged list carries the parity; the card keeps its contents.

## Breakpoints

The dashboard tree never got unit 1's breakpoint. It still branches at Tailwind's defaults in nine
places:

| File | Prefixes |
|---|---|
| `routes/dashboard.index.tsx` | `max-md:hidden`, `md:block`, `sm:grid-cols-2`, `sm:inline`, `sm:opacity-0`, `sm:transition-opacity`, `sm:group-hover:` |
| `components/dashboard/CreateRoomDialog.tsx` | `sm:max-w-md` |
| `components/dashboard/RoomSettingsDialog.tsx` | `sm:max-w-sm` |

All of them move to the 1024px line, except where the change is subsumed by other work in this unit:
`max-md:hidden` goes away with the quick-join bar becoming always-visible, and the three `sm:`
hover-reveal prefixes go away with `RecentRoomRow`.

The card grid is the one judgement call. It reads `grid-cols-1 sm:grid-cols-2 lg:grid-cols-3
xl:grid-cols-4` today, so it reaches two columns at 640px — a tablet-portrait layout, which unit 1
decided should be the phone layout instead.

Moving only the `sm:` step to `lg:` would put two rules at the same breakpoint, and `cn` keeps the
last, so the two-column step would silently vanish. The whole progression shifts up one stop
instead:

```
grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4
```

A portrait tablet then gets one column of cards, as a phone does and as Android does, and each
desktop width keeps a column count it can fill.

The "New room" button beside the quick-join bar becomes `max-lg:hidden` rather than inheriting the
row's old wrapper, and its `sm:inline` label prefix goes away with it: the button is desktop-only
now, so the label is never hidden.

## Out of the nav

The Android bottom navigation adds an Admin tab for admins (`MainScreen.kt:95`); the web's
`MobileBottomNav` has Rooms, Profile and Settings only, though it already reads `user?.isAdmin` for
the create dialog.

The tab is deliberately **not** added here. `routes/dashboard/admin.tsx` and the six routes beneath
it have no phone handling of any kind — no gate, no `lg:` branches — so a tab added now would put a
navigation item on phones that lands on an unadapted desktop screen. The tab and a phone layout for
the admin tree belong to one later unit, and ship together.

## Tests

- `parseJoinInput` extracts the room from a full meeting URL, from a bare `/m/` path, from a `/c/`
  path, from a URL carrying a port, and returns a slug for a plain name and null for input with no
  usable segment.
- The merged ordering puts a recent-only room and a recently joined owned room in recency order,
  appends never-joined owned rooms in server order, and emits a room that is both owned and recent
  exactly once.
- The My Rooms chip yields server rooms only.
- The quick-join bar carries no breakpoint prefix that could hide it, asserted the way the existing
  token tests read a component's class strings.
- The dashboard tree carries no `sm:`, `max-sm:`, `md:` or `max-md:` prefix.

Each test is written before the change it covers and seen failing first.

## Verification on a device

- Playwright at 390 × 844, both themes, before and after: the quick-join bar is present and joins
  from a pasted link, the chips filter the list, and a recent-only room and an owned room appear in
  one list in recency order.
- The same at 800px wide, to prove the grid and the two dialogs now use the phone layout there.
- Android emulator Chrome against the dev server, installed to the home screen, to confirm the bar
  clears the top safe area and the list scrolls under the bottom navigation.

## Out of scope, tracked separately

- Pull-to-refresh. Android wraps the list in a `PullToRefreshBox`; the web refetches on mount and
  nothing else. It is a new gesture with its own edge cases and earns its own unit.
- The Admin tab and a phone layout for the admin tree, as above.
- The dashboard FAB's colour role: the web uses `bg-primary` and `text-primary-foreground` where
  Android uses `primaryContainer` and `onPrimaryContainer`. Same shape, different token.
- The `// TODO oncoming feature` marker at `dashboard.index.tsx:1`, which names nothing and predates
  this plan.
