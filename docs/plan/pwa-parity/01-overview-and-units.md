# 01 — Overview & Units

**Status: in progress.** Unit 1 is specified in [02 — Foundation](./02-foundation.md) and unit 6 in [06 — Dashboard parity](./06-dashboard-parity.md); units 2 to 5 get their own file when they start.

## Problem

Bedrud ships a native Android client (`apps/android`, Material 3, rounded, rose + teal) and a web app (`apps/web`, shadcn/ui, TailwindCSS v4) that also serves phones through the browser and as an installed PWA. The two clients share a brand and a backend but not a phone experience: the web meeting opens chat as a full-screen modal where Android opens a sheet over the live call, the web controls bar is a shrunken desktop bar where Android floats a pill, and every corner on the web is flattened to 4px where Android uses a rounded scale. The iOS client, when it arrives, will follow Android.

Someone who installs the web app on a phone should recognise the Android app in it.

## Goals

1. The installed web app and the Android app share one information architecture, one navigation model, and one component vocabulary on phones and tablets.
2. The web app follows standard PWA practice: a web manifest, `standalone` display, safe-area handling, theme colour, and installability on Android Chrome and iOS Safari. Nothing custom.
3. The web keeps its own stack. shadcn/ui, Tailwind tokens, and the existing `theme.css` single-file brand layer stay; Android patterns are expressed in them.
4. Every design decision that both clients depend on is written down here and in `DESIGN.md`, so the iOS client can follow the same table.

## Non-goals

- **Pixel parity with Compose.** Same shapes, same placement, same behaviour; not the same pixels.
- **Material 3 on the web.** No `material-web`, no MUI, no second design system next to shadcn.
- **A service worker.** Install works from the manifest alone on current Chrome and iOS Safari. A video-meeting app has no useful offline mode, and caching `/api` under an authenticated session is a hazard. An offline fallback page can be added later without touching anything here.
- **Native-only capabilities.** A call surviving the app being backgrounded on iOS, dialer and telecom integration, proximity screen-off, and screen share on iOS Safari (no `getDisplayMedia`) are out of reach for a PWA and are not simulated.

## Settled decisions

### Shape scale

The web adopts the Android shape scale, on every width including desktop, so the brand has one set of corners. The scale is applied through the existing Tailwind radius tokens; the global `border-radius … !important` override that flattened everything to 4px is removed. Details and the class-to-token table are in [02 — Foundation](./02-foundation.md#shape-scale).

### One breakpoint

Phone layout applies below 1024px, the `lg` Tailwind breakpoint the dashboard shell already uses. The meeting screen, which switched at 640px, and the create-room dialog, which switched at 768px, move to the same line. Tablets in portrait get the phone layout, as they do on Android. A desktop browser window narrower than 1024px gets it too; that is the ordinary responsive contract.

### Component vocabulary

| Android | Web equivalent | Where it lives |
|---|---|---|
| `BedrudBottomNavigationBar` (Rooms / Profile / Settings / Admin) | `MobileBottomNav` | exists, `components/dashboard/MobileBottomNav.tsx` |
| Dashboard FAB, "New room" | FAB in `MobileBottomNav` | exists |
| `BedrudCompactTopBar` | dashboard mobile header | exists |
| `BedrudBottomSheet` | shadcn `Sheet` with `side="bottom"`, drag handle, `sheetTop` corners | unit 2 decides Sheet vs a vaul-based Drawer, once for all sheets |
| `MeetingChatSheet` (half / full / dismiss over the live call) | bottom sheet over the meeting | unit 2 |
| `MeetingControlsPanel` (floating pill, handle unfolds options upward) | floating controls pill | unit 3 |
| `MeetingInviteSheet` (avatar grid + share targets) | invite sheet | unit 4 |
| `SettingsContent` (one scrolling page of card sections) | mobile settings page | unit 5 |
| `QuickJoinBar` + `FilterRow` (dashboard) | quick-join bar + filter chips | unit 6 |

## Units

Each unit is one branch and one pull request, cut fresh from `master`.

| # | Unit | Scope | Status |
|---|---|---|---|
| 1 | Foundation | shape scale, one breakpoint hook, PWA head metas and manifest, docs, contract tests | in progress, [02](./02-foundation.md), [03](./03-foundation-plan.md) |
| 2 | Meeting chat sheet | chat as a bottom sheet over the live call: half on open, full on drag or keyboard, dismissed on a second drag down | planned |
| 3 | Meeting controls pill | floating bottom-centre pill (camera, screen share, mic, chat, hang up), drag handle unfolds room options upward in the same surface, no "more" menu | planned |
| 4 | Invite sheet | participants and invite as one sheet: avatar grid, count, share targets, room link | planned |
| 5 | Mobile settings page | one scrolling page of card sections on phones | planned |
| 6 | Dashboard parity | quick-join bar on phones with pasted-link support, filter chips over one merged room list, and the dashboard tree moved to the shared breakpoint | done, [06](./06-dashboard-parity.md), [07](./07-dashboard-parity-plan.md) |

## Verification

Every unit is verified on a device, not only by tests:

- Android emulator Chrome against the dev server, installed to the home screen, launched standalone.
- Playwright at a phone viewport for repeatable screenshots, both themes.

Unit 1 has no server dependency. Units 2 to 4 need a second participant; the Android `dev` build on the emulator and a Playwright guest against the same room cover that.
