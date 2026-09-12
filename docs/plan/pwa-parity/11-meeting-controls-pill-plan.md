# 11 — Meeting controls pill: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Below 1024px, replace the nine-control meeting bar and its two full-screen dialogs with
the Android controls pill: five controls on one bottom-anchored surface whose handle unfolds the
room options upward inside that same surface.

**Architecture:** Four presentational components under `components/meeting/`, two pure modules
carrying the decisions that are neither CSS's nor a library's, and a phone branch in `ControlsBar`
that renders the pill instead of the bar. The pure modules are the only new logic; everything else
is markup over state `ControlsBar` already holds.

**Tech Stack:** React 19, TanStack Start, TailwindCSS v4, shadcn/ui, lucide-react, Vitest 4, Bun,
Biome.

The spec is [10 — Meeting controls pill](./10-meeting-controls-pill.md). Read it before Task 1; the
values below are copied from it and it explains why each one is what it is.

## Global Constraints

- Branch is `feat/pwa-parity-controls-pill`, cut from `feat/pwa-parity-foundation`. **The PR targets
  `feat/pwa-parity-foundation`, not `master`.**
- The phone breakpoint is 1024px, from `MOBILE_BREAKPOINT_PX` in `apps/web/src/lib/use-is-mobile.ts`.
  Use `useIsMobile()`; never write a second breakpoint.
- Verify command, run from `apps/web`: `bun run check && bun run test`. `check` is
  `biome check . && tsc --noEmit`.
- Baseline before Task 1: **58 test files, 300 tests, all passing.** `check` reports three
  pre-existing `noDocumentCookie` warnings in `api.test.ts`; they are not yours.
- Path alias: import with `@/…`. Both `#/*` and `@/*` map to `./src/*`, and this unit's area uses
  `@/`.
- `cn` is `twMerge(clsx(…))`. A second `rounded-*` on one element **silently deletes the first**.
  One corner class per element.
- Test house style: every test file is `.test.ts`, never `.test.tsx`. Descriptions are imperative
  and start with "should". There are no component render tests in this repo and this unit adds
  none — components are covered by reading their source text.
- No abbreviated identifiers, including callback parameters and loop variables. `index`, not `i`.
- Comments sit above what they describe, never trailing. Complete sentences, present tense, third
  person. They explain why.
- Private helpers are defined textually above the code that calls them.
- Commit style on this branch family is `<action> <what> for <why>` with the actions `add`,
  `update`, `delete` — the format in the repo's root `AGENTS.md` and the one the foundation branch
  uses. Do **not** use master's `type(scope):` prefixes here.
- No attribution of any kind in commit messages.

**Values fixed by the spec, to be used verbatim:**

| Thing | Value |
|---|---|
| Surface corner | `rounded-3xl` |
| Surface inset | `inset-x-2`, bottom `calc(12px + env(safe-area-inset-bottom, 0px))` |
| Handle | 32 × 4px pill in a 48px tap target |
| Drag threshold | 24px |
| Expand / collapse duration | 320ms / 260ms |
| Easing | `cubic-bezier(0.4, 0, 0.2, 1)` |
| Scrim | `--meet-scrim`, `rgba(23, 23, 23, 0.32)` light, `rgba(0, 0, 0, 0.32)` dark |
| Mic pill max width | 136px |
| Mic pill labels | `Speak`, `Muted`, `Push to Talk`, `Talking…` |

---

## File structure

| File | Responsibility | Task |
|---|---|---|
| `src/components/meeting/controlsPanelDrag.ts` | a finished drag plus the current state to the next state | 1 |
| `src/components/meeting/controlsPanelDrag.test.ts` | its tests | 1 |
| `src/components/meeting/meetingOptionRows.ts` | capability flags to the ordered row list | 2 |
| `src/components/meeting/meetingOptionRows.test.ts` | its tests | 2 |
| `src/components/meeting/meeting.css` | two new tokens, two retuned | 3 |
| `src/components/meeting/meetingShapeTokens.test.ts` | pins all four | 3 |
| `src/components/meeting/MeetingMicPill.tsx` | the centre slot, both input modes | 4 |
| `src/components/meeting/MeetingCallControlsRow.tsx` | the five controls | 5 |
| `src/components/meeting/MeetingOptionsPanel.tsx` | the rows, their scroll, their close rule | 6 |
| `src/components/meeting/MeetingControlsPill.tsx` | the surface: handle, scrim, gesture, state | 7 |
| `src/components/meeting/controlsPill.test.ts` | source-text assertions across the above | 7 |
| `src/components/meeting/ControlsBar.tsx` | phone branch; two dialogs deleted | 7 |
| `src/components/meeting/MeetingPanels.tsx` | chat props down, phone chat toggle hidden | 8 |
| `src/components/meeting/MeetingControls.tsx` | passes the chat props through | 8 |
| `apps/web/AGENTS.md`, `docs/plan/pwa-parity/01-overview-and-units.md` | docs | 9 |

---

## Task 1: The drag rule

**Files:**
- Create: `apps/web/src/components/meeting/controlsPanelDrag.ts`
- Test: `apps/web/src/components/meeting/controlsPanelDrag.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `PANEL_DRAG_THRESHOLD_PX: number`, and
  `expandedAfterDrag(expanded: boolean, deltaY: number): boolean`. Task 7 calls it on drag end.

`deltaY` is the pointer's total vertical travel in CSS pixels, negative upward, matching a
`PointerEvent`'s coordinate system. Up opens, down closes, so the panel answers the same gesture in
both directions.

- [ ] **Step 1: Write the failing test**

`apps/web/src/components/meeting/controlsPanelDrag.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { expandedAfterDrag, PANEL_DRAG_THRESHOLD_PX } from './controlsPanelDrag'

describe('expandedAfterDrag', () => {
  it('should expand when a collapsed panel is dragged up past the threshold', () => {
    expect(expandedAfterDrag(false, -(PANEL_DRAG_THRESHOLD_PX + 1))).toBe(true)
  })

  it('should collapse when an expanded panel is dragged down past the threshold', () => {
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX + 1)).toBe(false)
  })

  it('should ignore a drag shorter than the threshold in either direction', () => {
    expect(expandedAfterDrag(false, -(PANEL_DRAG_THRESHOLD_PX - 1))).toBe(false)
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX - 1)).toBe(true)
  })

  it('should ignore a drag exactly at the threshold', () => {
    expect(expandedAfterDrag(false, -PANEL_DRAG_THRESHOLD_PX)).toBe(false)
    expect(expandedAfterDrag(true, PANEL_DRAG_THRESHOLD_PX)).toBe(true)
  })

  it('should leave the panel alone when dragged further in the direction it already sits', () => {
    expect(expandedAfterDrag(true, -(PANEL_DRAG_THRESHOLD_PX + 1))).toBe(true)
    expect(expandedAfterDrag(false, PANEL_DRAG_THRESHOLD_PX + 1)).toBe(false)
  })

  it('should ignore a drag that did not move', () => {
    expect(expandedAfterDrag(false, 0)).toBe(false)
    expect(expandedAfterDrag(true, 0)).toBe(true)
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bun run test controlsPanelDrag
```

Expected: `Failed to resolve import "./controlsPanelDrag"`.

- [ ] **Step 3: Write the module**

`apps/web/src/components/meeting/controlsPanelDrag.ts`:

```ts
/**
 * How far a vertical drag must travel before it moves the panel. Android's
 * `Dimens.meetingHandleSwipeThreshold`.
 */
export const PANEL_DRAG_THRESHOLD_PX = 24

/**
 * Resolves a finished vertical drag against the panel's current state. `deltaY` is the pointer's
 * total travel, negative upward. Returns the state the panel should be in, which equals the state
 * it was already in when the drag was too short, or pushed it further in the direction it already
 * sits.
 */
export function expandedAfterDrag(expanded: boolean, deltaY: number): boolean {
  if (deltaY < -PANEL_DRAG_THRESHOLD_PX) return true
  if (deltaY > PANEL_DRAG_THRESHOLD_PX) return false
  return expanded
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd apps/web && bun run test controlsPanelDrag && bun run check
```

Expected: 6 passed, then `check` exit 0. Run `check` before every commit, not only at the end of a
task that touches components — Biome's import ordering and formatting are errors, not warnings, and
a test-only task can still break them.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/meeting/controlsPanelDrag.ts apps/web/src/components/meeting/controlsPanelDrag.test.ts
git commit -m "add the controls panel's drag rule for a handle that opens and closes"
```

---

## Task 2: The option rows

**Files:**
- Create: `apps/web/src/components/meeting/meetingOptionRows.ts`
- Test: `apps/web/src/components/meeting/meetingOptionRows.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `MeetingOptionRowId`, `MeetingOptionRow`, `MeetingOptionsInput`, and
  `meetingOptionRows(input: MeetingOptionsInput): MeetingOptionRow[]`. Task 6 renders the result
  and maps each `id` to an icon and a handler.

This lifts the `moreRows` `useMemo` out of `ControlsBar.tsx` (currently lines 437–587). The rows
carry no icons and no callbacks on purpose: those are React values and would make the module
untestable in Node, which is the point of splitting it out.

**Order is part of the contract** — room, then audio, then stage — and is asserted.

- [ ] **Step 1: Write the failing test**

`apps/web/src/components/meeting/meetingOptionRows.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { meetingOptionRows, type MeetingOptionsInput } from './meetingOptionRows'

/** Everything off, so each test turns on only what it is about. */
const nothingAvailable: MeetingOptionsInput = {
  videoSidebarAvailable: false,
  videoSidebarOpen: false,
  roomAccessAvailable: false,
  isPublic: false,
  roomId: undefined,
  linkCopied: false,
  isSelfDeafened: false,
  fullscreenAvailable: false,
  isFullscreen: false,
  whiteboardEnabled: false,
  isWhiteboardOnStage: false,
  isWhiteboardHost: false,
  youtubeEnabled: false,
  isYoutubeOnStage: false,
  isYoutubeHost: false,
  webxdcEnabled: false,
  isWebxdcOnStage: false,
  stageTakenByOther: false,
}

/** Every capability on, for the order and the full-length cases. */
const everythingAvailable: MeetingOptionsInput = {
  ...nothingAvailable,
  videoSidebarAvailable: true,
  roomAccessAvailable: true,
  roomId: 'room-1',
  fullscreenAvailable: true,
  whiteboardEnabled: true,
  youtubeEnabled: true,
  webxdcEnabled: true,
}

function idsOf(input: MeetingOptionsInput): string[] {
  return meetingOptionRows(input).map((row) => row.id)
}

describe('meetingOptionRows', () => {
  it('should always offer the link, the audio rows and settings', () => {
    expect(idsOf(nothingAvailable)).toEqual(['copy-link', 'deafen', 'audio-devices', 'noise', 'settings'])
  })

  it('should order the rows room, then audio, then stage', () => {
    expect(idsOf(everythingAvailable)).toEqual([
      'videos',
      'access',
      'info',
      'copy-link',
      'deafen',
      'audio-devices',
      'noise',
      'settings',
      'fullscreen',
      'whiteboard',
      'youtube',
      'app-gallery',
    ])
  })

  it('should omit each optional row when its capability is absent', () => {
    expect(idsOf(nothingAvailable)).not.toContain('videos')
    expect(idsOf(nothingAvailable)).not.toContain('access')
    expect(idsOf(nothingAvailable)).not.toContain('info')
    expect(idsOf(nothingAvailable)).not.toContain('fullscreen')
    expect(idsOf(nothingAvailable)).not.toContain('whiteboard')
    expect(idsOf(nothingAvailable)).not.toContain('youtube')
    expect(idsOf(nothingAvailable)).not.toContain('app-gallery')
  })

  it('should mark deafen and videos as toggles and everything else as actions', () => {
    const byId = new Map(meetingOptionRows(everythingAvailable).map((row) => [row.id, row.kind]))
    expect(byId.get('deafen')).toBe('toggle')
    expect(byId.get('videos')).toBe('toggle')
    expect(byId.get('copy-link')).toBe('action')
    expect(byId.get('settings')).toBe('action')
    expect(byId.get('whiteboard')).toBe('action')
  })

  it('should check the deafen toggle while deafened', () => {
    const rows = meetingOptionRows({ ...nothingAvailable, isSelfDeafened: true })
    expect(rows.find((row) => row.id === 'deafen')?.checked).toBe(true)
    expect(meetingOptionRows(nothingAvailable).find((row) => row.id === 'deafen')?.checked).toBe(false)
  })

  it('should name the room access row for the state the room is in', () => {
    const publicRows = meetingOptionRows({ ...everythingAvailable, isPublic: true })
    expect(publicRows.find((row) => row.id === 'access')?.label).toBe('Public room')
    const privateRows = meetingOptionRows({ ...everythingAvailable, isPublic: false })
    expect(privateRows.find((row) => row.id === 'access')?.label).toBe('Private room')
  })

  it('should name the video row for the action it performs, not the state', () => {
    const openRows = meetingOptionRows({ ...everythingAvailable, videoSidebarOpen: true })
    expect(openRows.find((row) => row.id === 'videos')?.label).toBe('Hide videos')
    const closedRows = meetingOptionRows({ ...everythingAvailable, videoSidebarOpen: false })
    expect(closedRows.find((row) => row.id === 'videos')?.label).toBe('Show videos')
  })

  it('should say the link was copied for as long as the caller reports it', () => {
    const rows = meetingOptionRows({ ...nothingAvailable, linkCopied: true })
    expect(rows.find((row) => row.id === 'copy-link')?.label).toBe('Copied!')
  })

  it('should offer to close a stage feature it is hosting and to open one it is not', () => {
    const hosting = meetingOptionRows({
      ...everythingAvailable,
      isWhiteboardOnStage: true,
      isWhiteboardHost: true,
    })
    expect(hosting.find((row) => row.id === 'whiteboard')?.label).toBe('Close whiteboard')
    expect(meetingOptionRows(everythingAvailable).find((row) => row.id === 'whiteboard')?.label).toBe(
      'Open whiteboard',
    )
  })

  it('should disable a stage feature while someone else holds the stage', () => {
    const rows = meetingOptionRows({ ...everythingAvailable, stageTakenByOther: true })
    expect(rows.find((row) => row.id === 'whiteboard')?.disabled).toBe(true)
    expect(rows.find((row) => row.id === 'youtube')?.disabled).toBe(true)
    expect(rows.find((row) => row.id === 'copy-link')?.disabled).toBe(false)
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bun run test meetingOptionRows
```

Expected: `Failed to resolve import "./meetingOptionRows"`.

- [ ] **Step 3: Write the module**

`apps/web/src/components/meeting/meetingOptionRows.ts`:

```ts
/** Every row the options panel can show. The union is the contract Task 6 maps to icons. */
export type MeetingOptionRowId =
  | 'videos'
  | 'access'
  | 'info'
  | 'copy-link'
  | 'deafen'
  | 'audio-devices'
  | 'noise'
  | 'settings'
  | 'fullscreen'
  | 'whiteboard'
  | 'youtube'
  | 'app-gallery'

export interface MeetingOptionRow {
  id: MeetingOptionRowId
  label: string
  /** Toggles keep the panel open and carry a check. Actions close it on the way. */
  kind: 'toggle' | 'action'
  checked: boolean
  disabled: boolean
}

export interface MeetingOptionsInput {
  videoSidebarAvailable: boolean
  videoSidebarOpen: boolean
  roomAccessAvailable: boolean
  isPublic: boolean
  roomId: string | undefined
  linkCopied: boolean
  isSelfDeafened: boolean
  fullscreenAvailable: boolean
  isFullscreen: boolean
  whiteboardEnabled: boolean
  isWhiteboardOnStage: boolean
  isWhiteboardHost: boolean
  youtubeEnabled: boolean
  isYoutubeOnStage: boolean
  isYoutubeHost: boolean
  webxdcEnabled: boolean
  isWebxdcOnStage: boolean
  stageTakenByOther: boolean
}

/** Fills in the two fields most rows do not care about. */
function action(id: MeetingOptionRowId, label: string, disabled = false): MeetingOptionRow {
  return { id, label, kind: 'action', checked: false, disabled }
}

/** A row whose tap flips a setting in place rather than going somewhere. */
function toggle(id: MeetingOptionRowId, label: string, checked: boolean): MeetingOptionRow {
  return { id, label, kind: 'toggle', checked, disabled: false }
}

/**
 * Builds the options panel's rows from what this room and this client can do. The order — room,
 * then audio, then whatever is on the stage — is part of the contract: the panel is anchored to
 * the controls and read bottom-up, so the rows nearest the thumb are the ones about the room the
 * user is in.
 */
export function meetingOptionRows(input: MeetingOptionsInput): MeetingOptionRow[] {
  const rows: MeetingOptionRow[] = []

  if (input.videoSidebarAvailable) {
    rows.push(toggle('videos', input.videoSidebarOpen ? 'Hide videos' : 'Show videos', input.videoSidebarOpen))
  }
  if (input.roomAccessAvailable) {
    rows.push(action('access', input.isPublic ? 'Public room' : 'Private room'))
  }
  if (input.roomId) {
    rows.push(action('info', 'Room info'))
  }

  rows.push(action('copy-link', input.linkCopied ? 'Copied!' : 'Copy room link'))
  rows.push(toggle('deafen', input.isSelfDeafened ? 'Undeafen' : 'Deafen', input.isSelfDeafened))
  rows.push(action('audio-devices', 'Audio settings'))
  rows.push(action('noise', 'Noise suppression'))
  rows.push(action('settings', 'Settings'))

  if (input.fullscreenAvailable) {
    rows.push(action('fullscreen', input.isFullscreen ? 'Exit fullscreen' : 'Fullscreen'))
  }

  const hostsWhiteboard = input.isWhiteboardOnStage && input.isWhiteboardHost
  if (hostsWhiteboard) {
    rows.push(action('whiteboard', 'Close whiteboard'))
  } else if (input.whiteboardEnabled) {
    rows.push(
      action('whiteboard', input.isWhiteboardOnStage ? 'Whiteboard on stage' : 'Open whiteboard', input.stageTakenByOther),
    )
  }

  const hostsYoutube = input.isYoutubeOnStage && input.isYoutubeHost
  if (hostsYoutube) {
    rows.push(action('youtube', 'Stop YouTube'))
  } else if (input.youtubeEnabled) {
    rows.push(
      action('youtube', input.isYoutubeOnStage ? 'YouTube on stage' : 'Share YouTube', input.stageTakenByOther),
    )
  }

  if (input.webxdcEnabled) {
    rows.push(action('app-gallery', input.isWebxdcOnStage ? 'Gallery (on stage)' : 'App gallery'))
  }

  return rows
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd apps/web && bun run test meetingOptionRows && bun run check
```

Expected: 10 passed, then `check` exit 0.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/meeting/meetingOptionRows.ts apps/web/src/components/meeting/meetingOptionRows.test.ts
git commit -m "add the meeting options row list for one panel instead of a menu"
```

---

## Task 3: The tokens

**Files:**
- Modify: `apps/web/src/components/meeting/meeting.css` — the `:root` block near line 27 and the
  dark block near line 126
- Test: `apps/web/src/components/meeting/meetingShapeTokens.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: CSS custom properties `--meet-scrim` and `--meet-controls-panel-max-height`; retuned
  `--meet-controls-bar-radius` and `--meet-btn-leave-radius`. Tasks 4 to 7 use all four.

`--meet-chat-bubble-radius` and `--meet-chat-bubble-radius-near` are defined as
`var(--meet-controls-bar-radius)` and `var(--meet-btn-leave-radius)` and follow them automatically.
Do not touch those two lines.

Read `meeting.css` before editing to confirm the current line numbers — the file is long and other
units are editing it on their own branches.

- [ ] **Step 1: Write the failing test**

`apps/web/src/components/meeting/meetingShapeTokens.test.ts`:

```ts
// @vitest-environment node
//
// This file reads a stylesheet from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingCss = readFileSync(new URL('./meeting.css', import.meta.url), 'utf8')

/** Reads the first `--name: value;` declaration from a stylesheet. */
function tokenValue(css: string, name: string): string | undefined {
  return new RegExp(`${name}:\\s*([^;]+);`).exec(css)?.[1].trim()
}

describe('meeting shape tokens', () => {
  it('should give the controls bar the Android xxl corner', () => {
    expect(tokenValue(meetingCss, '--meet-controls-bar-radius')).toBe('28px')
  })

  it('should make the leave button a pill', () => {
    expect(tokenValue(meetingCss, '--meet-btn-leave-radius')).toBe('9999px')
  })

  it('should leave the chat bubble radii following the two above', () => {
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius')).toBe('var(--meet-controls-bar-radius)')
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius-near')).toBe('var(--meet-btn-leave-radius)')
  })
})

describe('meeting panel tokens', () => {
  it('should define a scrim for the options panel', () => {
    expect(tokenValue(meetingCss, '--meet-scrim')).toBe('rgba(23, 23, 23, 0.32)')
  })

  it('should cap the options panel below the status bar', () => {
    expect(tokenValue(meetingCss, '--meet-controls-panel-max-height')).toBe(
      'calc(var(--app-height, 100svh) - 12px - env(safe-area-inset-top, 0px))',
    )
  })

  it('should darken the scrim in the dark theme', () => {
    // The light value is read first by `tokenValue`, so the dark override is checked by hand.
    expect(meetingCss).toContain('--meet-scrim: rgba(0, 0, 0, 0.32);')
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bun run test meetingShapeTokens
```

Expected: five failures. `--meet-controls-bar-radius` reads `4px`, `--meet-btn-leave-radius` reads
`3px`, and the two new tokens are `undefined`. The chat bubble assertions already pass.

- [ ] **Step 3: Retune the two radius tokens**

In the `:root` block of `apps/web/src/components/meeting/meeting.css`, replace:

```css
  --meet-controls-bar-radius: 4px;
  --meet-btn-leave-radius: 3px;
```

with:

```css
  /* Android BedrudShapeTokens.controlsBar — the xxl corner, on every width. */
  --meet-controls-bar-radius: 28px;
  /* Android MeetEndCallButton uses the pill shape. */
  --meet-btn-leave-radius: 9999px;
```

- [ ] **Step 4: Add the two new tokens**

In the same `:root` block, directly below the two lines above:

```css
  /* Dims the call while the options panel is open. Android's ScrimAlpha. */
  --meet-scrim: rgba(23, 23, 23, 0.32);
  /* The options panel's ceiling: the visible viewport less the status bar and Android's space12. */
  --meet-controls-panel-max-height: calc(var(--app-height, 100svh) - 12px - env(safe-area-inset-top, 0px));
```

In the dark block — the one that redefines `--meet-chrome: rgba(12, 12, 22, 0.9)` — add:

```css
  --meet-scrim: rgba(0, 0, 0, 0.32);
```

`--meet-controls-panel-max-height` is not redefined in the dark block; it carries no colour.

- [ ] **Step 5: Run it and watch it pass**

```bash
cd apps/web && bun run test meetingShapeTokens
```

Expected: 6 passed.

- [ ] **Step 6: Check the whole suite, because these tokens are shared**

```bash
cd apps/web && bun run check && bun run test
```

Expected: 61 files, 322 tests, 0 failures. The desktop controls bar and the chat bubbles now render
with new corners; nothing asserts their old values, so nothing else should move.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/components/meeting/meeting.css apps/web/src/components/meeting/meetingShapeTokens.test.ts
git commit -m "update the meeting radius tokens to the Android scale for one set of corners"
```

---

## Task 4: The mic pill

**Files:**
- Create: `apps/web/src/components/meeting/MeetingMicPill.tsx`

**Interfaces:**
- Consumes: `--meet-control`, `--meet-btn-alert-bg`, `--meet-btn-muted-bg`, `--meet-border` from
  Task 3's file.
- Produces:

```tsx
export function MeetingMicPill(props: {
  pushToTalk: boolean
  micOpen: boolean
  transmitting: boolean
  available: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
}): React.JSX.Element
```

Android's pill also carries an error badge and a status ring. Neither is here: nothing in
`ControlsBar` produces a microphone error state today, and a prop no caller can fill is a stub.

Task 5 renders it in the centre slot.

`micOpen` is whether the microphone is actually open; `transmitting` is only ever true in
push-to-talk mode while the key is held.

The pill keeps one width across every mode and state. All four labels render stacked in one grid
cell with three of them `invisible`, so the pill sizes to the widest — nothing in the bar may move
when the mode or the hold state changes.

There is no test step. This is markup, the repo has no component render tests, and Task 7's
source-text test asserts the four labels are present.

- [ ] **Step 1: Write the component**

`apps/web/src/components/meeting/MeetingMicPill.tsx`:

```tsx
import { Mic, MicOff } from 'lucide-react'
import { cn } from '@/lib/utils'

/** Every label the pill can show, longest first so the invisible stack is easy to read. */
const MIC_PILL_LABELS = ['Push to Talk', 'Talking…', 'Muted', 'Speak'] as const

/** Android's Dimens.meetingMicPillMaxWidth: the sides never squeeze at this width. */
const MIC_PILL_MAX_WIDTH_CLASS = 'max-w-[136px]'

interface MeetingMicPillProps {
  pushToTalk: boolean
  micOpen: boolean
  transmitting: boolean
  available: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
}

/**
 * The centre slot, one pill for both input modes so the bar's rhythm never changes. Voice activity
 * renders filled and toggles on tap; push to talk renders as an outline until held, when it fills.
 */
export function MeetingMicPill({
  pushToTalk,
  micOpen,
  transmitting,
  available,
  onToggleMic,
  onPushToTalkChange,
}: MeetingMicPillProps) {
  const label = transmitting ? 'Talking…' : pushToTalk ? 'Push to Talk' : micOpen ? 'Speak' : 'Muted'

  const container = transmitting
    ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
    : pushToTalk
      ? 'bg-transparent border border-[var(--meet-border)] text-[var(--meet-fg-muted)]'
      : micOpen
        ? 'bg-[var(--meet-control)] text-[var(--meet-control-fg)]'
        : 'bg-[var(--meet-btn-alert-bg)] text-[var(--meet-btn-alert-fg)]'

  // Push to talk is held, voice activity is tapped, so the two modes bind different handlers to
  // the same element rather than rendering two pills.
  const gestureProps = pushToTalk
    ? {
        onPointerDown: (event: React.PointerEvent<HTMLButtonElement>) => {
          if (!available) return
          event.currentTarget.setPointerCapture(event.pointerId)
          onPushToTalkChange(true)
        },
        onPointerUp: (event: React.PointerEvent<HTMLButtonElement>) => {
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId)
          }
          onPushToTalkChange(false)
        },
        onPointerLeave: () => onPushToTalkChange(false),
        onPointerCancel: () => onPushToTalkChange(false),
      }
    : { onClick: onToggleMic }

  return (
    <button
      type="button"
      aria-label={pushToTalk ? 'Push to talk' : micOpen ? 'Mute microphone' : 'Unmute microphone'}
      className={cn(
        'flex h-12 shrink-0 items-center justify-center gap-2 rounded-full px-3 transition-[background,color,transform] duration-150 active:scale-[0.96]',
        MIC_PILL_MAX_WIDTH_CLASS,
        container,
        !available && 'cursor-not-allowed opacity-40',
      )}
      {...gestureProps}
    >
      {micOpen || transmitting ? <Mic size={18} className="shrink-0" /> : <MicOff size={18} className="shrink-0" />}
      {/* Every label occupies the same cell so the pill is as wide as its longest one, always. */}
      <span className="grid">
        {MIC_PILL_LABELS.map((candidate) => (
          <span
            key={candidate}
            aria-hidden={candidate !== label}
            className={cn(
              'col-start-1 row-start-1 whitespace-nowrap text-[13px] font-medium',
              candidate !== label && 'invisible',
            )}
          >
            {candidate}
          </span>
        ))}
      </span>
    </button>
  )
}
```

- [ ] **Step 2: Check it compiles and is formatted**

```bash
cd apps/web && bun run check
```

Expected: exit 0, three pre-existing `noDocumentCookie` warnings.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/meeting/MeetingMicPill.tsx
git commit -m "add the meeting mic pill for one centre slot across both input modes"
```

---

## Task 5: The controls row

**Files:**
- Create: `apps/web/src/components/meeting/MeetingCallControlsRow.tsx`

**Interfaces:**
- Consumes: `MeetingMicPill` from Task 4, with exactly the props listed there.
- Produces:

```tsx
export function MeetingCallControlsRow(props: {
  cameraEnabled: boolean
  onToggleCamera: () => void
  screenShareEnabled: boolean
  screenShareAvailable: boolean
  onToggleScreenShare: () => void
  micPushToTalk: boolean
  micOpen: boolean
  micTransmitting: boolean
  micAvailable: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
  chatOpen: boolean
  unreadCount: number
  onToggleChat: () => void
  onLeave: () => void
}): React.JSX.Element
```

Task 7 renders it as the surface's floor.

Three sections with equal-weight sides keep the mic slot centred under the handle whatever its
label says.

- [ ] **Step 1: Write the component**

`apps/web/src/components/meeting/MeetingCallControlsRow.tsx`:

```tsx
import { MessageSquare, MonitorOff, MonitorUp, PhoneOff, Video, VideoOff } from 'lucide-react'
import { MeetingMicPill } from '@/components/meeting/MeetingMicPill'
import { cn } from '@/lib/utils'

/** The circular side controls. 48px is the accessibility floor for a thumb. */
const SIDE_BUTTON_CLASS =
  'flex h-12 w-12 shrink-0 items-center justify-center rounded-full border-none transition-colors duration-150'

const SIDE_BUTTON_RESTING = 'bg-[var(--meet-control)] text-[var(--meet-control-fg)]'
const SIDE_BUTTON_ACTIVE = 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
const SIDE_BUTTON_ALERT = 'bg-[var(--meet-btn-alert-bg)] text-[var(--meet-btn-alert-fg)]'

interface MeetingCallControlsRowProps {
  cameraEnabled: boolean
  onToggleCamera: () => void
  screenShareEnabled: boolean
  screenShareAvailable: boolean
  onToggleScreenShare: () => void
  micPushToTalk: boolean
  micOpen: boolean
  micTransmitting: boolean
  micAvailable: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
  chatOpen: boolean
  unreadCount: number
  onToggleChat: () => void
  onLeave: () => void
}

/**
 * The five call controls: the row that sits at the foot of the pill, collapsed or expanded. The
 * side clusters carry equal weight so the mic slot stays centred under the handle however its
 * label changes.
 */
export function MeetingCallControlsRow({
  cameraEnabled,
  onToggleCamera,
  screenShareEnabled,
  screenShareAvailable,
  onToggleScreenShare,
  micPushToTalk,
  micOpen,
  micTransmitting,
  micAvailable,
  onToggleMic,
  onPushToTalkChange,
  chatOpen,
  unreadCount,
  onToggleChat,
  onLeave,
}: MeetingCallControlsRowProps) {
  return (
    <div className="flex w-full items-center gap-2 px-3 pb-3">
      <div className="flex flex-1 items-center justify-start gap-2">
        <button
          type="button"
          onClick={onToggleCamera}
          aria-label={cameraEnabled ? 'Disable camera' : 'Enable camera'}
          className={cn(SIDE_BUTTON_CLASS, cameraEnabled ? SIDE_BUTTON_RESTING : SIDE_BUTTON_ALERT)}
        >
          {cameraEnabled ? <Video size={18} /> : <VideoOff size={18} />}
        </button>
        <button
          type="button"
          onClick={screenShareAvailable ? onToggleScreenShare : undefined}
          aria-label={screenShareEnabled ? 'Stop sharing' : 'Share screen'}
          className={cn(
            SIDE_BUTTON_CLASS,
            screenShareEnabled ? SIDE_BUTTON_ALERT : SIDE_BUTTON_RESTING,
            !screenShareAvailable && 'cursor-not-allowed opacity-40',
          )}
        >
          {screenShareEnabled ? <MonitorOff size={17} /> : <MonitorUp size={17} />}
        </button>
      </div>

      <MeetingMicPill
        pushToTalk={micPushToTalk}
        micOpen={micOpen}
        transmitting={micTransmitting}
        available={micAvailable}
        onToggleMic={onToggleMic}
        onPushToTalkChange={onPushToTalkChange}
      />

      <div className="flex flex-1 items-center justify-end gap-2">
        <button
          type="button"
          onClick={onToggleChat}
          aria-label={chatOpen ? 'Close chat' : `Open chat${unreadCount > 0 ? ` (${unreadCount} unread)` : ''}`}
          className={cn('relative', SIDE_BUTTON_CLASS, chatOpen ? SIDE_BUTTON_ACTIVE : SIDE_BUTTON_RESTING)}
        >
          <MessageSquare size={17} />
          {unreadCount > 0 && !chatOpen && (
            <span
              className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--meet-btn-leave-bg)] px-1 text-[10px] font-semibold text-[var(--meet-btn-leave-fg)]"
              aria-hidden="true"
            >
              {unreadCount > 99 ? '99+' : unreadCount}
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={onLeave}
          aria-label="Leave meeting"
          // meet-btn-leave: the corner comes from the token, which needs !important against the
          // utilities on the same element.
          className="meet-btn-leave flex h-12 w-14 shrink-0 items-center justify-center border-none bg-[var(--meet-btn-leave-bg)] text-[var(--meet-btn-leave-fg)] transition-colors duration-150 hover:bg-[var(--meet-btn-leave-hover)]"
        >
          <PhoneOff size={18} />
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Check it compiles**

```bash
cd apps/web && bun run check
```

Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/meeting/MeetingCallControlsRow.tsx
git commit -m "add the five phone call controls for the pill's floor"
```

---

## Task 6: The options panel

**Files:**
- Create: `apps/web/src/components/meeting/MeetingOptionsPanel.tsx`

**Interfaces:**
- Consumes: `MeetingOptionRow`, `MeetingOptionRowId` from Task 2;
  `--meet-controls-panel-max-height` from Task 3.
- Produces:

```tsx
export function MeetingOptionsPanel(props: {
  rows: MeetingOptionRow[]
  expanded: boolean
  onSelect: (id: MeetingOptionRowId) => void
}): React.JSX.Element
```

Task 7 places it between the handle and the controls row, and owns the `onSelect` handlers and the
close rule — this component only reports which row was tapped.

The panel is capped and scrolls. Android's never does, because five rows always fit; twelve do not.
Only the rows scroll: the handle and the controls row are the surface's fixed ends.

- [ ] **Step 1: Write the component**

`apps/web/src/components/meeting/MeetingOptionsPanel.tsx`:

```tsx
import { Check, Film, Globe, Headphones, Info, Link2, Lock, Maximize, Package, PenLine, Settings, Video } from 'lucide-react'
import type { MeetingOptionRow, MeetingOptionRowId } from '@/components/meeting/meetingOptionRows'
import { cn } from '@/lib/utils'

/**
 * The icon each row wears. Keyed by id so the pure row list stays free of React values, and so a
 * new row id fails the type check here until it is given one.
 */
const ROW_ICONS: Record<MeetingOptionRowId, React.ReactNode> = {
  videos: <Video size={18} className="shrink-0" />,
  access: <Globe size={18} className="shrink-0" />,
  info: <Info size={18} className="shrink-0" />,
  'copy-link': <Link2 size={18} className="shrink-0" />,
  deafen: <Headphones size={18} className="shrink-0" />,
  'audio-devices': <Headphones size={18} className="shrink-0" />,
  noise: <Settings size={18} className="shrink-0" />,
  settings: <Settings size={18} className="shrink-0" />,
  fullscreen: <Maximize size={18} className="shrink-0" />,
  whiteboard: <PenLine size={18} className="shrink-0" />,
  youtube: <Film size={18} className="shrink-0" />,
  'app-gallery': <Package size={18} className="shrink-0" />,
}

/** The private-room row is the one place the icon depends on the label rather than the id. */
function iconFor(row: MeetingOptionRow): React.ReactNode {
  if (row.id === 'access' && row.label === 'Private room') return <Lock size={18} className="shrink-0" />
  return ROW_ICONS[row.id]
}

interface MeetingOptionsPanelProps {
  rows: MeetingOptionRow[]
  expanded: boolean
  onSelect: (id: MeetingOptionRowId) => void
}

/**
 * The room options, unfolded above the controls inside the same surface. Collapsed it has no
 * height and no tab stops; it is not unmounted, so the height transition has something to animate
 * from.
 */
export function MeetingOptionsPanel({ rows, expanded, onSelect }: MeetingOptionsPanelProps) {
  return (
    <div
      // 320ms opening, 260ms closing — Android's two durations, which differ on purpose.
      className={cn(
        'w-full overflow-hidden transition-[max-height,opacity] ease-[cubic-bezier(0.4,0,0.2,1)]',
        expanded ? 'opacity-100 duration-[320ms]' : 'max-h-0 opacity-0 duration-[260ms]',
      )}
      style={expanded ? { maxHeight: 'var(--meet-controls-panel-max-height)' } : undefined}
      aria-hidden={!expanded}
    >
      <ul className="max-h-[calc(var(--meet-controls-panel-max-height)-6rem)] list-none overflow-y-auto px-2 py-1">
        {rows.map((row) => (
          <li key={row.id}>
            <button
              type="button"
              disabled={row.disabled || !expanded}
              onClick={() => onSelect(row.id)}
              className={cn(
                'flex w-full items-center gap-3 rounded-md px-3 py-3 text-left text-[14px] transition-colors duration-150',
                row.checked ? 'text-[var(--meet-btn-muted-fg)]' : 'text-[var(--meet-fg-strong)]',
                row.disabled ? 'cursor-not-allowed opacity-40' : 'hover:bg-[var(--meet-control-hover)]',
              )}
            >
              {iconFor(row)}
              <span className="flex-1">{row.label}</span>
              {row.kind === 'toggle' && row.checked && (
                <Check size={16} className="shrink-0 text-[var(--meet-btn-muted-fg)]" />
              )}
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
```

- [ ] **Step 2: Check it compiles**

```bash
cd apps/web && bun run check
```

Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/meeting/MeetingOptionsPanel.tsx
git commit -m "add the meeting options panel for room settings above the controls"
```

---

## Task 7: The surface, and the dialogs it replaces

This is the task that would not compile if split: the pill's first render and the deletion of the
surfaces it replaces are one change.

**Files:**
- Create: `apps/web/src/components/meeting/MeetingControlsPill.tsx`
- Modify: `apps/web/src/components/meeting/ControlsBar.tsx`
- Test: `apps/web/src/components/meeting/controlsPill.test.ts`

**Interfaces:**
- Consumes: `expandedAfterDrag`, `PANEL_DRAG_THRESHOLD_PX` (Task 1); `meetingOptionRows`,
  `MeetingOptionRow`, `MeetingOptionRowId`, `MeetingOptionsInput` (Task 2); `--meet-scrim`,
  `--meet-controls-panel-max-height`, `--meet-controls-bar-radius` (Task 3);
  `MeetingCallControlsRow` (Task 5); `MeetingOptionsPanel` (Task 6).
- Produces: `MeetingControlsPill`, taking the union of `MeetingCallControlsRow`'s props plus
  `rows: MeetingOptionRow[]` and `onSelectOption: (id: MeetingOptionRowId) => void`.

**What to delete from `ControlsBar.tsx`:**

1. The `⋯` mobile dialog, currently the block starting `open={moreOpen && isMobile}` near line 1063,
   through its closing `</Dialog>`.
2. The audio mobile dialog, currently the block starting `open={audioOpen && isMobile}` near line
   924, through its closing `</Dialog>`.
3. `morePage`, `moreNavDir`, `morePageAnim`, and the `useEffect` that resets them.
4. `runMoreAction`, replaced by `runOptionRow` in Step 5. Its only remaining caller was the deleted
   dialog; the desktop dropdown switches to the new callback.
5. The `MoreRow` type, replaced by `MeetingOptionRow` from Task 2.
6. The mobile `⋯` trigger button and the mobile audio chevron button.
7. The `isMobile` branches inside the surviving bar element, which is now desktop-only: the
   `isMobile ?` ternaries on padding, `iconSize`, `iconSizeSm`, and `!isMobile &&` guards.

Keep `moreOpen` and `audioOpen` — the desktop dropdown and the desktop audio menu still use them.
Keep `moreRows`; replace its body with a call to `meetingOptionRows`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/components/meeting/controlsPill.test.ts`:

```ts
// @vitest-environment node
//
// These assertions read component sources from disk, so they run in Node rather than jsdom:
// jsdom's URL resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(fileName: string): string {
  return readFileSync(new URL(`./${fileName}`, import.meta.url), 'utf8')
}

const pill = source('MeetingControlsPill.tsx')
const micPill = source('MeetingMicPill.tsx')
const controlsBar = source('ControlsBar.tsx')

describe('the controls pill surface', () => {
  it('should carry exactly one corner class', () => {
    expect(pill).toContain('rounded-3xl')
    expect(pill.match(/rounded-(?:sm|md|lg|xl|2xl|3xl|none)\b/g)).toEqual(['rounded-3xl'])
  })

  it('should span the width rather than dock like the desktop bar', () => {
    expect(pill).toContain('inset-x-2')
    expect(pill).not.toContain('meetControlsDockClass')
    expect(pill).not.toContain('-translate-x-1/2')
  })

  it('should sit above the bottom safe area', () => {
    expect(pill).toContain('env(safe-area-inset-bottom, 0px)')
  })

  it('should dim the call with the meeting scrim', () => {
    expect(pill).toContain('var(--meet-scrim)')
  })

  it('should resolve a drag through the shared rule rather than its own threshold', () => {
    expect(pill).toContain('expandedAfterDrag')
    expect(pill).not.toMatch(/\b24\b/)
  })

  it('should close on Escape', () => {
    expect(pill).toContain('Escape')
  })
})

describe('the mic pill', () => {
  it.each(['Speak', 'Muted', 'Push to Talk', 'Talking…'])('should carry the %s label', (label) => {
    expect(micPill).toContain(label)
  })
})

describe('the controls bar after the pill lands', () => {
  it('should no longer open a full-screen sheet on a phone', () => {
    expect(controlsBar).not.toContain('moreOpen && isMobile')
    expect(controlsBar).not.toContain('audioOpen && isMobile')
  })

  it('should no longer carry the more menu sub-page machinery', () => {
    expect(controlsBar).not.toContain('morePage')
    expect(controlsBar).not.toContain('moreNavDir')
  })

  it('should render the pill below the phone breakpoint', () => {
    expect(controlsBar).toContain('MeetingControlsPill')
  })

  it('should keep the desktop dropdown', () => {
    expect(controlsBar).toContain('DropdownMenu')
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bun run test controlsPill
```

Expected: `Failed to resolve import` — `MeetingControlsPill.tsx` does not exist.

- [ ] **Step 3: Write the surface**

`apps/web/src/components/meeting/MeetingControlsPill.tsx`:

```tsx
import { useCallback, useEffect, useRef, useState } from 'react'
import { MeetingCallControlsRow } from '@/components/meeting/MeetingCallControlsRow'
import { MeetingOptionsPanel } from '@/components/meeting/MeetingOptionsPanel'
import { expandedAfterDrag } from '@/components/meeting/controlsPanelDrag'
import type { MeetingOptionRow, MeetingOptionRowId } from '@/components/meeting/meetingOptionRows'
import { cn } from '@/lib/utils'

/** The grab bar: Android's 32 × 4dp, inside a 48px tap target spanning the surface. */
const HANDLE_PILL_CLASS = 'h-1 w-8 rounded-full bg-[var(--meet-fg-muted)] opacity-55'

interface MeetingControlsPillProps {
  rows: MeetingOptionRow[]
  onSelectOption: (id: MeetingOptionRowId) => void
  cameraEnabled: boolean
  onToggleCamera: () => void
  screenShareEnabled: boolean
  screenShareAvailable: boolean
  onToggleScreenShare: () => void
  micPushToTalk: boolean
  micOpen: boolean
  micTransmitting: boolean
  micAvailable: boolean
  onToggleMic: () => void
  onPushToTalkChange: (held: boolean) => void
  chatOpen: boolean
  unreadCount: number
  onToggleChat: () => void
  onLeave: () => void
}

/**
 * The phone in-call controls, and the room options that grow out of them.
 *
 * One surface, anchored to the bottom, so the options unfold *above* the controls and the row your
 * thumb is resting on never moves — the pill simply becomes taller. This mirrors Android's
 * `MeetingControlsPanel`, which is deliberately not built on the app's sheet primitive for the
 * same reason: as a sheet, the options arrived as a second surface carrying a duplicate copy of
 * the same controls at a different height.
 */
export function MeetingControlsPill({
  rows,
  onSelectOption,
  chatOpen,
  onToggleChat,
  onLeave,
  ...controls
}: MeetingControlsPillProps) {
  const [expanded, setExpanded] = useState(false)
  const dragStartRef = useRef<number | null>(null)

  const collapse = useCallback(() => setExpanded(false), [])

  // Escape is the web's answer to Android's BackHandler.
  useEffect(() => {
    if (!expanded) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') collapse()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [expanded, collapse])

  const handlePointerDown = (event: React.PointerEvent<HTMLButtonElement>) => {
    dragStartRef.current = event.clientY
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const handlePointerUp = (event: React.PointerEvent<HTMLButtonElement>) => {
    const start = dragStartRef.current
    dragStartRef.current = null
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    if (start === null) return
    const next = expandedAfterDrag(expanded, event.clientY - start)
    // A drag the rule declined to act on was a tap on the handle, which toggles.
    setExpanded(next === expanded ? !expanded : next)
  }

  // Rows that lead somewhere else close the panel on the way; toggles leave it open so the flip is
  // visible. The caller knows which is which, so it decides — this only reports the tap.
  const selectRow = (id: MeetingOptionRowId) => {
    const row = rows.find((candidate) => candidate.id === id)
    if (row && row.kind === 'action') collapse()
    onSelectOption(id)
  }

  return (
    <>
      {expanded && (
        <button
          type="button"
          aria-label="Close room options"
          onClick={collapse}
          className="fixed inset-0 z-20 border-none bg-[var(--meet-scrim)] transition-opacity duration-[320ms] ease-[cubic-bezier(0.4,0,0.2,1)]"
        />
      )}

      <div
        id="meet-controls"
        // meet-controls-bar: the corner comes from the token, which needs !important against the
        // utilities on the same element.
        className={cn(
          'meet-controls-bar fixed inset-x-2 z-30 flex flex-col items-center overflow-hidden rounded-3xl',
          'border border-[var(--meet-border-subtle)] bg-[var(--meet-chrome)] backdrop-blur-xl',
          'shadow-[var(--meet-shadow),var(--meet-shadow-inset)]',
        )}
        style={{ bottom: 'calc(12px + env(safe-area-inset-bottom, 0px))' }}
      >
        <button
          type="button"
          aria-label="Room options"
          aria-expanded={expanded}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          className="flex h-12 w-full shrink-0 cursor-grab items-center justify-center border-none bg-transparent active:cursor-grabbing"
        >
          <span className={HANDLE_PILL_CLASS} />
        </button>

        <MeetingOptionsPanel rows={rows} expanded={expanded} onSelect={selectRow} />

        <MeetingCallControlsRow
          {...controls}
          chatOpen={chatOpen}
          onToggleChat={() => {
            collapse()
            onToggleChat()
          }}
          onLeave={() => {
            collapse()
            onLeave()
          }}
        />
      </div>
    </>
  )
}
```

- [ ] **Step 4: Branch `ControlsBar` on the breakpoint**

In `apps/web/src/components/meeting/ControlsBar.tsx`, replace the `#meet-controls` element — the
`<div id="meet-controls" …>` that opens near line 616 and its children through its closing `</div>`
— with a conditional. The phone side renders the pill; the desktop side keeps exactly the markup
that is there today:

```tsx
{isMobile ? (
  <MeetingControlsPill
    rows={moreRows}
    onSelectOption={runOptionRow}
    cameraEnabled={camEnabled}
    onToggleCamera={() => localParticipant?.setCameraEnabled(!camEnabled).catch(() => {})}
    screenShareEnabled={isScreenShareEnabled}
    screenShareAvailable={canShare && !stageTakenByOther}
    onToggleScreenShare={toggleScreenShare}
    micPushToTalk={pushToTalkEnabled}
    micOpen={micUiEnabled && !isSelfDeafened}
    micTransmitting={pttVisible}
    micAvailable={pushToTalkEnabled ? pttAvailable : true}
    onToggleMic={() => {
      if (isSelfDeafened) {
        toggleSelfDeafen()
        return
      }
      toggleMic()
    }}
    onPushToTalkChange={(held) => (held ? startPtt() : stopPtt())}
    chatOpen={chatOpen}
    unreadCount={unreadCount}
    onToggleChat={onToggleChat}
    onLeave={onLeave}
  />
) : (
  <div id="meet-controls" className={…}>
    …
  </div>
)}
```

The `…` above is not shorthand for "write something here": it is the existing `<div
id="meet-controls">` element and every child it already has, moved verbatim into the `else` branch.
Do not retype it and do not restyle it. The only edits inside it are the `isMobile` removals in
item 7 of the deletion list, which are all now dead branches.

`chatOpen`, `unreadCount` and `onToggleChat` arrive in Task 8. Until then, pass `chatOpen={false}`,
`unreadCount={0}` and `onToggleChat={() => {}}` so this task compiles on its own, and replace them
in Task 8.

The screen-share handler in the current markup is a long inline async arrow. Lift it to a named
`toggleScreenShare` callback above the return, unchanged, so both branches call the same one rather
than duplicating it.

- [ ] **Step 5: Rebuild `moreRows` on the pure module**

Replace the body of the `moreRows` `useMemo` with a call to `meetingOptionRows`, and add a
`runOptionRow` callback beside the existing `runMoreAction` that maps a row id to its effect:

```tsx
const moreRows = useMemo(
  () =>
    meetingOptionRows({
      videoSidebarAvailable: Boolean(moreExtras?.showVideoSidebarToggle && moreExtras.onToggleVideoSidebar),
      videoSidebarOpen: Boolean(moreExtras?.videoSidebarOpen),
      roomAccessAvailable: Boolean(moreExtras?.onRoomAccess),
      isPublic: Boolean(moreExtras?.isPublic),
      roomId: moreExtras?.roomId,
      linkCopied,
      isSelfDeafened,
      fullscreenAvailable: typeof document !== 'undefined' && document.fullscreenEnabled,
      isFullscreen: typeof document !== 'undefined' && Boolean(document.fullscreenElement),
      whiteboardEnabled,
      isWhiteboardOnStage: stage?.kind === 'whiteboard',
      isWhiteboardHost,
      youtubeEnabled,
      isYoutubeOnStage: stage?.kind === 'youtube',
      isYoutubeHost,
      webxdcEnabled,
      isWebxdcOnStage,
      stageTakenByOther,
    }),
  [
    moreExtras,
    linkCopied,
    isSelfDeafened,
    whiteboardEnabled,
    youtubeEnabled,
    webxdcEnabled,
    isWebxdcOnStage,
    stage,
    isWhiteboardHost,
    isYoutubeHost,
    stageTakenByOther,
  ],
)

/** Runs the effect behind a row id, for both the phone panel and the desktop dropdown. */
const runOptionRow = useCallback(
  (id: MeetingOptionRowId) => {
    switch (id) {
      case 'videos':
        moreExtras?.onToggleVideoSidebar?.()
        return
      case 'access':
        moreExtras?.onRoomAccess?.()
        return
      case 'info':
        openRoomInfo()
        return
      case 'copy-link':
        copyRoomLink()
        return
      case 'deafen':
        toggleSelfDeafen()
        return
      case 'audio-devices':
        setAudioOpen(true)
        return
      case 'noise':
        setAudioOpen(true)
        return
      case 'settings':
        setSettingsOpen(true)
        return
      case 'fullscreen':
        toggleFullscreen()
        return
      case 'whiteboard': {
        if (stage?.kind === 'whiteboard' && isWhiteboardHost) {
          clearStage()
          return
        }
        const error = requestStartWhiteboard()
        if (error) toast.error(error)
        return
      }
      case 'youtube':
        if (stage?.kind === 'youtube' && isYoutubeHost) stopYoutubeShare()
        else openShareDialog()
        return
      case 'app-gallery':
        setWebxdcAppsOpen(true)
        return
    }
  },
  [
    moreExtras,
    copyRoomLink,
    toggleSelfDeafen,
    toggleFullscreen,
    stage,
    isWhiteboardHost,
    isYoutubeHost,
    clearStage,
    requestStartWhiteboard,
    stopYoutubeShare,
    openShareDialog,
  ],
)
```

`openRoomInfo` dispatches the existing `MEETING_OPEN_ROOM_INFO` event, which `MeetingRoomShell`
already listens for — that is what replaces the deleted in-dialog sub-page:

```tsx
/** Opens the room info panel the shell owns, in place of the deleted in-dialog sub-page. */
const openRoomInfo = useCallback(() => {
  window.dispatchEvent(new CustomEvent(MEETING_OPEN_ROOM_INFO))
}, [])
```

`audio-devices` and `noise` share an effect because the desktop audio menu holds both lists in one
surface. On a phone both rows open that same menu; splitting it into two surfaces is not this
unit's job.

- [ ] **Step 6: Delete the two phone dialogs and their machinery**

Remove, in this order, so the file compiles at each stop: the mobile `⋯` trigger button, the mobile
audio chevron button, the `open={audioOpen && isMobile}` dialog, the `open={moreOpen && isMobile}`
dialog, then `morePage`, `moreNavDir`, `morePageAnim` and the `useEffect` that resets them.
`tsc` will name anything left unused.

- [ ] **Step 7: Run the test and the suite**

```bash
cd apps/web && bun run test controlsPill && bun run check && bun run test
```

Expected: `controlsPill` 14 passed; `check` exit 0; the full suite 62 files, 336 tests.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/components/meeting/MeetingControlsPill.tsx apps/web/src/components/meeting/controlsPill.test.ts apps/web/src/components/meeting/ControlsBar.tsx
git commit -m "add the phone controls pill and delete the two full-screen menus it replaces"
```

---

## Task 8: Chat moves into the bar

**Files:**
- Modify: `apps/web/src/components/meeting/MeetingPanels.tsx` — the mobile top-right cluster near
  line 110, and the `<MeetingControls …>` call near line 148
- Modify: `apps/web/src/components/meeting/MeetingControls.tsx` — props through to `ControlsBar`
- Modify: `apps/web/src/components/meeting/ControlsBar.tsx` — the three placeholder props from
  Task 7

**Interfaces:**
- Consumes: `MeetingControlsPill`'s `chatOpen`, `unreadCount`, `onToggleChat` from Task 7.
- Produces: nothing later tasks depend on.

`MeetingPanels` already owns `chatOpen` and `toggleChat`, and already reads `unreadCount` through
`useMeetingChatContext` inside `ChatToggle`. Two hops of prop drilling — `MeetingPanels` →
`MeetingControls` → `ControlsBar` — beat a third event on the chrome bus for state this component
tree already holds.

**Merge-conflict warning:** unit 2 edits the same top-right cluster in `MeetingPanels.tsx`,
splitting `mobileOverlayOpen` into two flags. Whichever unit merges second resolves by hand. Do not
try to anticipate unit 2's shape here; make the smallest change that works on this branch.

- [ ] **Step 1: Hide the phone chat toggle**

In `apps/web/src/components/meeting/MeetingPanels.tsx`, delete the `ChatToggle` from the mobile
top-right cluster, leaving `ParticipantsToggle` alone:

```tsx
<ParticipantsToggle isOpen={participantsOpen} onToggle={onToggleParticipants} variant="icon" />
```

The desktop `ChatToggle` on the following line keeps its `className="hidden lg:flex"` and is not
touched.

- [ ] **Step 2: Pass chat down**

Add to the `<MeetingControls …>` call in `MeetingPanels.tsx`:

```tsx
chatOpen={chatOpen}
onToggleChat={toggleChat}
```

In `MeetingControls.tsx`, add both to `MeetingControlsProps` and forward them:

```tsx
interface MeetingControlsProps {
  onNavigate: () => void
  /** Hide the floating controls bar on mobile (e.g. full-screen participants list). */
  hideOnMobile?: boolean
  /** Merged into the phone options panel and the desktop ⋯ menu. */
  moreExtras?: ControlsBarMoreExtras
  chatOpen: boolean
  onToggleChat: () => void
}
```

```tsx
<ControlsBar onLeave={handleLeaveRequest} moreExtras={moreExtras} chatOpen={chatOpen} onToggleChat={onToggleChat} />
```

In `ControlsBar.tsx`, add them to `Props`, read the unread count from the chat context, and replace
Task 7's three placeholders:

```tsx
const { unreadCount } = useMeetingChatContext()
```

- [ ] **Step 3: Run the suite**

```bash
cd apps/web && bun run check && bun run test
```

Expected: exit 0; 62 files, 336 tests. This task adds no tests — the count is unchanged from
Task 7.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/components/meeting/MeetingPanels.tsx apps/web/src/components/meeting/MeetingControls.tsx apps/web/src/components/meeting/ControlsBar.tsx
git commit -m "update the phone chat toggle to live in the controls pill for one entry point"
```

---

## Task 9: Docs

**Files:**
- Modify: `apps/web/AGENTS.md` — a new section after "Phone meeting chat" if unit 2 has merged,
  otherwise after "Mobile detection"
- Modify: `docs/plan/pwa-parity/01-overview-and-units.md` — the unit 3 row and the controls
  vocabulary row

**Interfaces:** none.

- [ ] **Step 1: Write the AGENTS.md section**

````markdown
### Phone meeting controls

Below `MOBILE_BREAKPOINT_PX`, `ControlsBar` renders `MeetingControlsPill` instead of the desktop
bar. One surface, anchored to the bottom: handle, options panel, controls row. The options unfold
*above* the controls, so the controls never move.

Five controls, in Android's order: camera, screen share, mic pill, chat, leave. Chat has no
top-right toggle on a phone — the pill is its only entry point.

Two things are easy to get wrong:

- **The panel is not a sheet.** It must not be rebuilt on `BedrudSheet`. A sheet puts the options
  on a second surface with its own copy of the controls, which is the bug Android's
  `MeetingControlsPanel` documents at length.
- **The drag threshold lives in `controlsPanelDrag.ts`,** not in the component. `controlsPill.test.ts`
  fails if a bare `24` appears in `MeetingControlsPill.tsx`.

The row list is pure and lives in `meetingOptionRows.ts`; icons and handlers are mapped from the row
id in `MeetingOptionsPanel.tsx` and `ControlsBar.tsx`. Adding a row means adding to
`MeetingOptionRowId`, which makes both maps fail the type check until they are updated.
````

- [ ] **Step 2: Update the overview**

In `docs/plan/pwa-parity/01-overview-and-units.md`, change the unit 3 row's status cell to:

```
done, [10](./10-meeting-controls-pill.md), [11](./11-meeting-controls-pill-plan.md)
```

and the `MeetingControlsPanel` vocabulary row's "Where it lives" cell to:

```
`MeetingControlsPill`, `components/meeting/MeetingControlsPill.tsx` — built in unit 3
```

- [ ] **Step 3: Commit**

```bash
git add apps/web/AGENTS.md docs/plan/pwa-parity/01-overview-and-units.md docs/plan/pwa-parity/10-meeting-controls-pill.md docs/plan/pwa-parity/11-meeting-controls-pill-plan.md
git commit -m "document the phone controls pill and its options panel"
```

---

## After the tasks: verify on a device

Tests pin what the files say. They cannot show where the surface lands. Unit 2 shipped three
defects that were green in CI and wrong on screen, all found by driving a live meeting — do not
skip this.

Run the stack, join a room at 375 × 812, and drive every line in the spec's
[Verification on a device](./10-meeting-controls-pill.md#verification-on-a-device) checklist.
Capture before and after screenshots in both themes, the before taken on
`feat/pwa-parity-foundation`.

**Then stop.** Do not commit the captures, push the branch, or open the PR without explicit
approval.

---

## Deviations found during implementation

Five places this plan was wrong or incomplete. Recorded here rather than quietly fixed, because
the next unit's plan is written by someone reading this one.

1. **Tasks 1 and 2 ran only `bun run test`.** Biome treats import ordering and formatting as
   errors, so Task 2's commit went in red and had to be amended. Both tasks now run `check` too.
2. **The audio device lists had nowhere to go.** The spec said they become panel rows; the plan
   made them two action rows calling `setAudioOpen(true)`, which only rendered inside the dialog
   the same task deletes. On desktop that surface is a `DropdownMenu` opened by its own trigger and
   cannot be opened programmatically either, so the rows would have done nothing. Resolved by
   making each microphone, speaker and noise mode its own row, which is what the spec said.
   `meetingOptionRows` gained heading rows and device-prefixed ids.
3. **Noise modes need a `disabled` flag.** The deleted dialog showed an "N/A" badge for Krisp when
   the browser cannot run it. Without the flag the panel renders a selectable row that silently
   does nothing.
4. **The phone branch cannot be an early `return`.** `BedrudSettingsDialog` and `WebxdcAppsDialog`
   render after the bar and are reached from the panel; returning the pill early unmounts both and
   the Settings and App gallery rows stop working. Only the bar element branches.
5. **Tasks 7 and 8 are one commit.** The pill needs `chatOpen` and `onToggleChat` flowing from
   `MeetingPanels`, so Task 7 cannot compile without Task 8's plumbing. The plan's suggestion to
   pass placeholder values would have shipped a commit whose chat button did nothing.

Task 7 also split its cleanup into a second commit: with the bar now desktop-only, thirteen
`isMobile ? … : …` expressions inside it could only ever take the desktop side, and `CtrlBtn`'s
`isMobile` prop had no caller left.
