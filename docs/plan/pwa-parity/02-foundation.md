# 02 — Foundation

Unit 1 of [PWA parity](./01-overview-and-units.md). Four parts: the shape scale, one breakpoint, the PWA head and manifest, and the docs and tests that pin them. No screen is redesigned here; this unit makes the later units possible and makes the web app a well-formed PWA.

## Shape scale

### Today

`apps/web/src/styles.css` sets `--radius-sm/md/lg/xl` to 4px and, in `@layer base`, applies `border-radius: var(--radius, 4px) !important` to every element. Components already carry `rounded-sm`, `rounded-md`, `rounded-lg`, `rounded-xl` and `rounded-full` (about 130 uses), but the override flattens all of them. The root `DESIGN.md` still describes a 0px system that the code left behind.

### Target

The Android scale from `apps/android` `Shape.kt`, expressed through the Tailwind radius tokens. Web class, pixel value, and the Android token it equals:

| Web class | px | Android token | Used for |
|---|---|---|---|
| `rounded` | 4 | `xs` | Tailwind default, untouched |
| `rounded-sm` | 8 | `sm` | chips |
| `rounded-md` | 12 | `md` | buttons |
| `rounded-lg` | 12 | `md` | fields |
| `rounded-xl` | 16 | `lg` | cards |
| `rounded-2xl` | 20 | `xl` | |
| `rounded-3xl` | 28 | `xxl` | sheet top, video tile, controls bar |
| `rounded-full` | pill | `full` | avatars, pills, FAB |

`rounded-md` and `rounded-lg` both resolve to 12px on purpose: the web codebase uses `rounded-md` for buttons and `rounded-lg` for inputs, and Android gives buttons and fields the same `md` corner.

### Changes

1. In `styles.css`, set `--radius-sm: 8px`, `--radius-md: var(--radius)`, `--radius-lg: var(--radius)`, `--radius-xl: 16px`, `--radius-2xl: 20px`, `--radius-3xl: 28px` inside the `@theme inline` block.
2. In `theme.css`, set `--radius: 12px`. It is the default corner (button and field) and the single knob a self-hoster edits; because `--radius-md` and `--radius-lg` resolve to it, retuning it actually moves those corners instead of leaving a dead token.
3. Delete the universal `border-radius … !important` declaration. The `.meet-*` exceptions that follow it keep their token-driven values and keep `!important`: they override `rounded-*` utilities on the same elements, and a layered rule without `!important` would lose to those utilities.
4. Replace the arbitrary `rounded-[7px]` (7 uses), `rounded-[10px]` (4 uses) and `rounded-[14px]` (1 use, the chat toast) in meeting components with `rounded-sm`, `rounded-md` and `rounded-xl`. They were invisible under the override and would otherwise start rendering as unnamed values.
5. Leave `rounded-none` (8 uses) alone; a deliberately square element stays square.
6. Restore the corner class on the shadcn primitives under `components/ui/`, which lost every `rounded-*` class when the repo was created: button `rounded-md`, input and select trigger `rounded-lg`, card `rounded-xl`, alert `rounded-lg`, dialog `rounded-3xl`, badge `rounded-sm`, tabs list `rounded-lg` with `rounded-md` triggers, menus and popovers `rounded-md` with `rounded-sm` items, tooltip and checkbox `rounded`. Alert is the inline banner, so it takes the banner corner rather than the card's; badge is the chip, so it takes `rounded-sm` and stops being a pill. Each primitive carries exactly one `rounded-*`, because `cn` is `twMerge(clsx(…))` and a second one deletes the first. `design-tokens.test.ts` pins each one.
7. Give every raw bordered box its own corner class, since removing the universal override left them square: a card or a selectable option gets `rounded-xl`, an inline banner, row container, segmented-control shell or compound input gets `rounded-lg`. Applies to `routes/settings.tsx`, `routes/index.tsx`, `routes/auth.register.tsx`, `routes/auth.reset-password.tsx`, `routes/auth.forgot-password.tsx`, `routes/dashboard/settings.tsx`, the five admin routes under `routes/dashboard/admin/`, `components/dashboard/CreateRoomDialog.tsx`, `components/dashboard/RoomSettingsDialog.tsx`, `components/auth/PasskeyButton.tsx`, `components/settings/SecuritySettingsPanel.tsx`, `components/settings/BedrudSettingsDialog.tsx`, `components/ErrorPage.tsx`, the fifteen admin components (`RoomEventsTable.tsx`, `RecentSignupsTable.tsx`, `DataTableToolbar.tsx`, `DataTableBulkBar.tsx`, `DataTablePagination.tsx`, `DataTableSearch.tsx`, `DataTableFacetedFilter.tsx`, `AdminControlBar.tsx`, `overview/index.tsx`, `queue-stats.tsx`, and `general-tab.tsx`, `invite-tokens-section.tsx`, `server-tab.tsx`, `shared.tsx` and `webxdc-tab.tsx` under `admin/settings/`), plus `components/meeting/webxdc/WebxdcFrame.tsx` and `components/meeting/chat/ChatInput.tsx`. Boxes built on a shadcn primitive already carry a corner and are left alone, as are the `.meet-*` boxes whose radius comes from a `meeting.css` token.
8. Give the filled boxes a corner too — the deleted override flattened those as well, and they sit beside the bordered ones on the same screens. A segmented shell gets `rounded-lg` and its items `rounded-md` (`routes/auth.tsx`, `routes/dashboard/settings.tsx`), a sidebar nav row `rounded-md` (`routes/dashboard.tsx`), a 28–32px icon chip or an icon button's hover fill `rounded-sm` (`routes/settings.tsx`, `components/settings/BedrudSettingsDialog.tsx`, `components/admin/overview/recent-events.tsx`, `components/admin/RoomEventsTable.tsx`, `components/admin/settings/general-tab.tsx`, `components/admin/settings/invite-tokens-section.tsx`, `routes/dashboard/archived_.$roomId.tsx`, `routes/dashboard/admin/rooms_.$roomId.tsx`), and a padded `bg-muted` block `rounded-lg` (`routes/dashboard/admin/users_.$userId.tsx`). A full-bleed row or strip inside a container that already clips it keeps no corner of its own.

The meeting's own radius tokens in `meeting.css` (`--meet-controls-bar-radius`, `--meet-btn-leave-radius`, `--meet-gallery-icon-radius`) keep their current values. Units 3 and 4 restyle the meeting chrome and set them to the Android `xxl` corner then.

## One breakpoint

### Today

Six copies of a width check, at three different widths:

| File | Width |
|---|---|
| `components/dashboard/MobileOnlyGate.tsx` | 1024 |
| `components/dashboard/CreateRoomDialog.tsx` | 768 |
| `components/meeting/ChatPanel.tsx` | 640 |
| `components/meeting/ControlsBar.tsx` | 640 |
| `components/meeting/MeetingRoomShell.tsx` | 640 |
| `components/meeting/ParticipantVideoSidebar.tsx` | 640 |

Each initialises state from `window.matchMedia` when `window` exists, which is a hydration mismatch waiting to happen under TanStack Start's server render.

### Target

One module, `apps/web/src/lib/use-is-mobile.ts`:

- `MOBILE_BREAKPOINT_PX = 1024` and `MOBILE_MEDIA_QUERY = '(max-width: 1023px)'`, exported so CSS and JS quote the same number.
- `useIsMobile(): boolean`, built on `useSyncExternalStore` with `matchMedia` as the store and `false` as the server snapshot. First paint on the server is the desktop tree; the `lg:` classes already hide desktop chrome on narrow screens, so nothing flashes.

All six call sites import it. The meeting components that pair their JS check with `sm:` classes (`MeetingPanels.tsx`, `ParticipantsList.tsx`, `ChatPanel.tsx`, `ControlsBar.tsx`, plus `MeetingUILayoutContext.tsx`, `MeetingControls.tsx`, `chat/ChatInput.tsx`, `RoomInfoPanel.tsx` and `webxdc/WebxdcAppsDialog.tsx`) move those classes to `lg:` and `max-lg:`, so the CSS and the hook flip on the same pixel. The overlays that hand `meetStageShellClass` a phone-compact padding move with it — `webxdc/WebxdcStageOverlay.tsx`, `webxdc/WebxdcFrame.tsx`, `stage/StageScreenShareOverlay.tsx`, `youtube/YoutubeWatchOverlay.tsx` and `whiteboard/WhiteboardOverlay.tsx` — otherwise the same class string would mix the two breakpoints between 640px and 1023px.

What stays at `sm` in the meeting tree is the shadcn dialog and grid vocabulary — `sm:max-w-*`, `sm:justify-end`, `sm:gap-0`, `sm:flex-row`, `sm:flex-col`, `sm:grid-cols-*` — none of which reads the phone hook. The effect on the meeting is that phone chrome now applies below 1024px instead of 640px; that is the decision recorded in [01](./01-overview-and-units.md#one-breakpoint).

## PWA head and manifest

### Today

`apps/web/public/manifest.json` exists with `display: standalone`, `theme_color: #E11D48`, `background_color: #ffffff`, 192 and 512 icons and two screenshots. The viewport meta already carries `viewport-fit=cover`; safe-area insets and the visual-viewport fix are in place. Missing: a `theme-color` meta, an Apple touch icon, the mobile-web-app metas, and a maskable icon. There is no service worker, and none is added (see [01 — Non-goals](./01-overview-and-units.md#non-goals)).

### Head, in `routes/__root.tsx`

| Meta | Value | Why |
|---|---|---|
| `theme-color` | `--background` | Set by the pre-paint script and `applyTheme`, never rendered by React, because React 19 re-appends a recoloured hoistable meta on hydration. The system bar takes the surface colour, as Android's compact top bar does; rose was wrong there. |
| `mobile-web-app-capable` | `yes` | Standard replacement for the deprecated Apple-only meta. |
| `apple-mobile-web-app-title` | `Bedrud` | Home-screen label on iOS. |
| `apple-mobile-web-app-status-bar-style` | `default` | `black-translucent` forces white status text over a light theme. |

Link: `apple-touch-icon` to `/apple-touch-icon.png` (180 × 180).

The theme is class-based and user-selectable, so a media-query pair of `theme-color` metas would follow the OS and not the app. Instead the single meta is owned entirely by the pre-paint theme script in `__root.tsx` and by `applyTheme` in `lib/theme.store.ts`, which already toggles the `dark` class: whichever runs first creates the meta, and both write the resolved `--background` into it, so a dark app in a light OS gets a dark system bar from the first frame. Values come from the `--background` tokens: `#fafafa` light, `#0C0A09` dark.

React must never render this meta. React 19 hydrates head metas as hoistables keyed by their content, so the meta the pre-paint script has already recoloured no longer matches what React expects, and React appends a second, stale one — a dark page ends up carrying both `#0C0A09` and `#fafafa`. `documentMeta` therefore omits `theme-color`; `LIGHT_THEME_COLOR` survives only to pin `manifest.json` and `theme.css` to each other.

### Manifest

| Field | Value | Why |
|---|---|---|
| `start_url` | `/dashboard` | A signed-in user lands on rooms; a signed-out one is bounced to `/auth` by the route guard. Same as Android's auth force-route. |
| `scope` | `/` | Explicit. |
| `theme_color` | `#fafafa` | Matches the meta. |
| `background_color` | `#fafafa` | Splash background equals the page background, so launch does not flash white then grey. |
| icons | existing entries plus `/icon-maskable-512.png`, `purpose: maskable` | Android home screens mask icons; without a maskable entry the mark is shrunk inside a white disc. |

### Icons

Two new PNGs under `apps/web/public/`: `apple-touch-icon.png` (180 × 180, opaque) and `icon-maskable-512.png` (512 × 512, mark inside the 80% safe zone). Both are rendered by `apps/web/scripts/pwa-icons.mjs` (`sharp`, a new devDependency) from `apps/web/scripts/app-icon.svg`: the web's brand mark, the Lucide `Radio` glyph that `favicon.svg` and `logo512.png` already carry, drawn white on the brand rose `#E11D48`, which is also the Android adaptive icon's background colour. The generation command is recorded in `apps/web/public/README.md` next to the assets so they can be regenerated rather than hand-edited.

The Android launcher foreground today is a different mark (a white rounded square with a two-by-two grid, `apps/android` `ic_launcher_foreground.xml`). Making both clients share the Radio mark is an Android change and is tracked as a follow-up issue in the Android repository, not done here.

## Docs

- Root `DESIGN.md`: the Border Radius section and the matching "What NOT to Do" bullet are rewritten around the scale table above. The `--radius` line in Spacing & Layout follows.
- `apps/web/AGENTS.md` (the web design doc; there is no `apps/web/DESIGN.md`, although the root `AGENTS.md` still points at one): a Mobile detection section naming `lib/use-is-mobile.ts` as the only mobile check and `MOBILE_BREAKPOINT_PX` as the only number, and a corner note under Buttons pointing at the scale table.

## Tests

Written before the code, one file beside each source file, imperative descriptions, one `describe` per unit under test.

| Test file | Pins |
|---|---|
| `lib/use-is-mobile.test.ts` | `should report mobile at 1023px`, `should report desktop at 1024px`, `should follow change events across the breakpoint`, `should stop listening after unmount` |
| `design-tokens.test.ts` | the six radius token values; `--radius` is 12px; no universal `border-radius` rule |
| `lib/document-head.test.ts` | the metas and links the root route emits; the theme colour equals the light `--background` in `theme.css` |
| `lib/theme.store.test.ts` | `applyTheme` copies the resolved `--background` into the theme-color meta; `setTheme` refreshes it even when a view transition already toggled the class |
| `pwa-manifest.test.ts` | `start_url`, `scope`, `display`, `theme_color`, `background_color`, a maskable 512 icon, every PNG icon at its declared size, the Apple touch icon at 180px |

The head metas live in `lib/document-head.ts` rather than inline in the route so they can be tested without importing the router. The token and manifest tests read the files as text or JSON. They exist so a later edit cannot drift the three places (CSS, head, manifest) that must agree. Tests that read from disk run in Vitest's Node environment (a `@vitest-environment node` header), because jsdom's `URL` resolves `import.meta.url` against `http://localhost`; the one Node module they use, `node:fs`, is declared in `src/node-fs.d.ts` with just the two shapes they call, since the app's tsconfig keeps `@types/node` out to avoid its `setTimeout` colliding with the DOM's.

The step-by-step plan is [03 — Foundation Implementation Plan](./03-foundation-plan.md).

## Verification on a device

1. `make dev-web`, Android emulator Chrome at `http://10.0.2.2:7070`.
2. Chrome offers install; add to home screen; the icon is the Android mark, not a shrunken disc.
3. Launch from the home screen: standalone, no browser chrome, lands on `/dashboard` (signed in) or `/auth` (signed out).
4. System bar is the surface colour in light theme; switch the app to dark in settings and the bar follows without an OS change.
5. Dashboard cards, buttons, inputs, dialog and bottom nav show the scale corners in both themes; nothing is still square except `rounded-none` elements.
6. Meeting screen at a tablet width (768 to 1023px): phone chrome, not desktop chrome.
7. Playwright at 390 × 844 for before and after screenshots of dashboard, settings and meeting, both themes, for the pull request.

## Out of scope, tracked separately

- Root `DESIGN.md` palette values (`--bg #FFFBF9`) disagree with `theme.css` (`#fafafa`). Not touched here beyond the radius section.
- `ParticipantContextMenu.tsx` inline styles, already listed in `apps/web/DESIGN.md`.
- An offline fallback page via a service worker, and Web Push, if ever wanted.
