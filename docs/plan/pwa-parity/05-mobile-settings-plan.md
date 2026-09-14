# Mobile settings page implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the phone settings drill-down into one scrolling page of sections, move the settings dialog onto the shared 1024px breakpoint, and send phones that land on the desktop settings route to the phone one.

**Architecture:** A new pure module owns the ordered section list and the path-to-section mapping. The `/settings` layout renders every panel inline, each behind a section header and anchored by its route segment, and scrolls to the section named by the current path. The five sub-routes keep their URLs and titles and render nothing of their own. No settings panel is modified, so the in-meeting dialog that reuses all six panels is untouched.

**Tech Stack:** React 19, TanStack Router file routes, TailwindCSS v4, Vitest 4 with jsdom, Bun scripts, Biome.

## Global Constraints

- The spec is `docs/plan/pwa-parity/04-mobile-settings.md`. Where this plan and the spec disagree, ask; do not choose.
- Corner classes come from the shape scale in `DESIGN.md`: `rounded-sm` 8px chips, `rounded-md`/`rounded-lg` 12px buttons and fields, `rounded-xl` 16px cards. One `rounded-*` per element, never an arbitrary `rounded-[Npx]`.
- The single phone breakpoint is `MOBILE_BREAKPOINT_PX = 1024` in `src/lib/use-is-mobile.ts`. CSS uses `lg:` and `max-lg:`; `sm:` and `max-sm:` are not phone checks and must not be introduced.
- The section header is exactly `text-xs font-semibold uppercase tracking-wide text-primary`.
- This repo has no React render tests. Every app-owned test is pure logic or reads a source file and asserts on its text. Follow that; do not introduce `@testing-library/react`.
- Biome owns final layout: import order and the wrapping of long `it.each(...)` calls are its business, not the code blocks' below. Write what the task gives you, then let the formatter settle it and commit what it produces. `bun run check` is the arbiter, and it must pass with only the three known `noDocumentCookie` warnings in `api.test.ts`.
- Test files sit beside their source, one per source file. Descriptions are imperative (`should …`). Every test sits inside a `describe()` naming the unit under test.
- No abbreviated identifiers. `index`, not `i`; `element`, not `el`.
- Comments sit above the code they describe, as complete present-tense sentences ending in a period. Never trailing.
- No magic values: a repeated literal gets a name in the file that owns it.
- Do not modify any file under `src/components/settings/` other than `BedrudSettingsDialog.tsx` and the new `settingsSections.ts` plus its test.
- Commit each task separately. No attribution of any kind in commit messages, code or docs. Do not push and do not open a pull request.

---

## File Structure

**Created**
- `apps/web/src/components/settings/settingsSections.ts` — the ordered section list and the path-to-section mapping. Pure data and one function; no React.
- `apps/web/src/components/settings/settingsSections.test.ts` — its test.
- `apps/web/src/components/settings/settingsBreakpoints.test.ts` — reads `BedrudSettingsDialog.tsx` and pins that it carries no `sm:` prefix.

**Modified**
- `apps/web/src/routes/settings.tsx` — becomes the one-page screen.
- `apps/web/src/routes/settings.appearance.tsx`, `.audio.tsx`, `.video.tsx`, `.security.tsx`, `.experimental.tsx` — keep their route and title, render nothing.
- `apps/web/src/components/settings/BedrudSettingsDialog.tsx` — six breakpoint branches.
- `apps/web/src/routes/dashboard/settings.tsx` — redirect phones.
- `docs/plan/pwa-parity/01-overview-and-units.md` — unit 5 status.
- `apps/web/AGENTS.md` — one line on the settings surface.

---

### Task 1: The section model

**Files:**
- Create: `apps/web/src/components/settings/settingsSections.ts`
- Test: `apps/web/src/components/settings/settingsSections.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `SettingsSectionId` (a union of the five ids), `SETTINGS_SECTIONS` (a readonly array of `{ id, label, route }` in display order), and `sectionIdFromPathname(pathname: string): SettingsSectionId | null`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/components/settings/settingsSections.test.ts`:

```ts
// The settings page renders one section per entry, in this order, and anchors each by its id.
// The ids double as the route segments under `/settings`, so the mapping below is what keeps a
// bookmarked sub-route pointing at the right section.

import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS, sectionIdFromPathname } from './settingsSections'

describe('SETTINGS_SECTIONS', () => {
  it('should list the five sections in display order', () => {
    expect(SETTINGS_SECTIONS.map((section) => section.id)).toEqual([
      'appearance',
      'audio',
      'video',
      'security',
      'experimental',
    ])
  })

  it('should give every section a route built from its id', () => {
    for (const section of SETTINGS_SECTIONS) {
      expect(section.route).toBe(`/settings/${section.id}`)
    }
  })

  it('should give every section a non-empty label', () => {
    for (const section of SETTINGS_SECTIONS) {
      expect(section.label.length).toBeGreaterThan(0)
    }
  })
})

describe('sectionIdFromPathname', () => {
  it.each(['appearance', 'audio', 'video', 'security', 'experimental'] as const)(
    'should read %s from its sub-route',
    (id) => {
      expect(sectionIdFromPathname(`/settings/${id}`)).toBe(id)
    },
  )

  it('should ignore a trailing slash', () => {
    expect(sectionIdFromPathname('/settings/audio/')).toBe('audio')
  })

  it('should return null on the settings index', () => {
    expect(sectionIdFromPathname('/settings')).toBeNull()
    expect(sectionIdFromPathname('/settings/')).toBeNull()
  })

  it('should return null for a segment that is not a section', () => {
    expect(sectionIdFromPathname('/settings/nonsense')).toBeNull()
  })

  it('should return null for a path outside settings', () => {
    expect(sectionIdFromPathname('/dashboard/settings/audio')).toBeNull()
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/components/settings/settingsSections.test.ts` from `apps/web`
Expected: FAIL, `Failed to resolve import "./settingsSections"`.

- [ ] **Step 3: Write the implementation**

Create `apps/web/src/components/settings/settingsSections.ts`:

```ts
export type SettingsSectionId = 'appearance' | 'audio' | 'video' | 'security' | 'experimental'

export interface SettingsSection {
  id: SettingsSectionId
  label: string
  route: `/settings/${SettingsSectionId}`
}

/**
 * The settings sections in the order the page renders them. Each id is also the section's route
 * segment and the `id` attribute its heading carries, so a link to `/settings/audio` and an anchor
 * to `/settings#audio` reach the same place.
 */
export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { id: 'appearance', label: 'Appearance', route: '/settings/appearance' },
  { id: 'audio', label: 'Audio', route: '/settings/audio' },
  { id: 'video', label: 'Video', route: '/settings/video' },
  { id: 'security', label: 'Security', route: '/settings/security' },
  { id: 'experimental', label: 'Experimental', route: '/settings/experimental' },
]

const SETTINGS_PATH_PREFIX = '/settings/'

/** Returns the section a settings sub-route points at, or null for the index and anything else. */
export function sectionIdFromPathname(pathname: string): SettingsSectionId | null {
  if (!pathname.startsWith(SETTINGS_PATH_PREFIX)) return null
  const segment = pathname.slice(SETTINGS_PATH_PREFIX.length).replace(/\/$/, '')
  const section = SETTINGS_SECTIONS.find((candidate) => candidate.id === segment)
  return section ? section.id : null
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/components/settings/settingsSections.test.ts`
Expected: PASS, 9 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/settings/settingsSections.ts apps/web/src/components/settings/settingsSections.test.ts
git commit -m "add settings section model for one page of anchored sections"
```

---

### Task 2: The one-page settings screen

**Files:**
- Modify: `apps/web/src/routes/settings.tsx` (replace the whole component)

**Interfaces:**
- Consumes: `SETTINGS_SECTIONS`, `sectionIdFromPathname` from Task 1; the five panels, unchanged.
- Produces: nothing importable. The five sub-routes rely on this layout rendering the page.

**Context:** the file today is a layout that renders either a push-list or `<Outlet />` behind a back link. Everything between `function SettingsLayout()` and the end of the file is replaced. The `Route` definition at the top keeps its `beforeLoad`, `loader`, `staleTime` and `head`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/routes/settings.route.test.ts`:

```ts
// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS } from '../components/settings/settingsSections'

const settingsRouteSource = readFileSync(new URL('./settings.tsx', import.meta.url), 'utf8')

describe('the settings route', () => {
  it('should build its sections from the shared model rather than its own list', () => {
    expect(settingsRouteSource).toContain('SETTINGS_SECTIONS')
  })

  it('should render every panel inline', () => {
    for (const section of SETTINGS_SECTIONS) {
      const panelName = `${section.label}SettingsPanel`
      expect(settingsRouteSource).toContain(`<${panelName} />`)
    }
  })

  it('should scroll to the section the current path names', () => {
    expect(settingsRouteSource).toContain('sectionIdFromPathname')
  })

  it('should no longer push to a sub-page', () => {
    expect(settingsRouteSource).not.toContain('ChevronRight')
    expect(settingsRouteSource).not.toContain('ChevronLeft')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/routes/settings.route.test.ts`
Expected: FAIL on "should render every panel inline" and "should no longer push to a sub-page".

- [ ] **Step 3: Write the implementation**

Replace the contents of `apps/web/src/routes/settings.tsx` with:

```tsx
import { createFileRoute, Outlet, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'
import { loadRegisteredUser, requireRegisteredUser } from '#/lib/require-registered-user'
import { MobileOnlyGate } from '@/components/dashboard/MobileOnlyGate'
import { AppearanceSettingsPanel } from '@/components/settings/AppearanceSettingsPanel'
import { AudioSettingsPanel } from '@/components/settings/AudioSettingsPanel'
import { ExperimentalSettingsPanel } from '@/components/settings/ExperimentalSettingsPanel'
import { SecuritySettingsPanel } from '@/components/settings/SecuritySettingsPanel'
import {
  SETTINGS_SECTIONS,
  type SettingsSectionId,
  sectionIdFromPathname,
} from '@/components/settings/settingsSections'
import { VideoSettingsPanel } from '@/components/settings/VideoSettingsPanel'

export const Route = createFileRoute('/settings')({
  beforeLoad: requireRegisteredUser,
  loader: loadRegisteredUser,
  staleTime: Infinity,
  head: () => ({ meta: [{ title: 'Settings — Bedrud' }] }),
  component: SettingsLayout,
})

/** Matches the Android section header: a small label in the primary colour above its card. */
const SECTION_HEADER_CLASS = 'text-xs font-semibold uppercase tracking-wide text-primary'

/** Renders the panel that belongs to a section. */
function SettingsSectionPanel({ id }: { id: SettingsSectionId }) {
  switch (id) {
    case 'appearance':
      return <AppearanceSettingsPanel />
    case 'audio':
      return <AudioSettingsPanel />
    case 'video':
      return <VideoSettingsPanel />
    case 'security':
      return <SecuritySettingsPanel />
    case 'experimental':
      return <ExperimentalSettingsPanel />
  }
}

/**
 * Scrolls the section named by the current path, or by the hash, into view. A sub-route such as
 * `/settings/audio` renders this same page, so the scroll is what makes a bookmark or a link land
 * on its section.
 *
 * This renders inside the gate rather than beside it. `MobileOnlyGate` returns null until its own
 * effect has confirmed the viewport, so an effect placed in the layout would run while the sections
 * are still absent from the document, find nothing, and never run again.
 */
function SettingsSectionScroll({ pathname, hash }: { pathname: string; hash: string }) {
  useEffect(() => {
    const sectionId = sectionIdFromPathname(pathname) ?? hash.replace(/^#/, '')
    if (!sectionId) return
    document.getElementById(sectionId)?.scrollIntoView({ block: 'start' })
  }, [pathname, hash])

  return null
}

function SettingsLayout() {
  const { location } = useRouterState()

  return (
    <MobileOnlyGate desktopTo="/dashboard/settings">
      <SettingsSectionScroll pathname={location.pathname} hash={location.hash} />

      {/* Sections sit 24px apart, matching the gap the panels already use between their own cards. */}
      <div className="space-y-6">
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>

        {SETTINGS_SECTIONS.map(({ id, label }) => (
          <section key={id} aria-labelledby={id} className="space-y-2 scroll-mt-4">
            <h2 id={id} className={SECTION_HEADER_CLASS}>
              {label}
            </h2>
            <SettingsSectionPanel id={id} />
          </section>
        ))}
      </div>

      {/* Renders the matched sub-route so its document title applies; the sub-routes draw nothing. */}
      <Outlet />
    </MobileOnlyGate>
  )
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/routes/settings.route.test.ts`
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/routes/settings.tsx apps/web/src/routes/settings.route.test.ts
git commit -m "render every settings panel on one scrolling page on phones"
```

---

### Task 3: The sub-routes keep their URLs and titles

**Files:**
- Modify: `apps/web/src/routes/settings.appearance.tsx`, `settings.audio.tsx`, `settings.video.tsx`, `settings.security.tsx`, `settings.experimental.tsx`

**Interfaces:**
- Consumes: the layout from Task 2, which renders the page and performs the scroll.
- Produces: nothing.

**Context:** each file today renders an `<h1>` and one panel. The layout now renders every panel, so a sub-route that still rendered its panel would render it twice. Each becomes a route that carries only its title.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/routes/settings.sections.test.ts`:

```ts
// @vitest-environment node
//
// This file reads route modules from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS } from '../components/settings/settingsSections'

/** Reads one settings sub-route module. */
function sectionRouteSource(id: string): string {
  return readFileSync(new URL(`./settings.${id}.tsx`, import.meta.url), 'utf8')
}

describe('the settings sub-routes', () => {
  it.each(SETTINGS_SECTIONS.map((section) => [section.id, section.label]))(
    'should keep the %s title',
    (id, label) => {
      expect(sectionRouteSource(id)).toContain(`title: '${label} — Bedrud'`)
    },
  )

  it.each(SETTINGS_SECTIONS.map((section) => [section.id, section.label]))(
    'should not render the %s panel a second time',
    (id, label) => {
      expect(sectionRouteSource(id)).not.toContain(`<${label}SettingsPanel`)
    },
  )
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/routes/settings.sections.test.ts`
Expected: FAIL on all five "should not render the … panel a second time".

- [ ] **Step 3: Write the implementation**

Replace each of the five files. For `settings.appearance.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/appearance')({
  head: () => ({ meta: [{ title: 'Appearance — Bedrud' }] }),
  component: AppearanceSettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function AppearanceSettingsPage() {
  return null
}
```

The other four files take exactly the same shape, including the comment. Only three values differ,
and nothing else in the file changes:

| file | `createFileRoute` argument | title | component name |
|---|---|---|---|
| `settings.audio.tsx` | `/settings/audio` | `'Audio — Bedrud'` | `AudioSettingsPage` |
| `settings.video.tsx` | `/settings/video` | `'Video — Bedrud'` | `VideoSettingsPage` |
| `settings.security.tsx` | `/settings/security` | `'Security — Bedrud'` | `SecuritySettingsPage` |
| `settings.experimental.tsx` | `/settings/experimental` | `'Experimental — Bedrud'` | `ExperimentalSettingsPage` |

Each keeps its single `import { createFileRoute } from '@tanstack/react-router'` and drops the panel
import, which is now unused.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/routes/settings.sections.test.ts`
Expected: PASS, 10 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/routes/settings.appearance.tsx apps/web/src/routes/settings.audio.tsx apps/web/src/routes/settings.video.tsx apps/web/src/routes/settings.security.tsx apps/web/src/routes/settings.experimental.tsx apps/web/src/routes/settings.sections.test.ts
git commit -m "keep settings sub-routes as titled anchors into the one page"
```

---

### Task 4: The settings dialog moves to the shared breakpoint

**Files:**
- Modify: `apps/web/src/components/settings/BedrudSettingsDialog.tsx` lines 312, 321, 324, 325, 327, 328, 331, 333
- Test: `apps/web/src/components/settings/settingsBreakpoints.test.ts`

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing.

**Context:** the dialog is the other settings shell, used in a meeting and from the home page. It branches at Tailwind's `sm` (640px) while every other phone check in the app uses 1024px. This task changes prefixes only: every `sm:` becomes `lg:` and every `max-sm:` becomes `max-lg:`. No class other than the prefix changes, and no behaviour changes.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/components/settings/settingsBreakpoints.test.ts`:

```ts
// @vitest-environment node
//
// This file reads a component from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dialogSource = readFileSync(new URL('./BedrudSettingsDialog.tsx', import.meta.url), 'utf8')

describe('the settings dialog breakpoint', () => {
  it('should not switch layout at the sm breakpoint', () => {
    expect(dialogSource).not.toMatch(/\bmax-sm:/)
    expect(dialogSource).not.toMatch(/(?<![\w-])sm:/)
  })

  it('should switch layout at the shared phone breakpoint', () => {
    expect(dialogSource).toMatch(/(?<![\w-])lg:hidden/)
    expect(dialogSource).toMatch(/\bmax-lg:/)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/components/settings/settingsBreakpoints.test.ts`
Expected: FAIL on "should not switch layout at the sm breakpoint".

- [ ] **Step 3: Write the implementation**

In `BedrudSettingsDialog.tsx`, rewrite every breakpoint prefix. The eight sites and their new text:

- `:312` `max-sm:h-[…]` → `max-lg:h-[…]`
- `:321` `sm:h-[min(90vh,720px)] sm:w-[…] sm:max-w-[…]` → the same three with `lg:`
- `:324` the long `max-sm:fixed max-sm:left-… max-sm:border-b` run → each `max-sm:` becomes `max-lg:`
- `:325` `max-sm:border-[var(--meet-border)]` / `max-sm:border-border` → `max-lg:`
- `:327` the second long `max-sm:` run → each becomes `max-lg:`
- `:328` `max-sm:[&>button.absolute]:hidden` → `max-lg:[&>button.absolute]:hidden`
- `:331` `sm:hidden` → `lg:hidden`
- `:333` `hidden … sm:flex` → `hidden … lg:flex`

Change nothing else in the file.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/components/settings/settingsBreakpoints.test.ts`
Expected: PASS, 2 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/settings/BedrudSettingsDialog.tsx apps/web/src/components/settings/settingsBreakpoints.test.ts
git commit -m "switch the settings dialog at the shared phone breakpoint"
```

---

### Task 5: The desktop settings route redirects phones

**Files:**
- Modify: `apps/web/src/routes/dashboard/settings.tsx`

**Interfaces:**
- Consumes: `isMobileViewport` from `#/lib/use-is-mobile`.
- Produces: nothing.

**Context:** `/settings` already sends desktop visitors to `/dashboard/settings` through `MobileOnlyGate`. Nothing sends phones the other way, so a phone that lands on `/dashboard/settings` gets the desktop pill tabs. This adds the mirror image, using the same effect-based approach and the same reasoning: the viewport is read inside the effect because the hook's server snapshot is "desktop", and an effect keyed on it would bounce desktop visitors on hydration.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/routes/dashboard/settings.route.test.ts`:

```ts
// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const desktopSettingsSource = readFileSync(new URL('./settings.tsx', import.meta.url), 'utf8')

describe('the desktop settings route', () => {
  it('should send phone visitors to the phone settings page', () => {
    expect(desktopSettingsSource).toContain('isMobileViewport')
    expect(desktopSettingsSource).toContain("to: '/settings'")
  })

  it('should replace the history entry rather than stacking one', () => {
    expect(desktopSettingsSource).toContain('replace: true')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bunx vitest run src/routes/dashboard/settings.route.test.ts`
Expected: FAIL on both.

- [ ] **Step 3: Write the implementation**

In `apps/web/src/routes/dashboard/settings.tsx`, extend the imports:

```tsx
import { createFileRoute, Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { Camera, Lock, Mic, User } from 'lucide-react'
import { useEffect } from 'react'
import { isMobileViewport } from '#/lib/use-is-mobile'
import { cn } from '@/lib/utils'
```

Then add this above `SettingsLayout`, below the `TABS` constant:

```tsx
/**
 * Sends a phone visitor to the phone settings page, mirroring the redirect `MobileOnlyGate`
 * already performs in the other direction. The viewport is read inside the effect rather than
 * through the hook, because the hook's server snapshot is "desktop" and an effect keyed on it
 * would bounce desktop visitors on hydration.
 */
function useRedirectPhonesToPhoneSettings() {
  const navigate = useNavigate()

  useEffect(() => {
    if (isMobileViewport()) navigate({ to: '/settings', replace: true })
  }, [navigate])
}
```

and call it as the first line of `SettingsLayout`:

```tsx
function SettingsLayout() {
  useRedirectPhonesToPhoneSettings()
  const { location } = useRouterState()
  const path = location.pathname
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bunx vitest run src/routes/dashboard/settings.route.test.ts`
Expected: PASS, 2 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/routes/dashboard/settings.tsx apps/web/src/routes/dashboard/settings.route.test.ts
git commit -m "send phones from the desktop settings route to the phone page"
```

---

### Task 6: Docs

**Files:**
- Modify: `docs/plan/pwa-parity/01-overview-and-units.md`, `apps/web/AGENTS.md`

**Interfaces:** none.

- [ ] **Step 1: Update the unit table**

In `docs/plan/pwa-parity/01-overview-and-units.md`, change unit 5's status cell from `in progress, [04](./04-mobile-settings.md)` to `done, [04](./04-mobile-settings.md), [05](./05-mobile-settings-plan.md)`.

- [ ] **Step 2: Record the settings surface in AGENTS.md**

In `apps/web/AGENTS.md`, add this paragraph to the section that describes layout and breakpoints:

```markdown
Phone settings live on one scrolling page at `/settings`, one section per panel, each anchored by
its route segment so `/settings/audio` and `/settings#audio` reach the same section. The sub-routes
carry only their document title. Desktop settings stay at `/dashboard/settings`, and each route
redirects the other's visitors. Section headers are `text-xs font-semibold uppercase tracking-wide
text-primary`, the web reading of Android's `labelLarge` in the primary colour.
```

- [ ] **Step 3: Verify**

Run from `apps/web`: `bun run check` then `bun run test`
Expected: 3 pre-existing `noDocumentCookie` warnings in `api.test.ts` and nothing else; every test file passing.

- [ ] **Step 4: Commit**

```bash
git add docs/plan/pwa-parity/01-overview-and-units.md apps/web/AGENTS.md
git commit -m "document the one-page phone settings surface"
```

---

## Verification, after every task is complete

Run from `apps/web` with the dev server on port 7070 and the API on 7071:

1. At 390 × 844 in both themes, open `/settings`. Every section header is visible in order, the page scrolls as one column, and the last section clears the bottom navigation and the safe area.
2. Open `/settings/audio` directly. The page lands on the Audio section and the tab title reads "Audio — Bedrud".
3. Open `/settings#video`. The page lands on the Video section.
4. At 800px wide, open the settings dialog from the home page. It uses the phone layout, not the desktop sidebar.
5. At 390px, open `/dashboard/settings`. It redirects to `/settings`.
6. At 1280px, open `/settings`. It redirects to `/dashboard/settings`, unchanged from today.
7. Capture before and after screenshots at 390 × 844 in both themes for the pull request.

Checks 2 and 3 are the ones that matter most. The ordering bug this unit hit — the scroll must run
inside `MobileOnlyGate`, because the gate renders nothing until its own effect confirms the viewport
— is now pinned by `settings.route.test.ts`, which asserts the placement, the effect ordering and the
dependency array. Those assertions catch the structure regressing; checks 2 and 3 remain the only
thing that observes an actual scroll, so they are what confirms the structure still produces one.
