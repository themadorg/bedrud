# Meeting chat sheet implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put the in-call chat on a bottom sheet over the live call on phones — half on open, full on drag, on a handle tap or when the keyboard opens, dismissed on a second drag down — and ship the sheet primitive units 3 and 4 will reuse.

**Architecture:** One new dependency, `vaul`, wrapped once as `BedrudSheet` in `components/ui/`. What vaul owns — drag, velocity, snapping, dismissal — is not reimplemented; what CSS owns — the sheet's ceiling — is a `--meet-sheet-max-height` token; what is left over is two small pure modules, the keyboard rule in `sheetKeyboard.ts` and the four-way surface choice in `chatSurface.ts`. `ChatPanel` keeps its desktop presentations untouched and gains the sheet below 1024px; the compensating `mobileMeetingBar` is deleted with it. The dead `components/ui/sheet.tsx` goes in the same change.

**Tech Stack:** React 19, TanStack Start, TailwindCSS v4, shadcn/ui, vaul 1.1.2, Vitest 4, Biome, Bun.

**Spec:** [08 — Meeting chat sheet](./08-meeting-chat-sheet.md). Read it before Task 1.

## Global Constraints

- One breakpoint. Phone layout is below **1024px**, expressed as `lg:` and `max-lg:`, or in TypeScript as `useIsMobile()`. No `sm:`, `max-sm:`, `md:` or `max-md:` prefix may be introduced by this unit.
- Exactly **one `rounded-*` class per element**. `cn` is `twMerge(clsx(...))` and keeps the last radius it sees, so a second one is silently dead. The sheet's top corner is `rounded-t-3xl` (28px) and is declared in `BedrudSheet` only.
- **The sheet's fixed values are fixed, not defaulted.** Corner, container colour, handle, bottom safe-area inset and side gutter are not props. Only the snap points, the handle's tap behaviour, the accessible label and the content are configurable.
- **vaul owns drag physics.** Do not write velocity, threshold or snap-resolution code. If a behaviour in the spec maps onto a vaul prop, use the prop.
- **Biome owns final layout.** Run `bunx biome check --write` on touched files before committing and take its output verbatim. Its import sort is case-insensitive.
- **Every commit compiles on its own.** A changed signature and the call sites that feed it belong in one commit, along with whatever the change deletes.
- **No abbreviated identifiers.** `index`, not `i`; `viewportHeight`, not `vh`. Lambda parameters included.
- **Comments sit above what they describe**, capitalised, a full sentence, explaining why rather than restating the code.
- **Define before use.** A private helper is defined textually above the component that calls it; the exported component is last in the file.
- Tests are written first and seen failing before the code that satisfies them.
- The repo's test convention is **pure logic, `renderHook`, or source-reading**. There are no React render tests in this codebase and this unit adds none; a claim about rendered structure is pinned by reading the source file, the way `design-tokens.test.ts` and unit 6's `dialog-breakpoints.test.ts` do.
- Test descriptions are imperative and begin with `should`, inside a `describe` naming the module under test.
- The one combined verify command is `bun run check && bun run test`, run from `apps/web`.
- No attribution of any kind in commits.

---

## File Structure

| File | Responsibility |
|---|---|
| `src/components/ui/sheetKeyboard.ts` | **Create.** Whether a visible-viewport shrink is an on-screen keyboard. No React. |
| `src/components/ui/sheetKeyboard.test.ts` | **Create.** Covers the threshold in both directions. |
| `src/components/meeting/chatSurface.ts` | **Create.** Resolves the four booleans around chat into one named surface. |
| `src/components/meeting/chatSurface.test.ts` | **Create.** Covers all four surfaces and the precedence between them. |
| `src/components/ui/BedrudSheetHandle.tsx` | **Create.** The 32 × 4 pill in a 48px drag strip; a `button` when it taps, a `div` when it does not. |
| `src/components/ui/BedrudSheet.tsx` | **Create.** The app's one bottom sheet, over vaul. |
| `src/components/ui/bedrudSheet.test.ts` | **Create.** Pins the fixed values and the absence of stale breakpoint prefixes. |
| `src/components/ui/sheet.tsx` | **Delete.** Dead shadcn wrapper, no importers. |
| `src/components/meeting/ChatPanel.tsx` | **Modify.** Renders the sheet below 1024px; `mobileMeetingBar` deleted; markers preserved. |
| `src/components/meeting/chatMarkers.test.ts` | **Create.** Every chat selector in `meeting.css` matches what `ChatPanel` renders. |
| `src/components/meeting/meeting.css` | **Modify.** The 23 chat selector lists gain the sheet's element, and a `--meet-sheet-max-height` token is added beside the other meeting tokens. |
| `src/components/meeting/MeetingPanels.tsx` | **Modify.** The top-right cluster stays visible while chat is open. |
| `apps/web/AGENTS.md` | **Modify.** A "Phone meeting chat" section beside "Phone settings" and "Phone dashboard". |
| `docs/plan/pwa-parity/01-overview-and-units.md` | **Modify.** Unit 2 row, and the sheet line in the component-vocabulary table. |
| `apps/web/package.json` | **Modify.** `vaul` added by `bun add`, never by hand. |

### Task order

Tasks 1 and 2 create pure modules and touch nothing that exists, so each stands alone. Task 3 is the
handle, which Task 4 consumes. Task 4 adds the dependency, writes the primitive, and deletes
`sheet.tsx` in one commit — the deletion is of dead code, so nothing breaks, and splitting it would
leave a commit with two sheet primitives in `components/ui/`. Task 5 is the only behavioural change
to chat and carries the CSS edit with it, because a commit that rendered the sheet without the
selector update would render chat text in the wrong colour. Tasks 6 and 7 are the chrome and the
docs.

---

### Task 1: The keyboard rule

The sheet's *height* is not here, and deliberately so. The full height is
`--app-height` minus 12px minus `env(safe-area-inset-top)` — three values CSS already holds, two of
which JavaScript can only get back by reading them out of the document again. It is expressed as a
`--meet-sheet-max-height` token in `meeting.css` in Task 5, beside the meeting's other tokens. A
TypeScript `sheetContentMaxHeight()` would be a second source for a number the stylesheet computes,
and the tests would pin the copy rather than the one the sheet draws with.

What is left is the one question CSS cannot answer: whether a viewport that just got shorter did so
because of a keyboard.

**Files:**
- Create: `apps/web/src/components/ui/sheetKeyboard.ts`
- Test: `apps/web/src/components/ui/sheetKeyboard.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `KEYBOARD_SHRINK_THRESHOLD_PX: number`, `shouldExpandForKeyboard(previousViewportHeight, nextViewportHeight): boolean`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'
import { KEYBOARD_SHRINK_THRESHOLD_PX, shouldExpandForKeyboard } from './sheetKeyboard'

describe('shouldExpandForKeyboard', () => {
  it('should treat a large shrink as a keyboard', () => {
    expect(shouldExpandForKeyboard(812, 480)).toBe(true)
  })

  it('should not treat a browser toolbar as a keyboard', () => {
    expect(shouldExpandForKeyboard(812, 760)).toBe(false)
  })

  it('should not expand when the viewport grows back', () => {
    expect(shouldExpandForKeyboard(480, 812)).toBe(false)
  })

  it('should not expand when the viewport does not move', () => {
    expect(shouldExpandForKeyboard(812, 812)).toBe(false)
  })

  it('should sit clear of a browser toolbar and under the shortest keyboard', () => {
    expect(KEYBOARD_SHRINK_THRESHOLD_PX).toBeGreaterThan(60)
    expect(KEYBOARD_SHRINK_THRESHOLD_PX).toBeLessThan(200)
  })
})
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `bun run test -- sheetKeyboard`
Expected: FAIL, "Failed to resolve import ./sheetKeyboard".

- [ ] **Step 3: Write the module**

```ts
/**
 * The one question about a bottom sheet that neither CSS nor vaul answers.
 *
 * Which detent a release lands on, the velocity that dismisses rather than snapping, and whether a
 * fast flick may skip a detent all belong to vaul's own drag handling. The sheet's height belongs to
 * the `--meet-sheet-max-height` token. Reimplementing either here would produce code the tests
 * exercise and the running sheet never calls.
 */

/**
 * A visible viewport shrinking by more than this is an on-screen keyboard rather than a browser
 * toolbar sliding away. Mobile Safari's toolbar is roughly 60px and Chrome's is roughly 56px, so the
 * threshold sits clear of both while staying well under the shortest keyboard.
 */
export const KEYBOARD_SHRINK_THRESHOLD_PX = 120

/**
 * Whether a viewport change should take the sheet to full. Only a shrink counts: a reader who
 * expanded to type is reading what they typed, so the sheet does not collapse when the keyboard
 * closes again.
 */
export function shouldExpandForKeyboard(
  previousViewportHeight: number,
  nextViewportHeight: number,
): boolean {
  return previousViewportHeight - nextViewportHeight > KEYBOARD_SHRINK_THRESHOLD_PX
}
```

- [ ] **Step 4: Run the test and watch it pass**

Run: `bun run test -- sheetKeyboard`
Expected: PASS, 5 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/ui/sheetKeyboard.ts apps/web/src/components/ui/sheetKeyboard.test.ts
git commit -m "tell an on-screen keyboard apart from a browser toolbar"
```

---

### Task 2: The chat surface choice

`ChatPanel` currently decides its presentation from four booleans spread across
`ChatPanel.tsx:80-86` — `isMobile`, `stuck`, `elevated`, and the derived `isDocked` and `isOverlay`.
This unit adds a fourth presentation to that, which is the moment to give the decision a name and a
test.

**Files:**
- Create: `apps/web/src/components/meeting/chatSurface.ts`
- Test: `apps/web/src/components/meeting/chatSurface.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `type ChatSurface = 'sheet' | 'elevated' | 'dock' | 'overlay'`, `chatSurfaceFor({ isMobile, stuck, elevated }): ChatSurface`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'
import { chatSurfaceFor } from './chatSurface'

describe('chatSurfaceFor', () => {
  it('should put chat on a sheet below the phone breakpoint', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: false, elevated: false })).toBe('sheet')
  })

  it('should ignore a stuck pin on a phone, because the pin is desktop-only', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: true, elevated: false })).toBe('sheet')
  })

  it('should use the elevated dock whatever the width, because it belongs to expanded WebXDC', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: false, elevated: true })).toBe('elevated')
    expect(chatSurfaceFor({ isMobile: false, stuck: true, elevated: true })).toBe('elevated')
  })

  it('should dock a pinned desktop chat', () => {
    expect(chatSurfaceFor({ isMobile: false, stuck: true, elevated: false })).toBe('dock')
  })

  it('should overlay an unpinned desktop chat', () => {
    expect(chatSurfaceFor({ isMobile: false, stuck: false, elevated: false })).toBe('overlay')
  })
})
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `bun run test -- chatSurface`
Expected: FAIL, "Failed to resolve import ./chatSurface".

- [ ] **Step 3: Write the module**

```ts
/** Which surface the in-call chat draws on. */
export type ChatSurface =
  /** A bottom sheet over the live call. Phones. */
  | 'sheet'
  /** The left dock inside expanded WebXDC, which owns its own chrome. */
  | 'elevated'
  /** A 320px sidebar the stage is inset for. Desktop, pinned. */
  | 'dock'
  /** A 320px panel floating over the stage. Desktop, unpinned. */
  | 'overlay'

interface ChatSurfaceInput {
  /** Below the app's single 1024px breakpoint. */
  isMobile: boolean
  /** The desktop pin. Phones have no pin control, so this is ignored there. */
  stuck: boolean
  /** Opened from expanded WebXDC, which stacks its own dock above everything. */
  elevated: boolean
}

/**
 * Resolves the presentation once, so the four booleans are read in one place rather than recombined
 * at each use. Order matters: the elevated dock outranks the width, because expanded WebXDC owns the
 * whole screen at any size, and the phone sheet outranks the pin, because the pin is a desktop
 * control that a phone can still be carrying from a wider window.
 */
export function chatSurfaceFor({ isMobile, stuck, elevated }: ChatSurfaceInput): ChatSurface {
  if (elevated) return 'elevated'
  if (isMobile) return 'sheet'
  return stuck ? 'dock' : 'overlay'
}
```

- [ ] **Step 4: Run the test and watch it pass**

Run: `bun run test -- chatSurface`
Expected: PASS, 5 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/meeting/chatSurface.ts apps/web/src/components/meeting/chatSurface.test.ts
git commit -m "name the four surfaces the in-call chat can draw on"
```

---

### Task 3: The sheet handle

**Files:**
- Create: `apps/web/src/components/ui/BedrudSheetHandle.tsx`

**Interfaces:**
- Consumes: `cn` from `#/lib/utils`.
- Produces: `BedrudSheetHandle({ onClick?, label? })`.

The handle's source is pinned by Task 4's test, together with the rest of the primitive's fixed
values, so this task writes the component and Task 4 asserts on it. It is a separate task because
it is a separate concern and a separate commit, not because it has its own test file.

- [ ] **Step 1: Write the component**

```tsx
import { cn } from '#/lib/utils'

/** The drawn pill: Android's `Dimens.meetingHandleWidth` × `meetingHandleHeight`. */
const HANDLE_PILL_CLASS = 'h-1 w-8 rounded-full bg-[var(--meet-fg-muted)]'

/**
 * The grab bar every sheet in the app wears, and the one the call's controls bar already draws for
 * itself. One handle, one shape.
 *
 * The pill is 32 × 4 as on Android, but the target around it is a 48px full-width strip rather than
 * Android's 28dp padded box: WCAG 2.5.8 asks for 44px, and a drag target that is hard to catch is
 * worse on glass than it looks in an emulator. The strip is transparent, so it costs no visible
 * pixels.
 *
 * `onClick` is optional, as it is on Android. A sheet with one height has nothing for a tap to do,
 * so it gets a plain strip with no role and no label — the sheet is still dragged and dismissed the
 * usual ways.
 */
export function BedrudSheetHandle({ onClick, label }: { onClick?: () => void; label?: string }) {
  const strip = 'flex h-12 w-full shrink-0 cursor-grab items-center justify-center active:cursor-grabbing'

  if (!onClick) {
    return (
      <div className={strip} aria-hidden="true">
        <span className={HANDLE_PILL_CLASS} />
      </div>
    )
  }

  return (
    <button type="button" onClick={onClick} className={cn(strip, 'border-none bg-transparent')} aria-label={label}>
      <span className={HANDLE_PILL_CLASS} />
    </button>
  )
}
```

- [ ] **Step 2: Check it compiles**

Run: `bun run check`
Expected: no error for the new file. It has no importers yet, which is fine.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/ui/BedrudSheetHandle.tsx
git commit -m "add the grab bar every sheet will wear"
```

---

### Task 4: The sheet primitive

**Files:**
- Modify: `apps/web/package.json` (via `bun add`, never by hand — the lockfile is generated)
- Create: `apps/web/src/components/ui/BedrudSheet.tsx`
- Create: `apps/web/src/components/ui/bedrudSheet.test.ts`
- Delete: `apps/web/src/components/ui/sheet.tsx`

**Interfaces:**
- Consumes: `BedrudSheetHandle` (Task 3).
- Produces: `SHEET_SNAP_POINTS: number[]`, `BedrudSheet({ open, onOpenChange, label, activeSnapPoint, onSnapPointChange, dataMarkers, children })`.

- [ ] **Step 1: Confirm `sheet.tsx` is genuinely dead before deleting it**

Run: `rg -l "components/ui/sheet" apps/web/src`
Expected: no output. If anything is listed, stop and report — the spec's premise is wrong.

- [ ] **Step 2: Write the failing test**

```ts
// @vitest-environment node
//
// This file reads a component's source from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const sheetSource = readFileSync(new URL('./BedrudSheet.tsx', import.meta.url), 'utf8')
const handleSource = readFileSync(new URL('./BedrudSheetHandle.tsx', import.meta.url), 'utf8')

describe('BedrudSheet', () => {
  it('should wear the sheet corner, and only that corner', () => {
    expect(sheetSource).toMatch(/\brounded-t-3xl\b/)
    expect(sheetSource.match(/\brounded-(?!t-3xl)[a-z0-9-]+\b/g)).toBeNull()
  })

  it('should lift off the call behind it with the meeting sidebar container', () => {
    expect(sheetSource).toContain('var(--meet-sidebar)')
  })

  it('should clear the bottom safe area itself', () => {
    expect(sheetSource).toContain('env(safe-area-inset-bottom')
  })

  it('should take its ceiling from the meeting token rather than a second copy of the arithmetic', () => {
    expect(sheetSource).toContain('max-h-[var(--meet-sheet-max-height)]')
    expect(sheetSource).not.toContain('maxHeight')
  })

  it('should not introduce a breakpoint other than the shared one', () => {
    expect(sheetSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})

describe('BedrudSheetHandle', () => {
  it('should draw the pill at the metrics Android uses', () => {
    expect(handleSource).toMatch(/\bh-1\b/)
    expect(handleSource).toMatch(/\bw-8\b/)
    expect(handleSource).toMatch(/\brounded-full\b/)
  })

  it('should give the pill a target that meets the touch minimum', () => {
    expect(handleSource).toMatch(/\bh-12\b/)
  })

  it('should be a labelled button only when tapping it does something', () => {
    expect(handleSource).toContain('aria-label={label}')
    expect(handleSource).toContain('aria-hidden="true"')
  })
})

describe('the ui folder', () => {
  it('should carry exactly one sheet primitive', () => {
    expect(() => readFileSync(new URL('./sheet.tsx', import.meta.url))).toThrow()
  })
})
```

- [ ] **Step 3: Run the test and watch it fail**

Run: `bun run test -- bedrudSheet`
Expected: FAIL — `BedrudSheet.tsx` does not exist, and `sheet.tsx` still does.

- [ ] **Step 4: Add the dependency**

```bash
bun add --cwd apps/web vaul@1.1.2
```

Expected: `vaul` in `dependencies`, and `bun.lock` updated. Do not hand-edit either file.

- [ ] **Step 5: Write the primitive**

```tsx
import type { ReactNode } from 'react'
import { Drawer } from 'vaul'
import { BedrudSheetHandle } from '#/components/ui/BedrudSheetHandle'

/**
 * The two heights every sheet in the app offers, as fractions of its own content height. Half is
 * where a sheet rests when it opens, so the call above it still reads.
 */
export const SHEET_SNAP_POINTS: number[] = [0.5, 1]

interface BedrudSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Names the sheet for assistive technology, and labels the handle's tap. */
  label: string
  activeSnapPoint: number | string | null
  onSnapPointChange: (snapPoint: number | string | null) => void
  /** Marker attributes the meeting stylesheet selects on. */
  dataMarkers?: Record<string, string>
  children: ReactNode
}

/**
 * The app's bottom sheet. Every sheet in the app is one of these.
 *
 * Corner, container, handle, bottom inset and gutter are fixed rather than defaulted. A default is a
 * suggestion, and the one caller that takes the suggestion up is the one that renders wrong — the
 * Android component this mirrors carries the same note for the same reason.
 *
 * Drag, velocity, snapping and dismissal are vaul's. `snapToSequentialPoint` keeps a fast flick from
 * skipping half on the way down, so the spec's "a second drag down dismisses" holds rather than one
 * hard flick closing from full.
 */
export function BedrudSheet({
  open,
  onOpenChange,
  label,
  activeSnapPoint,
  onSnapPointChange,
  dataMarkers,
  children,
}: BedrudSheetProps) {
  return (
    <Drawer.Root
      open={open}
      onOpenChange={onOpenChange}
      snapPoints={SHEET_SNAP_POINTS}
      activeSnapPoint={activeSnapPoint}
      setActiveSnapPoint={onSnapPointChange}
      snapToSequentialPoint
      dismissible
      modal
    >
      <Drawer.Portal>
        <Drawer.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Drawer.Content
          aria-label={label}
          {...dataMarkers}
          className="fixed right-0 bottom-0 left-0 z-40 flex max-h-[var(--meet-sheet-max-height)] flex-col rounded-t-3xl bg-[var(--meet-sidebar)] backdrop-blur-2xl pb-[env(safe-area-inset-bottom,0px)]"
        >
          <Drawer.Title className="sr-only">{label}</Drawer.Title>
          <Drawer.Handle asChild>
            <BedrudSheetHandle label={`Resize ${label}`} onClick={() => {}} />
          </Drawer.Handle>
          <div className="flex min-h-0 flex-1 flex-col px-4">{children}</div>
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
```

> **Implementer note.** `Drawer.Handle` cycles the snap points on tap by itself, which is exactly the
> handle-tap behaviour the spec asks for, so the `onClick` above exists only to make
> `BedrudSheetHandle` render its labelled `button` branch. If `asChild` turns out not to be supported
> on `Drawer.Handle` in 1.1.2, render `BedrudSheetHandle` directly and pass it an `onClick` that
> calls `onSnapPointChange` with the other snap point — report which one you used, do not leave both.

- [ ] **Step 6: Delete the dead primitive**

```bash
git rm apps/web/src/components/ui/sheet.tsx
```

- [ ] **Step 7: Run the test and watch it pass**

Run: `bun run test -- bedrudSheet`
Expected: PASS, 8 tests.

- [ ] **Step 8: Run the whole verify**

Run: `bun run check && bun run test`
Expected: no new warnings beyond the three pre-existing `noDocumentCookie` ones in `api.test.ts`.

- [ ] **Step 9: Commit**

```bash
git add apps/web/package.json apps/web/bun.lock apps/web/src/components/ui/BedrudSheet.tsx apps/web/src/components/ui/bedrudSheet.test.ts
git commit -m "add the app's bottom sheet and drop the unused shadcn one"
```

---

### Task 5: Chat on the sheet

The only behavioural change in the unit, and the one commit that must carry its stylesheet edit with
it.

**Files:**
- Modify: `apps/web/src/components/meeting/ChatPanel.tsx`
- Modify: `apps/web/src/components/meeting/meeting.css`
- Create: `apps/web/src/components/meeting/chatMarkers.test.ts`

**Interfaces:**
- Consumes: `BedrudSheet` and `SHEET_SNAP_POINTS` (Task 4), `chatSurfaceFor` (Task 2), `shouldExpandForKeyboard` (Task 1).
- Produces: no new export. `ChatPanel`'s props are unchanged.

- [ ] **Step 1: Write the failing test**

```ts
// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const chatPanelSource = readFileSync(new URL('./ChatPanel.tsx', import.meta.url), 'utf8')
const meetingCss = readFileSync(new URL('./meeting.css', import.meta.url), 'utf8')

/** Every distinct chat marker the meeting stylesheet selects on. */
const CHAT_MARKERS = ['data-chat-overlay', 'data-elevated-chat', 'aria-label="Chat"']

describe('the chat markers the meeting stylesheet depends on', () => {
  it.each(CHAT_MARKERS)('should still be rendered by ChatPanel: %s', (marker) => {
    expect(chatPanelSource).toContain(marker.replace(/"/g, ''))
  })

  it('should be selected on an element type the sheet actually renders', () => {
    // vaul renders a div, not an aside. A selector list that names only `aside` would stop
    // matching with no error and chat text would fall back to raw `text-white/*`.
    const asideOnly = /aside\[data-chat-overlay="true"\]/g
    const sheetSelector = /\[data-chat-overlay="true"\]/g
    expect(meetingCss.match(sheetSelector)?.length).toBeGreaterThan(
      meetingCss.match(asideOnly)?.length ?? 0,
    )
  })
})

describe('the phone chat surface', () => {
  it('should no longer carry a meeting controls strip of its own', () => {
    expect(chatPanelSource).not.toContain('mobileMeetingBar')
  })

  it('should choose its surface through the named helper', () => {
    expect(chatPanelSource).toContain('chatSurfaceFor')
  })
})
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `bun run test -- chatMarkers`
Expected: FAIL on the element-type assertion and on `mobileMeetingBar`.

- [ ] **Step 3: Add the sheet's ceiling as a meeting token**

In `meeting.css`, beside `--meet-controls-bar-radius` and the other meeting tokens:

```css
/*
 * The tallest a bottom sheet may draw. A fully expanded sheet stops short of the top so the room
 * name and a band of the call stay in view — a sheet that reaches the top of the screen is a
 * screen again. The 12px matches Android's `Dimens.space12` above the status bar, and
 * `--app-height` is the *visible* viewport, which `lib/visual-viewport.ts` keeps current.
 */
--meet-sheet-max-height: calc(var(--app-height, 100svh) - 12px - env(safe-area-inset-top, 0px));
```

It is one declaration in the light block; the value has no colour in it, so the dark block does not
repeat it.

- [ ] **Step 4: Widen the stylesheet's selector lists**

In `meeting.css`, the 23 selector lists currently read:

```css
:is(
    .meet-room,
    .meet-dialog,
    aside[aria-label="Chat"],
    aside[data-chat-overlay="true"],
    aside[data-elevated-chat="true"],
    [data-screenshare-overlay="true"]
  )
```

Drop the `aside` qualifier from the two chat markers, so the same rules match the sheet's `div`:

```css
:is(
    .meet-room,
    .meet-dialog,
    aside[aria-label="Chat"],
    [data-chat-overlay="true"],
    [data-elevated-chat="true"],
    [data-screenshare-overlay="true"]
  )
```

Do this with a single find-and-replace across the file, then read the diff: all 23 lists must change
and nothing else may. `aside[aria-label="Chat"]` keeps its qualifier — the sheet carries the data
marker, and an unqualified `[aria-label="Chat"]` would match the chat *toggle button* too.

- [ ] **Step 5: Rewrite the panel's phone branch**

Delete `mobileMeetingBar` entirely (`ChatPanel.tsx:135-193`) and the `useParticipants`,
`useLocalParticipant`, `useMeetingMicKeyboard`, `DeafenHeadphonesIcon`, `Mic`, `MicOff` and `Users`
imports it was the only user of. Verify with `rg` before deleting each one — `participants` is also
read elsewhere in the file.

Replace the surface decision at `ChatPanel.tsx:80-86` with the helper, and add the sheet branch:

```tsx
const surface = chatSurfaceFor({ isMobile, stuck, elevated })
const isOverlay = surface === 'overlay'
const fromLeft = side === 'left' && surface !== 'sheet'
```

The sheet branch, placed after the `elevated` branch and before the desktop `panel`:

```tsx
if (surface === 'sheet') {
  return (
    <BedrudSheet
      open
      onOpenChange={(nextOpen) => !nextOpen && handleClose()}
      label="Chat"
      activeSnapPoint={snapPoint}
      onSnapPointChange={setSnapPoint}
      dataMarkers={{ 'data-chat-overlay': 'true' }}
    >
      {chatBody}
    </BedrudSheet>
  )
}
```

The state it reads, defined with the file's other hooks:

```tsx
const [snapPoint, setSnapPoint] = useState<number | string | null>(SHEET_SNAP_POINTS[0])
const expandSheet = useCallback(() => setSnapPoint(SHEET_SNAP_POINTS[SHEET_SNAP_POINTS.length - 1]), [])
useKeyboardExpandsSheet(surface === 'sheet', expandSheet)
```

And the hook, defined textually above `ChatPanel` per the define-before-use rule:

```tsx
/**
 * Takes the sheet to full when the keyboard opens.
 *
 * A keyboard over a half-open sheet leaves a sliver of conversation, so focusing the composer
 * expands it. The Visual Viewport `resize` event is what an on-screen keyboard fires, and
 * `lib/visual-viewport.ts` already listens to it for `--app-height`; this reads the same signal
 * rather than guessing from focus events, which also fire for a hardware keyboard where nothing
 * needs to move.
 */
function useKeyboardExpandsSheet(enabled: boolean, onExpand: () => void): void {
  useEffect(() => {
    const viewport = window.visualViewport
    if (!enabled || !viewport) return

    let previousHeight = viewport.height
    const handleResize = () => {
      const nextHeight = viewport.height
      if (shouldExpandForKeyboard(previousHeight, nextHeight)) onExpand()
      previousHeight = nextHeight
    }

    viewport.addEventListener('resize', handleResize)
    return () => viewport.removeEventListener('resize', handleResize)
  }, [enabled, onExpand])
}
```

- [ ] **Step 6: Run the test and watch it pass**

Run: `bun run test -- chatMarkers`
Expected: PASS, 5 tests.

- [ ] **Step 7: Run the whole verify**

Run: `bun run check && bun run test`
Expected: clean but for the three known `noDocumentCookie` warnings.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/components/meeting/ChatPanel.tsx apps/web/src/components/meeting/meeting.css apps/web/src/components/meeting/chatMarkers.test.ts
git commit -m "open the in-call chat as a sheet over the call on phones"
```

---

### Task 6: The meeting chrome above the sheet

**Files:**
- Modify: `apps/web/src/components/meeting/MeetingPanels.tsx`

- [ ] **Step 1: Narrow what the overlay hides**

`mobileOverlayOpen` at `MeetingPanels.tsx:84` is `chatOpen || participantsOpen`. Split it, because
the two surfaces no longer behave alike:

```tsx
// The participants list is still a full-screen phone surface, so the chrome under it must go.
// Chat is a sheet now: it covers the bottom half at every height, so the controls bar below it
// still has to hide, but the top-right cluster stays — the chat toggle has to keep showing its
// active state, and the sheet stops short of the header band by design.
const mobileChromeHidden = participantsOpen
const mobileControlsHidden = chatOpen || participantsOpen
```

Then pass `mobileChromeHidden` to the top-right cluster's `cn(...)` at line 103 and
`mobileControlsHidden` to `MeetingControls`' `hideOnMobile` at line 150.

- [ ] **Step 2: Verify**

Run: `bun run check && bun run test`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/meeting/MeetingPanels.tsx
git commit -m "keep the meeting's top chrome visible behind the chat sheet"
```

---

### Task 7: Documentation

**Files:**
- Modify: `apps/web/AGENTS.md`
- Modify: `docs/plan/pwa-parity/01-overview-and-units.md`

- [ ] **Step 1: Add the AGENTS.md section**

After the existing "Phone dashboard" section:

```markdown
## Phone meeting chat

Below 1024px the in-call chat is a bottom sheet over the live call, not a full-screen surface.
`BedrudSheet` (`components/ui/BedrudSheet.tsx`) is the app's only bottom sheet; it is built on vaul
and fixes the corner (`rounded-t-3xl`), the container (`--meet-sidebar`), the handle, the bottom
safe-area inset and the gutter. Do not add a second sheet primitive, and do not turn those five into
props.

Two heights: half the visible viewport on open, and `--meet-sheet-max-height` when expanded — the
visible viewport minus 12px and the top safe-area inset. It expands on a drag up, on a handle tap,
and when the visible viewport shrinks far enough to be a keyboard (`components/ui/sheetKeyboard.ts`).
A drag below half dismisses. The drag itself is vaul's and is not reimplemented; there is no
TypeScript copy of the height, because the stylesheet is the one that draws with it.

`meeting.css` selects on `[data-chat-overlay="true"]` and `[data-elevated-chat="true"]` **without an
element qualifier**, because vaul renders a `div` where the desktop panel renders an `aside`.
Re-adding `aside` to those selectors silently breaks chat's text colours on the sheet;
`chatMarkers.test.ts` guards it.

Chat carries no meeting controls of its own. The strip of mic, deafen and participant count that
used to sit inside it existed only because the surface hid the whole call.
```

- [ ] **Step 2: Update the overview**

Set the unit 2 row's Status to `done, [08](./08-meeting-chat-sheet.md), [09](./09-meeting-chat-sheet-plan.md)`, and replace the component-vocabulary table's sheet line — "unit 2 decides Sheet vs a vaul-based Drawer, once for all sheets" — with `BedrudSheet`, `components/ui/BedrudSheet.tsx`, and note that units 3 and 4 reuse it.

- [ ] **Step 3: Commit**

```bash
git add apps/web/AGENTS.md docs/plan/pwa-parity/01-overview-and-units.md docs/plan/pwa-parity/08-meeting-chat-sheet.md docs/plan/pwa-parity/09-meeting-chat-sheet-plan.md
git commit -m "document the phone chat sheet and the sheet primitive"
```

---

## Verification checklist

Run before calling the unit done. Every line gets a real pass or fail.

- [ ] `bun run check` — clean but for the three known `noDocumentCookie` warnings in `api.test.ts`.
- [ ] `bun run test` — all green, and the count is the base count plus this unit's new tests.
- [ ] `rg -n "components/ui/sheet\"" apps/web/src` — no output; nothing imports the deleted primitive.
- [ ] `rg -n "(^|[\s\"'\`])(max-)?(sm|md):" apps/web/src/components/ui/BedrudSheet.tsx apps/web/src/components/ui/BedrudSheetHandle.tsx` — no output.
- [ ] On an Android emulator, Chrome against the dev server, installed to the home screen and launched standalone, in a room with a second participant:
  - [ ] Chat opens at half; tiles are visible above it.
  - [ ] Drag up reaches full; the room name band is still visible above the sheet.
  - [ ] Drag down from full returns to half; a second drag down dismisses.
  - [ ] The handle tap toggles half and full in both directions.
  - [ ] Focusing the composer expands to full and the composer sits above the keyboard.
  - [ ] Closing the keyboard does not collapse the sheet.
  - [ ] A message sent from the half sheet arrives on the second client.
  - [ ] The chat toggle in the top-right stays visible and shows its active state.
  - [ ] Both themes, and one landscape rotation.
- [ ] At 1280px, chat is still the 320px panel, the pin still works, and both dock sides still work.

## Deviations found during implementation

Recorded because the plan above is wrong in these four places, and reading it without them would
reintroduce two bugs that no test catches.

- **The sheet's height is set, not capped.** The plan specified
  `max-h-[var(--meet-sheet-max-height)]`. vaul measures its fractional snap points against the
  content's own height, so with only a cap the sheet stayed as tall as its content (254px on a
  375 × 812 phone) and was translated to `top: 964` — entirely off-screen, with every test green.
  It is `h-[var(--meet-sheet-max-height)]`.
- **`--snap-point-height` is an offset, not a height.** It is how far vaul has slid the sheet down —
  `406px` at half, `0px` at full. The sheet's body is
  `h-[calc(100%-3rem-var(--snap-point-height,0px))]`, so the composer stays inside the visible band.
  Using the variable as a height collapses the body to nothing once the sheet is fully open.
- **The handle tap is ours.** vaul 1.1.2's `Handle` closes the drawer when tapped at the last snap
  point instead of stepping back down, which contradicts the spec's dismissal table. `BedrudSheet`
  passes vaul's `preventCycle` and resolves the tap in `components/ui/sheetSnapPoints.ts`, a module
  the plan did not anticipate. `Drawer.Handle` takes no `asChild` in 1.1.2, so `BedrudSheetHandle`
  renders `Drawer.Handle` as its own root rather than being wrapped by it.
- **Deleting `mobileMeetingBar` cascaded further than the plan expected.** It was the only caller of
  `onOpenParticipantsFromChat` and `onCloseParticipants` on `ChatPanel`, which made
  `openParticipantsFromChat` and `participantsFromChatRef` unreachable in `MeetingRoomShell`, along
  with the branch of `closeParticipants` that reopened chat. All of it went, so `ChatPanel`'s props
  did change despite the plan saying they would not, and Task 5 touches `MeetingPanels.tsx` and
  `MeetingRoomShell.tsx` as well.

Task 6 also shipped a test the plan did not ask for, `meetingChrome.test.ts`, because a rendering
condition with no test regresses silently.

## Known leftovers

Filed as issues rather than fixed here, and each verified against current code before filing:

- `ParticipantsList` is still a full-screen phone surface. Unit 4 moves it onto `BedrudSheet`, and
  the wider mobile-layout backlog is tracked in #86.
- The meeting's own radius tokens in `meeting.css` keep their current values until unit 3.
- `Room` is declared twice, in the meeting route and in `RoomCard`, with `e2ee` required in one and
  optional in the other. Predates this unit; tracked in #157.
- `tsconfig.json` maps both `#/*` and `@/*` to `./src/*` and the codebase uses both. This unit's new
  and edited imports all take the `@/` form, which is what `components/ui/` already uses 25 times
  against 1; the rest of the tree is not swept. Tracked in #158.
- Nine inline `borderRadius` literals remain across six files, none of them touched here. Tracked
  in #159.
- Deleting the in-chat controls strip changes the premise of #39, which asked for meeting controls
  in the chat header because chat was a full-screen overlay. Recorded as a comment there rather than
  closed: the meeting controls bar is still hidden while the sheet is open.
