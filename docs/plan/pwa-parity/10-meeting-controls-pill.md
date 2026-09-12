# 10 — Meeting controls pill

Unit 3 of the PWA parity plan. On phones the in-call controls become the Android bar: one
bottom-anchored surface carrying five controls, with a drag handle that unfolds the room options
upward inside the same surface. The two full-screen dialogs the phone reaches those options through
today are deleted.

Unit 1 ([02](./02-foundation.md)) supplies the 1024px breakpoint and the shape scale this depends
on, and is the branch this unit is cut from. Unit 2 supplies the chat sheet the chat control opens;
its spec, [08](./08-meeting-chat-sheet.md), arrives with that unit's own branch and does not resolve
until both have merged. Nothing here depends on unit 2's code, for the reason in
[The handle](#the-handle).

Desktop is untouched apart from two corner values.

## The controls surface

### Today

`ControlsBar.tsx` renders one `#meet-controls` element for both widths, a centred auto-width pill
at `bottom-5` on desktop and `bottom-[calc(12px+env(safe-area-inset-bottom))]` on phones, differing
otherwise only in paddings, icon sizes and which children render. On a phone it carries, left to
right: camera, screen share, app gallery when WebXDC is enabled, leave, push-to-talk when that mode
is on, mic, deafen, an audio-devices chevron, and a `⋯` button. Nine controls on a 375px screen.

Two of those open full-screen dialogs that cover the call completely:

- the `⋯` dialog, a settings-app surface with its own header, back button and an animated room-info
  sub-page (`morePage`, `moreNavDir`)
- the audio dialog behind the chevron, holding microphone and speaker device lists and the noise
  suppression modes

This is the shape unit 2 removed for chat: a phone surface that replaces the meeting instead of
sitting over it.

### Target

Android's `MeetingControlsPanel`. One surface anchored to the bottom, with the controls row as its
floor. The options do not arrive as a second surface — the panel grows taller and the row your
thumb is resting on does not move. Android's own file records why it departs from the app's sheet
standard to get this: as a sheet, the options slid up over the real bar carrying a duplicate copy
of the same five controls, at a second elevation, and the bar jumped.

The web follows that decision. Below 1024px, `ControlsBar` renders `MeetingControlsPill` instead of
the bar above.

| Fixed, not a prop | Value | Equals |
|---|---|---|
| Corner | `rounded-3xl` (28px) | Android `BedrudShapeTokens.controlsBar` |
| Width | `inset-x-2`, full width less 8px a side | Android `Dimens.meetingScreenMargin` |
| Container | `bg-[var(--meet-chrome)]` with `backdrop-blur-xl` | Android `surfaceContainerHigh` |
| Border | `border-[var(--meet-border-subtle)]` | Android `colors.divider` |
| Bottom inset | `12px + env(safe-area-inset-bottom)` | Android `navigationBarsPadding()` |
| Expand duration | 320ms | Android `Motion.meetingOptionsExpandMs` |
| Collapse duration | 260ms | Android `Motion.meetingOptionsCollapseMs` |
| Easing | `cubic-bezier(0.4, 0, 0.2, 1)` | Android `FastOutSlowInEasing` |

Full width is the parity choice and a correction: Android's bar is `fillMaxWidth()` inside a screen
margin, not the centred auto-width pill the web draws. At 375px the centred pill had already grown
to within a few pixels of the screen edge, so it read as full width while behaving as though it
were not — `meetControlsDockClass` still applied `left-1/2 -translate-x-1/2` to it. The phone
branch drops that helper entirely; only the desktop bar docks.

### The handle

32 × 4px, `rounded-full`, `bg-[var(--meet-fg-muted)]` at 55% alpha, inside a 48px tap target that
spans the surface width.

It is **not** `BedrudSheetHandle` from unit 2. That component is a `Drawer.Handle` and only works
inside a vaul `Drawer.Root`; this panel is not a drawer. Android draws the same distinction in the
same place, and gives its bar handle its own tint for the same reason. Two handles will therefore
exist once both units land, matching on metrics and differing in what they are attached to. Folding
the pill into a shared primitive after the fact is listed under
[Out of scope](#out-of-scope-tracked-separately).

### The gesture

| Input | Result |
|---|---|
| Tap the handle | toggles expanded |
| Drag up past 24px | expands |
| Drag down past 24px | collapses |
| Tap the scrim | collapses |
| Escape | collapses |

24px is Android's `Dimens.meetingHandleSwipeThreshold`. The panel answers the same gesture in both
directions, so whatever opened it puts it away again. Escape is the web's answer to Android's
`BackHandler`.

While expanded, a scrim covers the call, fading with the same duration and easing as the panel so
the two read as one movement rather than a fade that finishes early and leaves the panel travelling
on its own. Its colour is a new token, `--meet-scrim`, because `meeting.css` has no scrim today —
the dialogs this unit deletes relied on the shadcn overlay, which goes with them.

## The five controls

The row is three sections with equal-weight sides, so the mic slot stays centred under the handle
whatever its label says.

| Slot | Control | Resting | Other states |
|---|---|---|---|
| Left | Camera | `--meet-control` | off: `--meet-btn-alert-*`, crossed icon |
| Left | Screen share | `--meet-control` | sharing: `--meet-btn-alert-*` |
| Centre | Mic pill | below | below |
| Right | Chat | `--meet-control` | open: `--meet-btn-muted-*`, unread badge |
| Right | Leave | `--meet-btn-leave-bg` | pill corner, no off state |

Those are the three button states `btnIconCn` already defines — resting, `muted` for an active
toggle, `alert` for a control that is off or capturing. Android tints a disabled camera or mic with
a dim neutral rather than the alert red the web uses. The web keeps its own language: the off-state
colour is an established pattern in this file and changing it is not this unit's job.

Chat and leave collapse the panel before they act, so the surface is never left expanded over a
screen the user just moved away from.

The chat control is new to the bar on the web, where chat has toggled from the top-right cluster.
That toggle is hidden below 1024px so chat has one entry point on a phone, and the unread badge
moves with it. Above 1024px the top-right toggle is unchanged.

## The mic pill

One pill for both input modes, so the bar's rhythm never changes:

| Mode | State | Label | Container |
|---|---|---|---|
| Voice activity | mic open | Speak | `--meet-control` |
| Voice activity | muted | Muted | `--meet-btn-alert-bg` |
| Push to talk | idle | Push to Talk | transparent, 1px `--meet-border` |
| Push to talk | held | Talking… | `--meet-btn-muted-bg` |

Tap toggles in voice-activity mode; press and hold transmits in push-to-talk mode. This absorbs the
separate push-to-talk button, so the phone bar loses a control while gaining a label.

The pill keeps one width across every mode and state. All four labels are rendered stacked in the
same grid cell, three of them `invisible`, and the pill sizes to the widest — the technique Android
uses, for the reason Android records: nothing in the bar may move when the mode or the hold state
changes. The visible label is centred, not start-aligned.

Width is capped at 136px (`Dimens.meetingMicPillMaxWidth`) so the side clusters never squeeze.

Out of scope here: Android's animated status ring around the pill (reconnecting, voice blocked) and
its live capture meter. Neither has a web equivalent today and both are features rather than
parity of an existing control.

## The options panel

Every row the phone reaches through `⋯` or the audio chevron today, in one panel. The order is the
room first, then audio, then whatever the room has put on the stage — read bottom-up from the
controls, because the row you were already touching stays the panel's floor.

| Row | Condition | Kind |
|---|---|---|
| Show / Hide videos | video sidebar available | toggle |
| Public / Private room | room access callback present | action |
| Room info | room id present | action |
| Copy room link | always | action |
| Deafen | always | toggle |
| Microphone — one row per input device | at least one device | heading, then a checked selection |
| Speaker — one row per output device | at least one device | heading, then a checked selection |
| Noise suppression — one row per mode | always | heading, then a checked selection |
| Settings | always | action, opens the settings dialog |
| Fullscreen | `document.fullscreenEnabled` | action |
| Open / Close whiteboard | whiteboard enabled or hosted | action |
| Share / Stop YouTube | YouTube enabled or hosted | action |
| App gallery | WebXDC enabled | action |

Toggles keep the panel open and carry a trailing check in the accent tint, so the flip is visible.
Actions that lead somewhere else close the panel on the way. This is Android's rule, verbatim. A
device selection is a toggle by that rule: the panel stays open and the check moves.

An audio group whose list is empty contributes nothing, heading included — a "Speaker" heading over
no speakers reads as a bug rather than as an empty state. A noise mode the instance offers but the
browser cannot run renders disabled, as it did in the dialog this replaces.

Room info was an animated sub-page inside the `⋯` dialog. It becomes an ordinary row that opens the
existing room info panel, and the sub-page machinery — `morePage`, `moreNavDir`, `morePageAnim`,
the header back button — is deleted with the dialog that held it.

Android's "Disable all cameras" row has no web equivalent; the web has no
hide-all-incoming-video setting to expose. It is not invented here.

### Where this diverges from Android

Android's panel never scrolls: five rows always fit. The web's can reach twelve, which at 56px a row
is roughly 670px of panel before the controls are counted — more than an 812px phone has.

The panel is therefore capped at `--meet-controls-panel-max-height` minus the controls row and
scrolls past that, with the handle and the controls row both staying fixed outside the scroll area.
Unit 2 defines an identical calculation as `--meet-sheet-max-height`, but on a branch this one is
not cut from; collapsing the two is listed under
[Out of scope](#out-of-scope-tracked-separately). This is the
one place the unit knowingly differs from the Android surface it mirrors, and it differs because
the web client has features the Android client does not.

## What the phone surface loses

- **The `⋯` full-screen dialog**, its header, its back button and its room-info sub-page.
- **The audio full-screen dialog** behind the chevron. Its device lists and noise modes become
  panel rows.
- **The separate push-to-talk button**, absorbed by the mic pill.
- **The deafen button and the audio chevron**, which become panel rows.
- **The top-right chat toggle**, which moves into the bar.

Both deleted dialogs were `isMobile`-gated (`open={moreOpen && isMobile}`,
`open={audioOpen && isMobile}`), so the desktop dropdown and desktop audio menu are untouched by
their removal.

After this unit the phone meeting has no full-screen surface left except the participants list,
which unit 4 moves onto `BedrudSheet`.

## Tokens

Two are new. `meeting.css` is the meeting's token layer, so both live there rather than in a
component:

| New token | Value | For |
|---|---|---|
| `--meet-scrim` | `rgba(23, 23, 23, 0.32)` light, `rgba(0, 0, 0, 0.32)` dark | the dim over the call while the panel is open |
| `--meet-controls-panel-max-height` | `calc(var(--app-height, 100svh) - 12px - env(safe-area-inset-top, 0px))` | the panel's ceiling |

0.32 is Android's `ScrimAlpha`. The 12px gap above the panel is Android's `Dimens.space12`, the
same clearance unit 2's sheet leaves for the status bar.

Two more are retuned. `meeting.css` still carries the flat scale unit 1 replaced everywhere else:

| Token | Today | Target | Why |
|---|---|---|---|
| `--meet-controls-bar-radius` | 4px | 28px | Android `controlsBar` = `xxl`; unit 1's table already lists the controls bar under `rounded-3xl` |
| `--meet-btn-leave-radius` | 3px | 9999px | Android `MeetEndCallButton` uses `BedrudShapeTokens.pill` |

`--meet-chat-bubble-radius` and `--meet-chat-bubble-radius-near` are defined in terms of these two
and follow them. Their values are unit 2's business and are left alone here beyond that inheritance.

These tokens are shared with the desktop bar, so **desktop corners change too**. That is unit 1's
stated goal — one set of corners on every width — and is the only desktop-visible change in this
unit.

## Files

| File | Responsibility |
|---|---|
| `components/meeting/MeetingControlsPill.tsx` | the surface: handle, scrim, expanded state, gesture |
| `components/meeting/MeetingCallControlsRow.tsx` | the five controls |
| `components/meeting/MeetingMicPill.tsx` | the centre slot, both input modes |
| `components/meeting/MeetingOptionsPanel.tsx` | the rows, their scroll and their close rule |
| `components/meeting/controlsPanelDrag.ts` | pure: a drag and the current state to the next state |
| `components/meeting/meetingOptionRows.ts` | pure: capability flags to the row list |

`meetingOptionRows.ts` lifts the 150-line `moreRows` `useMemo` out of `ControlsBar.tsx`, which is
1201 lines. It is the one part of that file that is pure and can be tested without a browser.

## Tests

The repo's test house style: every test is `.test.ts`, and takes one of three shapes — pure logic,
a React Testing Library `renderHook`, or a source-text read under `// @vitest-environment node`.
There are no component render tests, and this unit does not add the first one.

| File | Shape | Covers |
|---|---|---|
| `controlsPanelDrag.test.ts` | pure logic | up past threshold expands, down past threshold collapses, movement under 24px does nothing in either direction, a drag in the direction the panel is already in is a no-op |
| `meetingOptionRows.test.ts` | pure logic | each condition adds or omits its row, order is stable, toggles are marked as staying open, actions are marked as closing |
| `controlsPill.test.ts` | source text | the surface carries `rounded-3xl` and not a second `rounded-*`; the phone branch does not call `meetControlsDockClass`; both `isMobile`-gated dialogs are gone from `ControlsBar.tsx`; the four mic labels are all present in `MeetingMicPill.tsx` |
| `meetingShapeTokens.test.ts` | source text | `meeting.css` sets the two radius tokens to their target values |

The third file's negative assertions matter more than the positive ones. `cn` is
`twMerge(clsx(…))`, so a second `rounded-*` on the surface silently deletes the first, and the
deleted dialogs are the kind of thing a later change re-adds by reflex.

## Verification on a device

Tests pin what the files say. They cannot show where the surface lands, and unit 2 shipped three
defects that were green in CI and wrong on screen. Every line below is driven in a live meeting:

- [ ] Collapsed, 375 × 812: the bar spans the width less 8px a side, corners 28px, five controls.
- [ ] Tap the handle: the panel unfolds upward and the controls row does not move.
- [ ] Tap the handle again: it folds. Drag up opens, drag down closes, both at 24px.
- [ ] The scrim dims the call and closes on tap. Escape closes.
- [ ] With every feature flag on, the panel reaches its cap and scrolls, with the handle and the
      controls row fixed outside the scroll.
- [ ] A toggle row flips in place and leaves the panel open; an action row closes it.
- [ ] Push-to-talk: the pill reads "Push to Talk", fills and reads "Talking…" while held, and the
      bar does not shift width when it changes.
- [ ] Chat opens the unit 2 sheet and the panel collapses first.
- [ ] Both themes.
- [ ] 1280px: the desktop bar is unchanged except for its corners, and its `⋯` and audio menus
      still open.

Before and after captures at 375 × 812 in both themes, the before taken on
`feat/pwa-parity-foundation` at the start of the unit.

## Out of scope, tracked separately

- **One handle primitive.** This unit's handle and unit 2's `BedrudSheetHandle` share metrics and
  differ in host. Folding them into one component is worth doing once both have merged and the
  shared shape is visible in one tree.
- **One height calculation.** `--meet-controls-panel-max-height` here and
  `--meet-sheet-max-height` in unit 2 are the same expression under two names, because neither
  branch can see the other. One of them should absorb the other after both merge.
- **The mic status ring and the capture meter.** Android draws a reconnecting arc and a live level
  meter on its pill. Both are new capability on the web, not parity.
- **Disable all incoming cameras.** Android has the setting; the web has no such state to toggle.
- **`ControlsBar.tsx` at 1201 lines.** This unit removes roughly 300 of them and moves the pure
  part out. What remains is still one file carrying the desktop bar, two dialogs and the device
  lists.
