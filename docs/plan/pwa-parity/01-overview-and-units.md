# 01 — Overview & Units

**Status: in progress.** Unit 1 is specified in [02 — Foundation](./02-foundation.md); units 2 to 5 get their own file when they start.

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
| `BedrudBottomSheet` | `BedrudSheet`, `components/ui/BedrudSheet.tsx` — vaul, drag handle, `rounded-t-3xl` | built in unit 2; units 3 and 4 reuse it rather than adding a second sheet |
| `MeetingChatSheet` (half / full / dismiss over the live call) | bottom sheet over the meeting | unit 2 |
| `MeetingControlsPanel` (floating pill, handle unfolds options upward) | `MeetingControlsPill`, `components/meeting/MeetingControlsPill.tsx` | built in unit 3 |
| `MeetingInviteSheet` (avatar grid + share targets) | `MeetingInviteSheet`, on `BedrudSheet` | unit 4 |
| `SettingsContent` (one scrolling page of card sections) | mobile settings page | unit 5 |

## Units

Each unit is one branch and one pull request. Units 1 to 3 were cut fresh from `master`, which cost
them: units 2 and 3 could not see each other's work and each built a drag handle and a viewport-height
token the other already had ([#162](https://github.com/themadorg/bedrud/issues/162),
[#163](https://github.com/themadorg/bedrud/issues/163)). Units 2 and 3 are now merged into
`feat/pwa-parity-foundation`, and unit 4 is cut from that instead, so it can build on the bottom
sheet rather than beside it.

| # | Unit | Scope | Status |
|---|---|---|---|
| 1 | Foundation | shape scale, one breakpoint hook, PWA head metas and manifest, docs, contract tests | in progress, [02](./02-foundation.md), [03](./03-foundation-plan.md) |
| 2 | Meeting chat sheet | chat as a bottom sheet over the live call: half on open, full on drag or keyboard, dismissed on a second drag down | done, [08](./08-meeting-chat-sheet.md), [09](./09-meeting-chat-sheet-plan.md) |
| 3 | Meeting controls pill | floating bottom pill (camera, screen share, mic, chat, hang up), drag handle unfolds room options upward in the same surface, no "more" menu | done, [10](./10-meeting-controls-pill.md), [11](./11-meeting-controls-pill-plan.md) |
| 4 | Invite sheet | participants and invite as one sheet: avatar grid, count, share targets, room link | in progress, [12](./12-invite-sheet.md) |
| 5 | Mobile settings page | one scrolling page of card sections on phones; dashboard alignment (quick-join bar, filter chips) if still needed | planned |

## Verification

Every unit is verified on a device, not only by tests:

- Android emulator Chrome against the dev server, installed to the home screen, launched standalone.
- Playwright at a phone viewport for repeatable screenshots, both themes.

Unit 1 has no server dependency. Units 2 to 4 need a second participant; the Android `dev` build on the emulator and a Playwright guest against the same room cover that.
