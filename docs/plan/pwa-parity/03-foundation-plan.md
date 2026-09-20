# 03 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the web app a well-formed installable PWA that shares the Android client's shape scale and phone breakpoint, so units 2 to 5 can rebuild screens on one foundation.

**Architecture:** Three token-level changes (radius scale in `styles.css`/`theme.css`, one `useIsMobile` hook in `lib/`, PWA metas in a `lib/document-head.ts` module) plus manifest and icon assets. No screen is redesigned. Every value that must agree across CSS, head and manifest is pinned by a contract test that reads the files.

**Tech Stack:** React 19, TanStack Start, TailwindCSS v4, Vitest 4 with jsdom and `@testing-library/react`, Bun, Biome, `sharp` (new devDependency) for icon rendering.

Spec: [02 — Foundation](./02-foundation.md). Program: [01 — Overview & Units](./01-overview-and-units.md).

## Global Constraints

- Worktree: `.claude/worktrees/feat+pwa-parity-foundation`, branch `feat/pwa-parity-foundation`, base `origin/master` at `2e853c8`. All commands run from `apps/web` inside it unless stated.
- **No commit, push or pull request without the user's explicit approval.** Each task ends at "ready to commit" with a proposed message; the commit happens only after the word is given.
- Commit messages follow the repo's `<action> <what> for <why>` form with actions `add`, `delete`, `update`. No attribution of any kind.
- Identifiers are full words (`mediaQueryList`, not `mq`). Comments sit above the code they describe, as complete sentences in the present tense, and say why.
- Toolchain: Bun and Biome. After every task: `bun run check` (Biome + tsc) and `bun run test` (Vitest) must pass. Baseline on this machine: 254 tests in 52 files pass, `check` reports 3 pre-existing warnings.
- Path alias: match the touched file. Files under `components/` import with `@/…`; `routes/__root.tsx` and `lib/` use `#/…`. Never `../src/…`.
- Radius classes: only `rounded`, `rounded-sm`, `rounded-md`, `rounded-lg`, `rounded-xl`, `rounded-2xl`, `rounded-3xl`, `rounded-full`, `rounded-none`. Never `rounded-[Npx]`.
- No user-facing strings are added by this unit, so no locale files change. `Bedrud` in the Apple title meta is the brand name.
- Node on this machine is 26; `apps/web/vite.config.ts` carries `execArgv: ['--no-experimental-webstorage']` so jsdom's storage wins. CI runs Node 22, which accepts the same flag.

## File map

| File | Responsibility | Task |
|---|---|---|
| `apps/web/vite.config.ts` | Vitest runs under Node 22+ with jsdom storage | 1 |
| `apps/web/src/lib/use-is-mobile.ts` (new) | the one breakpoint, `isMobileViewport()`, `useIsMobile()` | 2 |
| `apps/web/src/lib/use-is-mobile.test.ts` (new) | pins the breakpoint edge and change tracking | 2 |
| `components/dashboard/MobileOnlyGate.tsx`, `components/dashboard/CreateRoomDialog.tsx`, `components/meeting/ChatPanel.tsx`, `components/meeting/ControlsBar.tsx`, `components/meeting/MeetingRoomShell.tsx`, `components/meeting/ParticipantVideoSidebar.tsx` | consume the shared hook | 3 |
| `components/meeting/MeetingPanels.tsx`, `components/meeting/ParticipantsList.tsx`, `components/meeting/MeetingHeader.tsx`, `ChatPanel.tsx`, `ControlsBar.tsx` | `sm:` chrome classes become `lg:` | 3 |
| `apps/web/src/styles.css`, `apps/web/src/theme.css`, `components/meeting/meeting.css` | radius scale, universal override removed | 4 |
| `apps/web/src/design-tokens.test.ts` (new) | pins the six radius tokens and the absence of the override | 4 |
| `apps/web/src/node-fs.d.ts` (new) | types the one Node module the contract tests use, without pulling in Node's globals | 4 |
| twelve `rounded-[7px]` / `rounded-[10px]` / `rounded-[14px]` sites | move to `rounded-sm` / `rounded-md` / `rounded-xl` | 4 |
| `apps/web/src/lib/document-head.ts` (new) + test | metas and links for the root route | 5 |
| `apps/web/src/routes/__root.tsx` | uses the module; pre-paint script syncs theme-color | 5, 6 |
| `apps/web/src/lib/theme.store.ts` + test (new) | `applyTheme` copies `--background` into the theme-color meta | 6 |
| `apps/web/scripts/app-icon.svg` (new), `apps/web/scripts/pwa-icons.mjs` (new), `apps/web/public/apple-touch-icon.png` (generated), `apps/web/public/icon-maskable-512.png` (generated), `apps/web/public/README.md` (new), `apps/web/public/manifest.json`, `apps/web/package.json` | icons and manifest | 7 |
| `apps/web/src/pwa-manifest.test.ts` (new) | pins manifest fields and icon sizes | 7 |
| `DESIGN.md`, `apps/web/AGENTS.md`, `docs/plan/pwa-parity/01-overview-and-units.md` | docs | 8 |

---

### Task 1: Vitest under Node 22 and later

The fix is already in the working tree from the baseline investigation. This task only verifies and packages it. The user decides whether it ships as its own pull request or as the first commit of this branch.

**Files:**
- Modify: `apps/web/vite.config.ts:181-190` (already modified)

- [ ] **Step 1: Confirm the change is present**

`apps/web/vite.config.ts` test block reads:

```ts
  test: {
    environment: 'jsdom',
    globals: true,
    // Node 22 and later define their own `localStorage` and `sessionStorage` accessors on
    // the global object, which yield `undefined` unless `--localstorage-file` is set. The
    // jsdom environment leaves an existing global alone, so tests would see Node's empty
    // accessor instead of jsdom's Storage. Switching Node's Web Storage off restores jsdom's.
    execArgv: ['--no-experimental-webstorage'],
    setupFiles: [],
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    exclude: ['**/node_modules/**', '**/vendor/excalidraw/**'],
  },
```

- [ ] **Step 2: Run the suite**

Run: `bun run test`
Expected: `Test Files  52 passed (52)` and `Tests  254 passed (254)`.

- [ ] **Step 3: Ready to commit**

Proposed message: `update vitest config for jsdom storage on Node 22 and later`

---

### Task 2: One breakpoint hook

**Files:**
- Create: `apps/web/src/lib/use-is-mobile.ts`
- Test: `apps/web/src/lib/use-is-mobile.test.ts`

**Interfaces:**
- Produces: `MOBILE_BREAKPOINT_PX: 1024`, `MOBILE_MEDIA_QUERY: '(max-width: 1023px)'`, `isMobileViewport(): boolean` (safe to call in effects, handlers and on the server), `useIsMobile(): boolean` (render-time, `useSyncExternalStore`).

- [ ] **Step 1: Write the failing test**

`apps/web/src/lib/use-is-mobile.test.ts`:

```ts
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { isMobileViewport, MOBILE_BREAKPOINT_PX, MOBILE_MEDIA_QUERY, useIsMobile } from './use-is-mobile'

type ChangeListener = (event: MediaQueryListEvent) => void

const originalMatchMedia = window.matchMedia

/** Installs a fake `matchMedia` that evaluates `(max-width: Npx)` against a settable viewport width. */
function installMatchMedia(initialWidth: number) {
  let width = initialWidth
  const listeners = new Set<ChangeListener>()
  const maxWidthOf = (query: string) => Number(/max-width:\s*(\d+)px/.exec(query)?.[1])
  window.matchMedia = vi.fn((query: string) => ({
    get matches() {
      return width <= maxWidthOf(query)
    },
    media: query,
    onchange: null,
    addEventListener: (_type: 'change', listener: ChangeListener) => {
      listeners.add(listener)
    },
    removeEventListener: (_type: 'change', listener: ChangeListener) => {
      listeners.delete(listener)
    },
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia
  return {
    resize(nextWidth: number) {
      width = nextWidth
      for (const listener of listeners) listener({ media: MOBILE_MEDIA_QUERY } as MediaQueryListEvent)
    },
    listenerCount() {
      return listeners.size
    },
  }
}

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe('MOBILE_MEDIA_QUERY', () => {
  it('should stop one pixel below the breakpoint', () => {
    expect(MOBILE_BREAKPOINT_PX).toBe(1024)
    expect(MOBILE_MEDIA_QUERY).toBe('(max-width: 1023px)')
  })
})

describe('isMobileViewport', () => {
  it('should report mobile at 1023px', () => {
    installMatchMedia(1023)
    expect(isMobileViewport()).toBe(true)
  })

  it('should report desktop at 1024px', () => {
    installMatchMedia(1024)
    expect(isMobileViewport()).toBe(false)
  })
})

describe('useIsMobile', () => {
  it('should follow change events across the breakpoint', () => {
    const viewport = installMatchMedia(1024)
    const { result } = renderHook(() => useIsMobile())
    expect(result.current).toBe(false)
    act(() => viewport.resize(1023))
    expect(result.current).toBe(true)
    act(() => viewport.resize(1024))
    expect(result.current).toBe(false)
  })

  it('should stop listening after unmount', () => {
    const viewport = installMatchMedia(1024)
    const { unmount } = renderHook(() => useIsMobile())
    expect(viewport.listenerCount()).toBe(1)
    unmount()
    expect(viewport.listenerCount()).toBe(0)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/lib/use-is-mobile.test.ts`
Expected: FAIL, `Failed to resolve import "./use-is-mobile"`.

- [ ] **Step 3: Write the implementation**

`apps/web/src/lib/use-is-mobile.ts`:

```ts
import { useSyncExternalStore } from 'react'

/**
 * Widths below this get the phone layout. It equals Tailwind's `lg` breakpoint, which the
 * dashboard shell already uses for its sidebar and bottom navigation, so CSS and JS flip on
 * the same pixel. Tablets in portrait count as phones, as they do in the Android app.
 */
export const MOBILE_BREAKPOINT_PX = 1024

/** The media query behind `useIsMobile`; the one line CSS and JS both quote. */
export const MOBILE_MEDIA_QUERY = `(max-width: ${MOBILE_BREAKPOINT_PX - 1}px)`

/** Reads the viewport once. Use it inside effects and event handlers; renders use `useIsMobile`. */
export function isMobileViewport(): boolean {
  if (typeof window === 'undefined') return false
  return window.matchMedia(MOBILE_MEDIA_QUERY).matches
}

function subscribeToViewport(onChange: () => void): () => void {
  const mediaQueryList = window.matchMedia(MOBILE_MEDIA_QUERY)
  mediaQueryList.addEventListener('change', onChange)
  return () => mediaQueryList.removeEventListener('change', onChange)
}

function getServerSnapshot(): boolean {
  return false
}

/**
 * True below `MOBILE_BREAKPOINT_PX`. The server snapshot is `false`, so hydration renders the
 * desktop tree and React re-renders with the real width right after; the `lg:` classes keep
 * desktop chrome hidden on narrow screens in the meantime.
 */
export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribeToViewport, isMobileViewport, getServerSnapshot)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/lib/use-is-mobile.test.ts`
Expected: 5 tests pass.

- [ ] **Step 5: Lint and typecheck**

Run: `bun run check`
Expected: no new errors or warnings (3 pre-existing warnings remain).

- [ ] **Step 6: Ready to commit**

Proposed message: `add use-is-mobile hook for one phone breakpoint`

---

### Task 3: Every mobile check uses the hook, meeting chrome flips at `lg`

Mechanical consolidation. The hook's own tests cover the behaviour; these edits are verified by `bun run check`, the existing suite, and the device check in Task 9. No new test file: a render test asserting class names would restate the diff.

**Files:**
- Modify: `apps/web/src/components/dashboard/MobileOnlyGate.tsx`
- Modify: `apps/web/src/components/dashboard/CreateRoomDialog.tsx:95-128`
- Modify: `apps/web/src/components/meeting/ChatPanel.tsx:24-25,62-74,96,213-276`
- Modify: `apps/web/src/components/meeting/ControlsBar.tsx:90-104,183,291`
- Modify: `apps/web/src/components/meeting/MeetingRoomShell.tsx:27-30`
- Modify: `apps/web/src/components/meeting/ParticipantVideoSidebar.tsx:1-27,59`
- Modify: `apps/web/src/components/meeting/MeetingPanels.tsx:90,103,115`
- Modify: `apps/web/src/components/meeting/ParticipantsList.tsx:54,58,69,72`
- Modify: `apps/web/src/components/meeting/MeetingHeader.tsx:90,92,101,187`

**Interfaces:**
- Consumes: `isMobileViewport`, `useIsMobile` from `@/lib/use-is-mobile` (Task 2).

- [ ] **Step 1: MobileOnlyGate**

Replace the whole file with:

```tsx
import { useNavigate } from '@tanstack/react-router'
import { type ReactNode, useEffect, useState } from 'react'
import { MobileBottomNav } from '@/components/dashboard/MobileBottomNav'
import { isMobileViewport } from '@/lib/use-is-mobile'

export function MobileOnlyGate({ desktopTo = '/dashboard', children }: { desktopTo?: string; children: ReactNode }) {
  const navigate = useNavigate()
  const [ready, setReady] = useState(false)

  // Reads the viewport inside the effect rather than through the hook: the hook's server
  // snapshot is "desktop", and an effect keyed on it would bounce phones away on hydration.
  useEffect(() => {
    if (!isMobileViewport()) {
      navigate({ to: desktopTo, replace: true })
      return
    }
    setReady(true)
  }, [desktopTo, navigate])

  if (!ready) return null

  return (
    <div className="min-h-screen bg-background lg:hidden">
      <main id="main-content" className="p-4 pb-[calc(5.5rem+env(safe-area-inset-bottom,0px))]">
        {children}
      </main>
      <MobileBottomNav />
    </div>
  )
}
```

- [ ] **Step 2: CreateRoomDialog**

Add the import next to the other `@/lib` imports:

```ts
import { isMobileViewport } from '@/lib/use-is-mobile'
```

Replace both `window.matchMedia('(max-width: 767px)').matches` conditions:

```ts
  useEffect(() => {
    if (!open) return
    if (isMobileViewport()) {
      setName(randomRoomName())
    }
  }, [open])
```

```ts
  function handleOpenAutoFocus(e: Event) {
    if (isMobileViewport()) {
      e.preventDefault()
      return
    }
    e.preventDefault()
    nameInputRef.current?.focus()
  }
```

- [ ] **Step 3: ChatPanel**

Delete lines 24-25 (`MOBILE_MAX_WIDTH_MQ` and its comment) and the `useIsMobileChat` function (lines 62-74). Add the import beside `@/lib/utils`:

```ts
import { useIsMobile } from '@/lib/use-is-mobile'
```

Change line 96 to `const isMobile = useIsMobile()`. If `useState` is no longer used anywhere in the file, drop it from the `react` import (Biome reports it).

Class changes, `sm` to `lg`:

| Line | Old | New |
|---|---|---|
| 213 | `px-3 sm:h-[52px] sm:px-4` | `px-3 lg:h-[52px] lg:px-4` |
| 215 | `gap-1 sm:gap-2` | `gap-1 lg:gap-2` |
| 220 | `'max-sm:hidden'` | `'max-lg:hidden'` |
| 230 | `'h-11 w-11 max-sm:rounded-lg'` | `'h-11 w-11 max-lg:rounded-lg'` |
| 273 | `'sm:fixed sm:top-0 sm:h-full sm:max-h-none sm:w-[min(320px,var(--app-width,100svw))] sm:max-w-none sm:pt-[env(safe-area-inset-top,0px)] sm:pb-[env(safe-area-inset-bottom,0px)]'` | same string with every `sm:` replaced by `lg:` |
| 275 | `'sm:left-0 sm:right-auto sm:border-r sm:border-[var(--meet-border-subtle)]'` | `'lg:left-0 lg:right-auto lg:border-r lg:border-[var(--meet-border-subtle)]'` |
| 276 | `'sm:left-auto sm:right-0 sm:border-l sm:border-[var(--meet-border-subtle)]'` | `'lg:left-auto lg:right-0 lg:border-l lg:border-[var(--meet-border-subtle)]'` |

- [ ] **Step 4: ControlsBar**

Delete the local `useIsMobile` (lines 90-104, including the `/* ── Mobile detection … */` banner). Add the import beside the other `@/lib` imports:

```ts
import { useIsMobile } from '@/lib/use-is-mobile'
```

Line 291 stays `const isMobile = useIsMobile()`. Line 183:

```ts
const dividerCn = 'w-px h-7 bg-[var(--meet-border)] mx-0.5 shrink-0 max-lg:hidden'
```

If `useState` or `useEffect` become unused, drop them from the `react` import.

- [ ] **Step 5: MeetingRoomShell**

Add the import beside the other `@/` imports:

```ts
import { isMobileViewport } from '@/lib/use-is-mobile'
```

Replace lines 27-30:

```ts
  // Desktop: open the chat sidebar by default. Phone: closed, chat opens over the call.
  const [chatOpen, setChatOpen] = useState(() => !isMobileViewport())
```

`isMobileViewport()` returns `false` on the server, so the server-rendered state stays `true` as before.

- [ ] **Step 6: ParticipantVideoSidebar**

Delete `useIsMobileFilmstrip` (lines 15-27). Add the import beside `@/lib/utils`:

```ts
import { useIsMobile } from '@/lib/use-is-mobile'
```

Line 59 becomes `const isMobile = useIsMobile()`. Drop `useEffect` and `useState` from the `react` import if nothing else in the file uses them.

- [ ] **Step 7: MeetingPanels, ParticipantsList, MeetingHeader**

| File:line | Old | New |
|---|---|---|
| MeetingPanels.tsx:90 | `"absolute z-[25] hidden items-center gap-2 sm:flex"` | `"absolute z-[25] hidden items-center gap-2 lg:flex"` |
| MeetingPanels.tsx:103 | `'absolute z-[25] flex h-9 items-center gap-2 sm:hidden'` | `'absolute z-[25] flex h-9 items-center gap-2 lg:hidden'` |
| MeetingPanels.tsx:115 | `className="hidden sm:flex"` | `className="hidden lg:flex"` |
| ParticipantsList.tsx:54 | the string starting `'sm:fixed sm:inset-y-0 …'` | same string with every `sm:` replaced by `lg:` |
| ParticipantsList.tsx:58 | `px-3 sm:h-[52px] sm:px-4` | `px-3 lg:h-[52px] lg:px-4` |
| ParticipantsList.tsx:69 | `sm:h-7 sm:w-7` | `lg:h-7 lg:w-7` |
| ParticipantsList.tsx:72 | `className="sm:h-[15px] sm:w-[15px]"` | `className="lg:h-[15px] lg:w-[15px]"` |
| MeetingHeader.tsx:90 | `max-sm:pe-[calc(100px+env(safe-area-inset-right,0px))]` | `max-lg:pe-[calc(100px+env(safe-area-inset-right,0px))]` |
| MeetingHeader.tsx:92 | `'sm:justify-center sm:px-4'` | `'lg:justify-center lg:px-4'` |
| MeetingHeader.tsx:101 | `'hidden sm:flex items-center …'` | `'hidden lg:flex items-center …'` |
| MeetingHeader.tsx:187 | `sm:inline` | `lg:inline` |
| MeetingUILayoutContext.tsx:38,41,44,58,77,80,83 | the `sm:right-[…]`, `max-sm:bottom-[…]` and `sm:left-[…]` returns | the same strings with `sm:` → `lg:` and `max-sm:` → `max-lg:` |
| MeetingControls.tsx:56 | `'max-sm:hidden'` | `'max-lg:hidden'` (the dialog's `sm:max-w-sm` and `sm:flex-col` stay) |
| chat/ChatInput.tsx:402 | `sm:pb-1.5` | `lg:pb-1.5` |
| RoomInfoPanel.tsx:245-249,253,256 | every `sm:` / `max-sm:` in the dialog-content block and the two body wrappers | the same strings with `sm:` → `lg:` and `max-sm:` → `max-lg:` |
| webxdc/WebxdcAppsDialog.tsx:211-212 | every `sm:` / `max-sm:` in the dialog-content block | the same strings with `sm:` → `lg:` and `max-sm:` → `max-lg:` (the `sm:grid-cols-3` grids stay) |
| webxdc/WebxdcStageOverlay.tsx:20,35 | `meetStageShellClass(layout, 'p-3 max-sm:p-2')`, `… 'p-3 max-sm:p-1.5'` | the same calls with `max-lg:` |
| stage/StageScreenShareOverlay.tsx:160,223 | `meetStageShellClass(layout, 'p-3 max-sm:p-2')` | the same call with `max-lg:` |
| youtube/YoutubeWatchOverlay.tsx:212 | `meetStageShellClass(layout, 'p-2 max-sm:p-1.5')` | the same call with `max-lg:` |
| whiteboard/WhiteboardOverlay.tsx:47 | `meetStageShellClass(layout, 'p-3 max-sm:p-1.5')` | the same call with `max-lg:` |
| webxdc/WebxdcFrame.tsx:82 | `px-2 py-1.5 sm:px-3 sm:py-2` | `px-2 py-1.5 lg:px-3 lg:py-2` |
| youtube/YoutubeWatchOverlay.tsx:35 | `px-2 py-1.5 sm:px-3 sm:py-2` | `px-2 py-1.5 lg:px-3 lg:py-2` |
| stage/StageScreenShareOverlay.tsx:43 | `px-2 py-1.5 sm:px-3 sm:py-2` | `px-2 py-1.5 lg:px-3 lg:py-2` |

The five overlay call sites pass their padding as the `extra` argument of `meetStageShellClass`, so a `max-sm:` there and the function's own `max-lg:` insets would disagree between 640px and 1023px: the shell would take the phone bottom inset while its padding stayed at the desktop value.

The invariant that ties these prefixes to `MOBILE_BREAKPOINT_PX` is a file-level doc comment at the top of `MeetingUILayoutContext.tsx`, above the first export, so it governs all four exported helpers rather than only the one it sits inside.

- [ ] **Step 8: Confirm no width check is left behind**

Run: `grep -rn "matchMedia" src/components src/routes`
Expected: no output. (`lib/theme.store.ts` and `routes/__root.tsx` keep their `prefers-color-scheme` queries; those are theme, not width.)

Run: `grep -rn "sm:" src/components/meeting/MeetingPanels.tsx src/components/meeting/ParticipantsList.tsx src/components/meeting/ChatPanel.tsx src/components/meeting/ControlsBar.tsx src/components/meeting/MeetingHeader.tsx`
Expected: no output.

Run: `grep -rnoE "(max-)?sm:[a-zA-Z0-9_.:/\[\]-]+" src/components/meeting src/routes/m.\$meetId.tsx`
Expected: only shadcn dialog and grid conventions, which are unrelated to the phone hook — `sm:max-w-*`, `sm:justify-end`, `sm:justify-center`, `sm:gap-0`, `sm:flex-row`, `sm:flex-col`, `sm:grid-cols-*`. Anything that controls layout padding, insets, visibility or position belongs at `lg`.

- [ ] **Step 9: Check and test**

Run: `bun run check` then `bun run test`
Expected: check clean, 259 tests pass (254 plus the 5 from Task 2).

- [ ] **Step 10: Ready to commit**

Proposed message: `update mobile checks to share one breakpoint for phone layout parity`

---

### Task 4: Radius scale

**Files:**
- Test: `apps/web/src/design-tokens.test.ts`
- Create: `apps/web/src/node-fs.d.ts`
- Modify: `apps/web/src/styles.css:26-29,53-56,90-101,204`
- Modify: `apps/web/src/theme.css:4,85`
- Modify: `apps/web/src/components/meeting/meeting.css:26`
- Modify: twelve class sites listed in Step 5

- [ ] **Step 1: Write the failing test**

Contract tests read files from disk, which needs two things this browser-only project lacks. First, jsdom's `URL` resolves `import.meta.url` against `http://localhost`, so file-reading tests opt into Vitest's Node environment with a header comment. Second, `tsconfig.json` leaves `@types/node` out on purpose (its `setTimeout` collides with the DOM's in `MeetingStageContext.tsx`), so `apps/web/src/node-fs.d.ts` declares the one module the tests use:

```ts
/**
 * Contract tests read stylesheets, the web manifest and icon bytes straight from disk. The
 * app's tsconfig leaves Node's type package out on purpose, because its globals (for
 * example `setTimeout`) collide with the browser's, so the one Node module those tests use
 * is declared here with only the shapes they call.
 */
declare module 'node:fs' {
  /** The part of Node's Buffer the tests need to read a PNG's dimensions. */
  export interface FileBytes {
    readUInt32BE(offset: number): number
  }

  export function readFileSync(path: string | URL, encoding: 'utf8'): string
  export function readFileSync(path: string | URL): FileBytes
}
```

`apps/web/src/design-tokens.test.ts`:

```ts
// @vitest-environment node
//
// This file reads stylesheets from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const stylesCss = readFileSync(new URL('./styles.css', import.meta.url), 'utf8')
const themeCss = readFileSync(new URL('./theme.css', import.meta.url), 'utf8')

/** Reads the first `--name: value;` declaration from a stylesheet. */
function tokenValue(css: string, name: string): string | undefined {
  return new RegExp(`${name}:\\s*([^;]+);`).exec(css)?.[1].trim()
}

describe('radius tokens', () => {
  it.each([
    ['--radius-sm', '8px'],
    ['--radius-md', 'var(--radius)'],
    ['--radius-lg', 'var(--radius)'],
    ['--radius-xl', '16px'],
    ['--radius-2xl', '20px'],
    ['--radius-3xl', '28px'],
  ])('should set %s to %s', (name, value) => {
    expect(tokenValue(stylesCss, name)).toBe(value)
  })

  it('should set the default corner to the button and field size', () => {
    expect(tokenValue(themeCss, '--radius')).toBe('12px')
  })

  it('should not flatten every element with a universal border-radius', () => {
    expect(stylesCss).not.toMatch(/\*\s*\{[^}]*border-radius/)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/design-tokens.test.ts`
Expected: 8 failures (`4px` where `8px`/`12px`/… expected, `--radius-2xl`/`3xl` undefined, universal rule present), 0 passes.

- [ ] **Step 3: styles.css**

Lines 26-29, the banner:

```css
/* ═══════════════════════════════════════════════════════════════════════════
   Bedrud Design Tokens — Rose + Teal
   Rounded corners on the Android client's scale. Bold colors. No purple.
   ═══════════════════════════════════════════════════════════════════════════ */
```

Lines 53-56 become the scale. Android's `Shape.kt` tokens are named in the comments so the two clients can be compared line by line:

```css
  /* Shape scale, equal to the Android client's BedrudRadius (Shape.kt). */
  /* chip: sm */
  --radius-sm: 8px;
  /* button: md, resolved through --radius so a self-hoster retunes it in theme.css */
  --radius-md: var(--radius);
  /* field: md, the same knob as the button corner */
  --radius-lg: var(--radius);
  /* card: lg */
  --radius-xl: 16px;
  /* xl */
  --radius-2xl: 20px;
  /* sheet top, video tile, controls bar: xxl */
  --radius-3xl: 28px;
```

Lines 90-101: delete the `border-radius: var(--radius, 4px) !important;` line from the `*` rule and replace the comment that follows it. The `.meet-*` exceptions keep their `!important`, because they override utility classes on the same elements:

```css
@layer base {
  * {
    border-color: var(--border);
    box-sizing: border-box;
  }
  /*
   * Meeting chrome radii override any rounded-* utility on the same element, so they keep
   * !important. Tokens: --meet-*-radius in meeting.css.
   */
  .meet-controls-bar {
```

Line 204, the scrollbar thumb, drops the override and the fallback:

```css
  border-radius: var(--radius);
```

- [ ] **Step 4: theme.css and meeting.css**

`theme.css` line 4:

```css
 * Rounded corners. Bold colors. No purple.
```

`theme.css` line 85:

```css
  /* The default corner: buttons and fields. The full scale is in styles.css. */
  --radius: 12px;
```

`meeting.css` line 26:

```css
  /* Chrome radii; the @layer base exceptions in styles.css apply these over rounded-* utilities. */
```

- [ ] **Step 5: Replace the twelve arbitrary radii**

`rounded-[7px]` becomes `rounded-sm` (8px); `rounded-[10px]` becomes `rounded-md` (12px); the chat toast's `rounded-[14px]` becomes `rounded-xl` (16px, the card corner). One class token each, nothing else on the line changes.

| File:line | Old class | New class |
|---|---|---|
| components/meeting/ChatPanel.tsx:56 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/MeetingHeader.tsx:101 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/ParticipantsList.tsx:69 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/ParticipantTile.tsx:276 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/ParticipantVideoSidebar.tsx:118 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/ScreenShareTile.tsx:23 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/SecureContextBanner.tsx:22 | `rounded-[7px]` | `rounded-sm` |
| components/meeting/ControlsBar.tsx:111 | `rounded-[10px]` | `rounded-md` |
| components/meeting/ControlsBar.tsx:892 | `rounded-[10px]` | `rounded-md` |
| components/meeting/ParticipantVideoSidebar.tsx:46 | `rounded-[10px]` | `rounded-md` |
| components/meeting/RecordingButton.tsx:15 | `rounded-[10px]` | `rounded-md` |
| components/meeting/ChatToastNotifier.tsx:66 | `rounded-[14px]` | `rounded-xl` |

Line numbers are those before Task 3's edits; search for the class, not the number.

- [ ] **Step 5b: Restore the primitives' corners**

The shadcn primitives under `components/ui/` lost every `rounded-*` class when the repo was created for the old 0px design, so this scale is otherwise invisible on `<Button>`, `<Input>`, `<Card>` and the rest.

Exactly one corner class per primitive: `cn` is `twMerge(clsx(…))`, so a second `rounded-*` in the same string silently deletes the first. `badge.tsx` is the chip and carries `rounded-sm` alone — no `rounded-full` beside it. `alert.tsx` is the inline banner and takes `rounded-lg`, the same corner as the hand-rolled banners it shares a screen with, not the card's `rounded-xl`. `command.tsx`'s `<DialogContent>` does not restate `rounded-3xl`; the dialog primitive owns that corner.

| file | class |
|---|---|
| `button.tsx` | `rounded-md` |
| `input.tsx` | `rounded-lg` |
| `select.tsx` | `rounded-lg` |
| `card.tsx` | `rounded-xl` |
| `alert.tsx` | `rounded-lg` |
| `dialog.tsx` | `rounded-3xl` |
| `badge.tsx` | `rounded-sm` |
| `tabs.tsx` | `rounded-lg` |
| `dropdown-menu.tsx` | `rounded-md` |
| `context-menu.tsx` | `rounded-md` |
| `popover.tsx` | `rounded-md` |
| `command.tsx` | `rounded-md` |
| `tooltip.tsx` | `rounded` |
| `skeleton.tsx` | `rounded-md` |
| `checkbox.tsx` | `rounded` |

- [ ] **Step 5c: Give the raw bordered boxes a corner**

Deleting the universal `border-radius` override leaves every hand-rolled bordered box square, because only the shadcn primitives carry a corner class of their own. Rule: a bordered box that is a card or a selectable option gets `rounded-xl`; an inline banner, row container, segmented-control shell or compound input gets `rounded-lg`.

| file | box | class |
|---|---|---|
| `routes/settings.tsx` | category list | `rounded-xl` |
| `routes/index.tsx` | recent-room row | `rounded-lg` |
| `routes/auth.register.tsx`, `routes/auth.reset-password.tsx`, `routes/auth.forgot-password.tsx` | destructive banners | `rounded-lg` |
| `routes/dashboard/settings.tsx` | section nav shell | `rounded-lg` |
| `routes/dashboard/admin/*.tsx` | destructive banners and rows | `rounded-lg` |
| `routes/dashboard/admin/*.tsx` | stat tiles, table and list containers, empty states | `rounded-xl` |
| `components/dashboard/CreateRoomDialog.tsx` | visibility option tiles | `rounded-xl` |
| `components/dashboard/CreateRoomDialog.tsx`, `components/dashboard/RoomSettingsDialog.tsx` | destructive banners | `rounded-lg` |
| `components/dashboard/RoomSettingsDialog.tsx` | segmented shell | `rounded-lg` |
| `components/auth/PasskeyButton.tsx` | info box | `rounded-lg` |
| `components/settings/SecuritySettingsPanel.tsx` | info row, status banner | `rounded-lg` |
| `components/settings/BedrudSettingsDialog.tsx` | category list | `rounded-xl` |
| `components/admin/RoomEventsTable.tsx`, `components/admin/RecentSignupsTable.tsx` | table containers, empty states | `rounded-xl` |
| `components/admin/DataTableToolbar.tsx`, `components/admin/AdminControlBar.tsx` | toolbar cards | `rounded-xl` |
| `components/admin/DataTableBulkBar.tsx`, `components/admin/DataTablePagination.tsx` | row containers | `rounded-lg` |
| `components/admin/DataTableSearch.tsx` | compound input | `rounded-lg` |
| `components/admin/DataTableFacetedFilter.tsx` | checkbox square | `rounded` |
| `components/admin/overview/index.tsx`, `components/admin/queue-stats.tsx`, `components/admin/settings/server-tab.tsx` | inline banners | `rounded-lg` |
| `components/admin/settings/shared.tsx`, `components/admin/settings/invite-tokens-section.tsx` | section cards | `rounded-xl` |
| `components/admin/settings/invite-tokens-section.tsx` | status chips | `rounded-sm` |
| `components/admin/settings/invite-tokens-section.tsx` | confirm and token rows | `rounded-lg` |
| `components/admin/settings/invite-tokens-section.tsx` | raw `<input>` and `<select>` | `rounded-lg` |
| `components/admin/settings/general-tab.tsx` | access-mode option tiles | `rounded-xl` |
| `components/admin/settings/webxdc-tab.tsx` | catalog tile, package icon, drop zone | `rounded-xl` |
| `components/admin/settings/webxdc-tab.tsx` | raw `<textarea>` | `rounded-lg` |
| `components/ErrorPage.tsx` | 64 × 64 icon square | `rounded-xl` |
| `components/meeting/webxdc/WebxdcFrame.tsx` | floating mini-app window (square when expanded) | `rounded-xl` |
| `components/meeting/chat/ChatInput.tsx` | 64 × 64 attachment thumbnail | `rounded-xl` |

A filled box with no border was left square by the same sweep, because the deleted override flattened those too. Same sizes, keyed on the fill instead: a segmented shell gets `rounded-lg` and its items `rounded-md`, a nav row `rounded-md`, an icon chip or icon-button hover fill `rounded-sm`, a padded `bg-muted` block `rounded-lg`.

| file | box | class |
|---|---|---|
| `routes/auth.tsx` | mode switcher shell, its items | `rounded-lg`, `rounded-md` |
| `routes/dashboard/settings.tsx` | section nav items | `rounded-md` |
| `routes/dashboard.tsx` | sidebar nav rows, account row | `rounded-md` |
| `routes/settings.tsx`, `components/settings/BedrudSettingsDialog.tsx` | 32 × 32 icon chips | `rounded-sm` |
| `components/admin/overview/recent-events.tsx`, `components/admin/RoomEventsTable.tsx` | 28 × 28 event icon chips | `rounded-sm` |
| `components/admin/settings/general-tab.tsx` | 32 × 32 mode icon chips | `rounded-sm` |
| `components/admin/settings/invite-tokens-section.tsx`, `routes/dashboard/archived_.$roomId.tsx`, `routes/dashboard/admin/rooms_.$roomId.tsx` | icon buttons with a hover fill | `rounded-sm` |
| `routes/dashboard/admin/users_.$userId.tsx` | `bg-muted p-3` confirmation blocks | `rounded-lg` |

Hand-rolled controls filled with `bg-primary`, `bg-destructive` or a hover tint were flattened by the same override and sit beside shadcn buttons on the same screens. A text button takes the button corner, an icon-only button matches the siblings in its group, the 24 × 24 brand mark matches `BedrudLogo.tsx`, and a class string that lives in a `.ts` helper counts like one in a `.tsx` file.

| file | box | class |
|---|---|---|
| `components/settings/settingsPanelTone.ts` | `panelSurfaceClass` panel surface, both tones (pinned by `settingsPanelTone.test.ts`) | `rounded-xl` |
| `components/ErrorPage.tsx`, `routes/index.tsx` | primary CTA links beside a `<Button>` | `rounded-lg` |
| `components/ErrorPage.tsx`, `routes/dashboard.tsx` | 24 × 24 brand mark | `rounded` |
| `components/admin/settings/invite-tokens-section.tsx` | generate, confirm and delete buttons | `rounded-lg` |
| `components/admin/settings/invite-tokens-section.tsx`, `routes/dashboard/admin/rooms_.$roomId.tsx` | icon-only buttons in a group with a `rounded-sm` sibling | `rounded-sm` |
| `components/admin/settings/webxdc-tab.tsx` | 28 × 28 icon button with a hover fill, standing alone on a catalog tile | `rounded-sm` |
| `routes/dashboard/admin/rooms_.$roomId.tsx` | kick confirm button | `rounded-lg` |
| `routes/index.tsx` | join-form error banner | `rounded-lg` |
| `components/meeting/presence/MeetingPresenceCursors.tsx` | presence name label (inline fill and border) | `rounded-sm` |
| `components/meeting/chat/ChatInput.tsx` | 20 × 20 attachment close chip | `rounded-sm` |

Boxes that already carried the 4px `rounded` were flattened by the override too, so the 4px only became visible when it went; on this scale 4px belongs to the brand mark, the checkbox square and the tooltip alone, and the rest take the corner their shape calls for:

| file | box | class |
|---|---|---|
| `components/admin/settings/email-tab.tsx` | 32 × 32 colour swatches | `rounded-sm` |
| `components/admin/settings/email-tab.tsx`, `components/admin/queue-stats.tsx` | result and last-error banners | `rounded-lg` |
| `components/admin/overview/index.tsx` | chart loading placeholder | `rounded-xl` |
| `components/meeting/ParticipantsList.tsx`, `components/meeting/webxdc/WebxdcPanel.tsx`, `routes/dashboard/admin/users.tsx`, `routes/dashboard.tsx` | role, experimental, and restricted chips | `rounded-sm` |
| `routes/dashboard.tsx` | sign-out `<Button size="icon">` | override removed, keeps the button corner |
| `routes/dashboard/admin/rooms_.events.tsx`, `routes/dashboard/admin/users_.recent-signups.tsx` | raw date inputs | `rounded-lg` |

Find the rest with `grep -rn --include="*.tsx" --include="*.ts" -E '["'"'"'` ]border["'"'"'` ]' src | grep -v rounded` over the whole of `src`, the filled boxes with `grep -rn --include="*.tsx" --include="*.ts" -E '(^|["'"'"' ])(bg-primary|bg-destructive|bg-secondary|bg-accent|bg-amber-|bg-emerald-|bg-red-|bg-\[var\(--|hover:bg-)' src | grep -v rounded`, and the boxes still on 4px with `grep -rnE "['\" ]rounded['\" ]" src | grep -E 'border|bg-'`. Boxes built on a shadcn primitive (`Card`, `DialogContent`, `Input`, `Badge`, `Button`, `Skeleton`, `RadioGroupItem`) already carry a corner and are left alone, as are `divide-border` dividers, the `.meet-*` boxes whose radius comes from a `meeting.css` token, meter segments and progress fills inside a clipped track, and full-bleed rows and strips inside a container that already clips them.

- [ ] **Step 6: Run the test to verify it passes**

Run: `bunx vitest run src/design-tokens.test.ts`
Expected: 8 tests pass.

Run: `grep -rn "rounded-\[" src/components src/routes`
Expected: only `rounded-[inherit]` (one line); `BedrudLogo.tsx`'s `rounded-[4px]` becomes `rounded`.

- [ ] **Step 7: See it rendered**

Run: `bun run dev` (from `apps/web`), open `http://localhost:7070/auth/login` at 390 px wide in the browser (Playwright MCP or the in-app browser), light and dark. Buttons, inputs and the card must show visibly rounded corners; the avatar stays a circle. Then `/` at 1280 px.

- [ ] **Step 8: Check and test**

Run: `bun run check` then `bun run test`
Expected: check clean, 267 tests pass.

- [ ] **Step 9: Ready to commit**

Proposed message: `update radius tokens to the Android shape scale for one brand across clients`

---

### Task 5: Document head module

**Files:**
- Create: `apps/web/src/lib/document-head.ts`
- Test: `apps/web/src/lib/document-head.test.ts`
- Modify: `apps/web/src/routes/__root.tsx:61-75`

**Interfaces:**
- Produces: `APP_NAME = 'Bedrud'`, `LIGHT_THEME_COLOR = '#fafafa'`, `documentMeta: DocumentMeta[]`, `documentLinks(stylesheetHref: string): DocumentLink[]`. Task 7's manifest test imports `LIGHT_THEME_COLOR`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/lib/document-head.test.ts` (Node environment: it reads `theme.css` from disk, see Task 4 Step 1):

```ts
// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { APP_NAME, documentLinks, documentMeta, LIGHT_THEME_COLOR } from './document-head'

function metaContent(name: string): string | undefined {
  const tag = documentMeta.find((meta) => meta.name === name)
  return typeof tag?.content === 'string' ? tag.content : undefined
}

describe('documentMeta', () => {
  it('should let the viewport reach under the system bars', () => {
    expect(metaContent('viewport')).toBe('width=device-width, initial-scale=1, viewport-fit=cover')
  })

  it('should leave the theme-color meta to the pre-paint script', () => {
    expect(documentMeta.some((meta) => meta.name === 'theme-color')).toBe(false)
  })

  it('should keep the theme colour equal to the light background token', () => {
    const themeCss = readFileSync(new URL('../theme.css', import.meta.url), 'utf8')
    expect(/--background:\s*([^;]+);/.exec(themeCss)?.[1].trim()).toBe(LIGHT_THEME_COLOR)
  })

  it('should declare the page installable on Android and iOS', () => {
    expect(metaContent('mobile-web-app-capable')).toBe('yes')
    expect(metaContent('apple-mobile-web-app-title')).toBe(APP_NAME)
    expect(metaContent('apple-mobile-web-app-status-bar-style')).toBe('default')
  })
})

describe('documentLinks', () => {
  it('should put the stylesheet first and link the manifest and Apple touch icon', () => {
    const links = documentLinks('/assets/styles.css')
    expect(links[0]).toEqual({ rel: 'stylesheet', href: '/assets/styles.css' })
    expect(links).toContainEqual({ rel: 'manifest', href: '/manifest.json' })
    expect(links).toContainEqual({ rel: 'apple-touch-icon', href: '/apple-touch-icon.png' })
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/lib/document-head.test.ts`
Expected: FAIL, `Failed to resolve import "./document-head"`.

- [ ] **Step 3: Write the module**

`apps/web/src/lib/document-head.ts`:

```ts
import type { JSX } from 'react'

type DocumentMeta = JSX.IntrinsicElements['meta'] & { title?: string }
type DocumentLink = JSX.IntrinsicElements['link']

export const APP_NAME = 'Bedrud'

/**
 * The light `--background` from theme.css, for the manifest's own `theme_color`. Nothing here
 * renders a theme-color meta: the system bar colour comes from the CSS token, written by the
 * pre-paint script in the root route and kept current by `applyTheme`. This constant exists only
 * so the two places that cannot read CSS stay pinned to each other — `document-head.test.ts` pins
 * it to theme.css and `pwa-manifest.test.ts` pins manifest.json to it.
 */
export const LIGHT_THEME_COLOR = '#fafafa'

export const documentMeta: DocumentMeta[] = [
  { charSet: 'utf-8' },
  { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
  { title: APP_NAME },
  { name: 'mobile-web-app-capable', content: 'yes' },
  { name: 'apple-mobile-web-app-title', content: APP_NAME },
  // `black-translucent` would force white status text over the light theme.
  { name: 'apple-mobile-web-app-status-bar-style', content: 'default' },
]

/** Head links; the stylesheet URL comes from Vite's `?url` import in the root route. */
export function documentLinks(stylesheetHref: string): DocumentLink[] {
  return [
    { rel: 'stylesheet', href: stylesheetHref },
    { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
    { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
    { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
    { rel: 'manifest', href: '/manifest.json' },
  ]
}
```

If `bun run check` rejects `documentMeta` where `createRootRoute` expects its own meta type, widen the array type to `Array<React.JSX.IntrinsicElements['meta'] & { title?: string }>` at the call site by spreading: `meta: [...documentMeta]`.

- [ ] **Step 4: Use it in the root route**

`apps/web/src/routes/__root.tsx`: add the import beside the other `#/lib` imports:

```ts
import { documentLinks, documentMeta } from '#/lib/document-head'
```

Replace the `head` block (lines 62-75):

```ts
  head: () => ({
    meta: documentMeta,
    links: documentLinks(appCss),
    scripts: [{ children: themeScript }, { children: viewportScript }],
  }),
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bunx vitest run src/lib/document-head.test.ts`
Expected: 5 tests pass.

- [ ] **Step 6: Check and test**

Run: `bun run check` then `bun run test`
Expected: check clean, 272 tests pass.

- [ ] **Step 7: See the metas**

Run: `bun run dev`, load `http://localhost:7070/` and read the served `<head>`: the four new metas and the `apple-touch-icon` link are present in the server-rendered HTML (view source or `curl -s http://localhost:7070/ | grep -o '<meta[^>]*>'`).

- [ ] **Step 8: Ready to commit**

Proposed message: `add document head module for PWA metas on every page`

---

### Task 6: Theme colour follows the app theme

**Files:**
- Test: `apps/web/src/lib/theme.store.test.ts`
- Modify: `apps/web/src/lib/theme.store.ts:38-43`
- Modify: `apps/web/src/routes/__root.tsx:15-25` (pre-paint script)

- [ ] **Step 1: Write the failing test**

`apps/web/src/lib/theme.store.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyTheme } from './theme.store'

/** Makes `--background` resolve to `value` without loading the real stylesheet into jsdom. */
function installBackgroundToken(value: string) {
  vi.spyOn(window, 'getComputedStyle').mockReturnValue({
    getPropertyValue: () => ` ${value} `,
  } as unknown as CSSStyleDeclaration)
}

function themeColorMeta(): string | null | undefined {
  return document.querySelector('meta[name="theme-color"]')?.getAttribute('content')
}

describe('applyTheme', () => {
  beforeEach(() => {
    document.head.innerHTML = '<meta name="theme-color" content="#fafafa">'
    document.documentElement.classList.remove('dark')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should toggle the dark class on the root element', () => {
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('should copy the resolved background token into the theme-color meta', () => {
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(themeColorMeta()).toBe('#0C0A09')
  })

  it('should leave the meta alone when the token does not resolve', () => {
    installBackgroundToken('')
    applyTheme('dark')
    expect(themeColorMeta()).toBe('#fafafa')
  })

  it('should create the meta when the document has none', () => {
    document.head.innerHTML = ''
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(document.head.querySelectorAll('meta[name="theme-color"]')).toHaveLength(1)
    expect(themeColorMeta()).toBe('#0C0A09')
  })
})

describe('setTheme', () => {
  beforeEach(() => {
    document.head.innerHTML = '<meta name="theme-color" content="#fafafa">'
    document.documentElement.classList.remove('dark')
    useThemeStore.setState({ theme: 'light' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should apply the class and the theme colour when the class is stale', () => {
    installBackgroundToken('#0C0A09')
    useThemeStore.getState().setTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(themeColorMeta()).toBe('#0C0A09')
  })

  it('should refresh the theme colour when a view transition already toggled the class', () => {
    installBackgroundToken('#0C0A09')
    document.documentElement.classList.add('dark')
    useThemeStore.getState().setTheme('dark')
    expect(themeColorMeta()).toBe('#0C0A09')
  })
})
```

The import line reads `import { applyTheme, useThemeStore } from './theme.store'`. The last case exists because `ThemeToggle.tsx` flips the `dark` class itself inside `document.startViewTransition` before calling `setTheme`, and `setTheme` skips `applyTheme` when the class is already right; without a sync on that path the system bar keeps the old colour.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/lib/theme.store.test.ts`
Expected: the theme-colour cases fail (`expected '#fafafa' to be '#0C0A09'`), the class-toggle cases pass.

- [ ] **Step 3: Sync the meta from the token**

`apps/web/src/lib/theme.store.ts`: insert above `applyTheme` and extend it:

```ts
/**
 * Copies the resolved `--background` into the theme-color meta, so the system bar of an
 * installed app follows the app's own theme instead of the OS preference. The token is read
 * from CSS so the value lives in theme.css only.
 *
 * This function owns the meta outright, creating it when it is absent: React must never render
 * it. React 19 hydrates head metas as hoistables keyed by their content, so a meta the pre-paint
 * script has already recoloured no longer matches what React expects and React appends a second,
 * stale one — leaving the page with two theme-color metas in dark mode.
 */
function syncThemeColor() {
  const background = getComputedStyle(document.documentElement).getPropertyValue('--background').trim()
  if (!background) return
  let meta = document.querySelector('meta[name="theme-color"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.setAttribute('name', 'theme-color')
    document.head.appendChild(meta)
  }
  meta.setAttribute('content', background)
}

/** Applies the correct class to <html> and the matching theme colour. Safe to call outside React. No-op on SSR. */
export function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return
  const resolved = resolveTheme(theme)
  document.documentElement.classList.toggle('dark', resolved === 'dark')
  syncThemeColor()
}
```

In `setTheme` inside the store, the guarded call becomes:

```ts
        if ((resolved === 'dark') !== isDark) {
          applyTheme(theme)
          return
        }
        // The view transition toggles only the class, so the theme colour still needs refreshing.
        syncThemeColor()
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/lib/theme.store.test.ts`
Expected: 6 tests pass.

- [ ] **Step 5: Pre-paint script does the same**

`apps/web/src/routes/__root.tsx`, `themeScript` (lines 15-25). The stylesheet link precedes the script in the document, and a pending stylesheet blocks script execution, so the token is readable here:

```ts
// Inline script that runs before first paint to avoid theme flash.
// Reads the persisted Zustand value from localStorage directly, then writes the resolved
// background into the theme-color meta so the system bar is right from the first frame. The
// script creates that meta itself because React must not render one: React 19 hydrates head
// metas as hoistables keyed by their content, so a meta this script has recoloured no longer
// matches and React appends a second, stale one.
const themeScript = `
(function(){
  try {
    var stored = JSON.parse(localStorage.getItem('theme') || '{}');
    var theme = stored.state?.theme || 'system';
    var dark = theme === 'dark' ||
      (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    if (dark) document.documentElement.classList.add('dark');
    var background = getComputedStyle(document.documentElement).getPropertyValue('--background').trim();
    if (background) {
      var meta = document.querySelector('meta[name="theme-color"]');
      if (!meta) {
        meta = document.createElement('meta');
        meta.setAttribute('name', 'theme-color');
        document.head.appendChild(meta);
      }
      meta.setAttribute('content', background);
    }
  } catch(e) {}
})();
`
```

- [ ] **Step 6: Check and test**

Run: `bun run check` then `bun run test`
Expected: check clean, 275 tests pass.

- [ ] **Step 7: See it**

Run: `bun run dev`, open `http://localhost:7070/` in the browser, switch the app theme to dark in the UI, run `document.querySelector('meta[name="theme-color"]').content` in the console: `#0C0A09`. Reload: still `#0C0A09` before any React effect (check the value in the console immediately after load).

- [ ] **Step 8: Ready to commit**

Proposed message: `update theme store to mirror the background token into theme-color for installed apps`

---

### Task 7: Icons and manifest

**Files:**
- Modify: `apps/web/package.json` (devDependency `sharp`, script `pwa-icons`)
- Create: `apps/web/scripts/app-icon.svg`
- Create: `apps/web/scripts/pwa-icons.mjs`
- Generate: `apps/web/public/icon-maskable-512.png`, `apps/web/public/apple-touch-icon.png`
- Create: `apps/web/public/README.md`
- Modify: `apps/web/public/manifest.json`
- Test: `apps/web/src/pwa-manifest.test.ts`

**Interfaces:**
- Consumes: `LIGHT_THEME_COLOR` from `#/lib/document-head` (Task 5).

- [ ] **Step 1: Write the failing test**

`apps/web/src/pwa-manifest.test.ts` (Node environment: it reads files from disk, see Task 4 Step 1):

```ts
// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { LIGHT_THEME_COLOR } from './lib/document-head'

interface ManifestIcon {
  src: string
  sizes: string
  type: string
  purpose?: string
}

const publicDir = new URL('../public/', import.meta.url)
const manifest = JSON.parse(readFileSync(new URL('manifest.json', publicDir), 'utf8'))
const pngIcons: ManifestIcon[] = manifest.icons.filter((icon: ManifestIcon) => icon.type === 'image/png')

/** Reads width and height from a PNG's IHDR chunk, which starts at byte 16. */
function pngSize(src: string): string {
  const bytes = readFileSync(new URL(src.replace(/^\//, ''), publicDir))
  return `${bytes.readUInt32BE(16)}x${bytes.readUInt32BE(20)}`
}

describe('manifest.json', () => {
  it('should open on the rooms list like the Android app', () => {
    expect(manifest.start_url).toBe('/dashboard')
    expect(manifest.scope).toBe('/')
    expect(manifest.display).toBe('standalone')
  })

  it('should paint the splash and system bar in the light surface colour', () => {
    expect(manifest.theme_color).toBe(LIGHT_THEME_COLOR)
    expect(manifest.background_color).toBe(LIGHT_THEME_COLOR)
  })

  it('should ship a maskable 512px icon for Android home screens', () => {
    const maskable = pngIcons.find((icon) => icon.purpose === 'maskable')
    expect(maskable).toEqual({ src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' })
  })

  it('should declare every PNG icon at its real size', () => {
    for (const icon of pngIcons) {
      expect(pngSize(icon.src), icon.src).toBe(icon.sizes)
    }
  })
})

describe('apple-touch-icon.png', () => {
  it('should be 180px square', () => {
    expect(pngSize('/apple-touch-icon.png')).toBe('180x180')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/pwa-manifest.test.ts`
Expected: 4 failures (`start_url` is `.`, colours are `#E11D48`/`#ffffff`, no maskable icon, `ENOENT` for apple-touch-icon), 1 pass (existing PNG sizes).

- [ ] **Step 3: Add sharp and the script entry**

Run: `bun add -d sharp`
Expected: `sharp` under `devDependencies` in `apps/web/package.json`, `bun.lock` updated (a generated file; commit it, never edit it).

Add to `"scripts"` in `apps/web/package.json`, after `"format"`:

```json
    "pwa-icons": "node scripts/pwa-icons.mjs",
```

- [ ] **Step 4: The icon source**

`apps/web/scripts/app-icon.svg`. The mark is the same Lucide `Radio` glyph as `favicon.svg`, white on the brand rose so it survives Android's launcher masks; the group is scaled to 60 % of the canvas, inside the 80 % safe zone. Hex values are allowed here because the file is an asset, like `favicon.svg`.

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">
  <rect width="512" height="512" fill="#E11D48"/>
  <g transform="translate(102.4 102.4) scale(12.8)" fill="none" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M16.247 7.761a6 6 0 0 1 0 8.478"/>
    <path d="M19.075 4.933a10 10 0 0 1 0 14.134"/>
    <path d="M4.925 19.067a10 10 0 0 1 0-14.134"/>
    <path d="M7.753 16.239a6 6 0 0 1 0-8.478"/>
    <circle cx="12" cy="12" r="2" fill="#FFFFFF" stroke="none"/>
  </g>
</svg>
```

- [ ] **Step 5: The generator**

`apps/web/scripts/pwa-icons.mjs`:

```js
#!/usr/bin/env node
/**
 * Renders the PWA icons from scripts/app-icon.svg into public/. Run `bun run pwa-icons`
 * after editing the SVG; the PNGs are generated files and are never edited by hand.
 */
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import sharp from 'sharp'

const scriptsDirectory = path.dirname(fileURLToPath(import.meta.url))
const sourceSvg = path.join(scriptsDirectory, 'app-icon.svg')
const publicDirectory = path.join(scriptsDirectory, '..', 'public')

// Android masks the 512px icon with the launcher's shape; iOS rounds the 180px one itself.
const outputs = [
  { fileName: 'icon-maskable-512.png', sizePx: 512 },
  { fileName: 'apple-touch-icon.png', sizePx: 180 },
]

for (const { fileName, sizePx } of outputs) {
  await sharp(sourceSvg).resize(sizePx, sizePx).png().toFile(path.join(publicDirectory, fileName))
  console.log(`wrote public/${fileName} (${sizePx}x${sizePx})`)
}
```

Run: `bun run pwa-icons`
Expected:

```
wrote public/icon-maskable-512.png (512x512)
wrote public/apple-touch-icon.png (180x180)
```

Open both PNGs (the Read tool renders images): a rose square with a white Radio mark filling the middle 60 %.

- [ ] **Step 6: Record how to regenerate**

`apps/web/public/README.md`:

```markdown
# Static assets

Generated files in this directory are never edited by hand.

| File | Source | Regenerate |
|---|---|---|
| `icon-maskable-512.png` | `../scripts/app-icon.svg` | `bun run pwa-icons` |
| `apple-touch-icon.png` | `../scripts/app-icon.svg` | `bun run pwa-icons` |

`manifest.json`, `document-head.ts` and `theme.css` must agree on the light surface colour;
`src/pwa-manifest.test.ts` and `src/lib/document-head.test.ts` fail when they drift.
```

- [ ] **Step 7: The manifest**

Replace `apps/web/public/manifest.json`:

```json
{
  "id": "/",
  "short_name": "Bedrud",
  "name": "Bedrud",
  "description": "Private video meetings",
  "icons": [
    {
      "src": "/favicon-16x16.png",
      "type": "image/png",
      "sizes": "16x16"
    },
    {
      "src": "/favicon-32x32.png",
      "type": "image/png",
      "sizes": "32x32"
    },
    {
      "src": "/favicon.svg",
      "type": "image/svg+xml",
      "sizes": "any",
      "purpose": "any"
    },
    {
      "src": "/favicon.ico",
      "sizes": "48x48",
      "type": "image/x-icon"
    },
    {
      "src": "/logo192.png",
      "type": "image/png",
      "sizes": "192x192"
    },
    {
      "src": "/logo512.png",
      "type": "image/png",
      "sizes": "512x512"
    },
    {
      "src": "/icon-maskable-512.png",
      "type": "image/png",
      "sizes": "512x512",
      "purpose": "maskable"
    }
  ],
  "screenshots": [
    {
      "src": "/screenshot-wide.png",
      "sizes": "1920x1080",
      "form_factor": "wide",
      "type": "image/png",
      "label": "Bedrud dashboard"
    },
    {
      "src": "/screenshot-narrow.png",
      "sizes": "2048x2732",
      "type": "image/png",
      "label": "Bedrud mobile view"
    }
  ],
  "start_url": "/dashboard",
  "scope": "/",
  "display": "standalone",
  "theme_color": "#fafafa",
  "background_color": "#fafafa"
}
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `bunx vitest run src/pwa-manifest.test.ts`
Expected: 5 tests pass.

- [ ] **Step 9: Check and test**

Run: `bun run check` then `bun run test`
Expected: check clean, 280 tests pass.

- [ ] **Step 10: Ready to commit**

Proposed message: `add maskable and Apple icons with manifest fields for installability`

---

### Task 8: Docs

**Files:**
- Modify: `DESIGN.md:3-5,102-112,149,177-180,205`
- Modify: `apps/web/AGENTS.md` (Buttons section note; new Mobile detection section)
- Modify: `docs/plan/pwa-parity/01-overview-and-units.md` (unit 1 status)

- [ ] **Step 1: Root DESIGN.md**

Lines 3-5:

```markdown
## Aesthetic — Rose + Teal (Rounded)

Rounded corners on the Android client's scale. Bold colors. No purple.
```

Replace the whole `## Border Radius` section (lines 102-112):

```markdown
## Border Radius

The web shares the Android client's shape scale (`apps/android`, `Shape.kt`), applied through
the Tailwind radius tokens in `styles.css`. Use the class, never an arbitrary `rounded-[Npx]`.

| Web class | px | Android token | Used for |
|---|---|---|---|
| `rounded` | 4 | `xs` | Tailwind default |
| `rounded-sm` | 8 | `sm` | chips |
| `rounded-md` | 12 | `md` | buttons |
| `rounded-lg` | 12 | `md` | fields |
| `rounded-xl` | 16 | `lg` | cards |
| `rounded-2xl` | 20 | `xl` | |
| `rounded-3xl` | 28 | `xxl` | sheet top, video tile, controls bar |
| `rounded-full` | pill | `full` | avatars, pills, FAB |

`--radius` in `theme.css` is the default corner (12px) that self-hosters may retune; the scale
itself lives in `styles.css`. `src/design-tokens.test.ts` pins both.
```

Line 149:

```markdown
- **Radius**: the scale above; `--radius` 12px is the default corner
```

Lines 177-180, the Inputs pattern:

```markdown
### Inputs
- Bare: `border` only, no background fill
- Focus: `ring-2 ring-ring`
- Corner: `rounded-lg` (12px, the field size)
```

Line 205, the first "What NOT to Do" bullet:

```markdown
- Do NOT write `rounded-[Npx]` — pick the class from the Border Radius table.
```

- [ ] **Step 2: apps/web/AGENTS.md**

Add after the `## Buttons` code block's closing rules (after "**No `active:scale-95`** — it feels cheap."):

```markdown
Corners come from the shared scale (`rounded-md` 12px for buttons, `rounded-lg` 12px for fields, `rounded-xl` 16px for cards, `rounded-3xl` 28px for sheets). The table with the Android token names is in the root `DESIGN.md`. Never `rounded-[Npx]`.
```

Add a new section before `## Do / Don't`:

```markdown
## Mobile detection

One breakpoint: `MOBILE_BREAKPOINT_PX` (1024, Tailwind `lg`) in `src/lib/use-is-mobile.ts`. Render-time branches use `useIsMobile()`; effects and event handlers use `isMobileViewport()`. CSS uses `lg:` / `max-lg:` for the same line. No component defines its own `matchMedia` width check.
```

- [ ] **Step 3: Unit status**

In `docs/plan/pwa-parity/01-overview-and-units.md`, the Units table row for unit 1: status `in progress, [02](./02-foundation.md), [03](./03-foundation-plan.md)`.

- [ ] **Step 4: Ready to commit**

Proposed message: `update design docs for the rounded scale and one breakpoint`

---

### Task 9: Verify on a device

No files. Evidence for the pull request; every line is observed, not assumed.

- [ ] **Step 1: Phone-width screenshots, before and after**

Base branch first: from a checkout of `origin/master` run `bun run dev`, capture `/auth/login`, `/dashboard` (signed in), `/dashboard/settings` and `/m/<room>` at 390 × 844, light and dark, with the Playwright MCP. Then the same on this branch. Keep the files for the `pr-evidence` skill.

- [ ] **Step 2: Install on the Android emulator**

Emulator Chrome at `http://10.0.2.2:7070` (dev server bound to all interfaces if needed: `BEDRUD_DEV_BIND_HOST=0.0.0.0 bun run dev`). Chrome's menu offers *Add to Home screen* / *Install*. Install. Home-screen icon: rose square with the white Radio mark, not a shrunken disc.

- [ ] **Step 3: Standalone launch**

Tap the icon. No browser chrome. Signed out lands on `/auth`; signed in lands on `/dashboard`. Status bar is `#fafafa` in the light theme. Switch the app to dark in settings without changing the OS setting: the status bar follows to `#0C0A09`. Relaunch: still dark from the first frame.

- [ ] **Step 4: Corners**

Dashboard cards, buttons, inputs, the create-room dialog and the bottom nav show the scale corners in both themes. Nothing is square except `rounded-none` elements.

- [ ] **Step 5: Meeting at tablet width**

Browser window at 900 px wide on `/m/<room>`: phone chrome (bottom controls sized for touch, chat opens as a full-screen modal, no desktop sidebar). At 1100 px: desktop chrome. The switch happens at 1024, not 640.

- [ ] **Step 6: Report**

Fill in the Definition of Done from the working agreement with a real pass or fail per line, then stop and wait for commit approval.
