# 08 — Meeting chat sheet

Unit 2 of the PWA parity plan. Builds on unit 1, which set the shape scale and the single 1024px
phone breakpoint. Scope is the in-call chat on phones: the surface it opens on, the sheet primitive
that surface is built from, and the meeting chrome that currently hides while chat is open.

The comparison this unit acts on was made by reading
`apps/android/.../ui/screens/meeting/MeetingChatSheet.kt`, `MeetingChatPanel.kt`,
`ui/components/BedrudBottomSheet.kt` and `BedrudSheetHandle.kt` against
`apps/web/src/components/meeting/ChatPanel.tsx` and `MeetingPanels.tsx`.

This unit is the first to need a bottom sheet, so it also settles the primitive that units 3 and 4
will use. That choice is recorded in [01 — Overview](./01-overview-and-units.md#component-vocabulary)
as unit 2's to make.

## The sheet primitive

### Today

`apps/web/src/components/ui/sheet.tsx` exists — the stock shadcn wrapper over
`@radix-ui/react-dialog` — and has **no importers anywhere in the app**. It has never been used. It
offers four fixed sides, a scrim, and an open/closed pair of states. It has no drag, no intermediate
height, and no gesture dismissal.

Android's sheets are all one component, `BedrudBottomSheet`, over M3's `ModalBottomSheet`. The
platform arbitrates the heights; the app supplies a shape token, a handle, and insets.

### Target

A new `BedrudSheet` primitive in `components/ui/`, built on [vaul](https://vaul.emilkowal.ski)
1.1.2, is the one bottom sheet in the web app. `sheet.tsx` is deleted in the same change: it is dead
code today, and leaving two sheet primitives in `components/ui/` guarantees the next person picks
the wrong one.

vaul is chosen over extending the Radix sheet because the behaviour this unit specifies — two
detents, drag between them, velocity dismissal below the lowest one, and arbitration between
dragging the sheet and scrolling its content — is the whole of what vaul does, and is several
hundred lines of pointer-event work to write by hand. The dependency is small in practice: vaul
peers on `@radix-ui/react-dialog`, already a direct dependency at `^1.1.15`, so it adds one package
rather than a tree, and inherits Radix's focus trap, scroll lock and escape handling.

`BedrudSheet` fixes what `BedrudBottomSheet` fixes, for the same reason — a default is a suggestion,
and the one caller who takes the suggestion up is the one that renders wrong:

| Fixed, not a prop | Value | Equals |
|---|---|---|
| Top corners | `rounded-t-3xl` (28px) | Android `BedrudShapeTokens.sheetTop` |
| Container | `bg-[var(--meet-sidebar)]` | M3 `surfaceContainerLow` — lifts off the call behind it |
| Handle | `BedrudSheetHandle`, below | Android `BedrudSheetHandle` |
| Bottom inset | `env(safe-area-inset-bottom)` | Android `navigationBarsPadding()` |
| Side gutter | 16px | Android `Dimens.sheetPadding` |

Configurable: the snap points, whether the handle taps, the accessible label, and the content.

### The handle

`BedrudSheetHandle` draws a 32 × 4 pill at `rounded-full` in `--meet-fg-muted`, which already
resolves to a 50–55% foreground in both themes and so needs no new token. It matches the handle the
meeting controls bar draws for itself today, and the metrics Android uses
(`Dimens.meetingHandleWidth` 32dp, `meetingHandleHeight` 4dp).

One departure from Android, deliberate: the pill sits inside a **48px-tall full-width strip** that
is the drag and tap target, rather than Android's 28dp padded box. WCAG 2.5.8 asks for 44px, a drag
target that is hard to catch is worse on a touchscreen than in an emulator, and the strip costs no
visible pixels — it is transparent.

The strip is a `button` when it has an `onClick`, with an explicit `aria-label` saying what tapping
does, and a plain `div` when it does not. Android makes the same split, for the same reason: a sheet
with one height has nothing for a tap to do.

## The chat sheet

### Today

`ChatPanel.tsx:254` gives the phone layout
`fixed left-[var(--app-offset-left)] top-[var(--app-offset-top)] h-[var(--app-height)] w-[var(--app-width)]`.
Chat is a full-screen surface over the call. Nothing of the room is left on screen: not the tiles,
not the room name, not who is speaking. `MeetingPanels.tsx:84` then sets `mobileOverlayOpen`, which
hides the top-right chrome cluster and passes `hideOnMobile` to the controls bar, because both would
otherwise draw on top of that surface.

Because the whole call disappears, the panel grows a compensating strip of its own —
`mobileMeetingBar` at `ChatPanel.tsx:135`: mic, deafen, and a participants count, 48px tall, present
only on phones and only in chat. It exists to give back three controls the surface took away.

Android does none of this. `MeetingChatSheet` is a `ModalBottomSheet` over the live call, and
`MeetingChatPanel` inside it is a message list and a composer — no mic, no deafen, no participant
count.

### Target

On phones, chat opens as a bottom sheet over the live call, at half the visible viewport. The tiles
stay on screen above it behind the scrim, so the reader can still see who is talking while they
type.

Three heights, as on Android:

| State | Height | Reached by |
|---|---|---|
| Half | 50% of `--app-height` | opening chat; dragging down from full; tapping the handle at full |
| Full | `--app-height` minus (12px + `env(safe-area-inset-top)`) | dragging up; tapping the handle at half; the keyboard opening |
| Dismissed | — | dragging down from half; the close button; Escape; the scrim |

Full deliberately stops short of the top. A sheet that reaches the status bar is a screen again, and
the whole point of the change is that the call never fully goes away. The 12px gap is Android's
`Dimens.space12` above the status bar.

In vaul terms that is `snapPoints={[0.5, 1]}` with a controlled active snap point, the drawer's own
height capped by the gap, and `dismissible` left on so a drag below the lowest snap point closes —
which is exactly "a second drag down dismisses".

### The keyboard takes it to full

A keyboard over a half-open sheet leaves a sliver of conversation, so focusing the composer expands
the sheet to full. Android reads `WindowInsets.ime`; the web reads the same event the app already
listens to. `lib/visual-viewport.ts` publishes `--app-height` from the Visual Viewport API and
updates it on `resize`, which is precisely what an on-screen keyboard fires. The sheet expands when
the visible viewport shrinks by more than a threshold, and does not collapse again on its own when
the keyboard closes — a reader who expanded to type is reading what they typed.

### The input dock stays on the bottom edge

At every height the composer sits on the sheet's bottom edge, where the call's own controls bar is,
so the thumb finds it in the same place whether the sheet is half or full.

This is the one place the Android file spends most of its length, because a Compose bottom sheet
lays its content out from its top edge downwards and a full-height column pushes the dock off
screen at half. The web has no such problem: the sheet element is the height of its snap point and a
`flex flex-col` with the message list at `flex-1` puts the composer on the bottom edge for free.
That difference is worth writing down, because the next person to read both files will wonder why
one is 170 lines and the other is not.

## What the phone surface loses

`mobileMeetingBar` is removed. It exists only to compensate for a full-screen chat, and with the
call visible behind a half sheet its three controls are no longer taken away — they are a drag
down, which is also how Android reaches them.

This is an honest cost and it is worth stating plainly: with the sheet open, the mic cannot be
toggled without first moving the sheet, on the web and on Android alike. The alternative — keeping
a 48px strip of meeting controls inside a sheet that is half a phone tall — spends the scarcest
space on the surface to duplicate controls that are two thumb-widths away.

## What the meeting chrome regains

`mobileOverlayOpen` no longer hides the top-right cluster when chat is open. The cluster sits in the
56px header band, the sheet at full stops 12px below it, and the chat toggle needs to keep showing
its active state so a second tap closes what a first tap opened.

The controls bar keeps `hideOnMobile` while chat is open. The sheet covers the bottom half of the
screen at every height it has, so the bar would be drawing underneath it.

`participantsOpen` keeps its current full-screen behaviour and its share of `mobileOverlayOpen`.
Participants become a sheet in unit 4, not here.

## The CSS coupling, which has to survive

`meeting.css` carries 23 rules keyed on `aside[aria-label="Chat"]`, `aside[data-chat-overlay="true"]`
and `aside[data-elevated-chat="true"]`. They exist because the chat panel portals to `document.body`
and therefore falls outside `.meet-room`, and they are what maps the `text-white/*` utilities inside
chat onto the meeting foreground tokens.

A vaul drawer portals to the body too, and renders a `div`, not an `aside`. Left alone, all 23 rules
would silently stop matching and chat text would render at raw `text-white/95` on a light theme.

The sheet therefore carries `data-chat-overlay="true"` and `aria-label="Chat"` on an element the
existing selector list matches. The selector list gains the sheet's element type rather than each
rule being rewritten. A test pins this, because it is a failure with no error message.

## Desktop is untouched

Above 1024px `ChatPanel` keeps both of its current presentations: the 320px docked sidebar when
pinned, and the same width as an overlay when not. The pin control, the left/right dock side, and
the elevated WebXDC dock (`MeetingElevatedLeftDock`) are desktop-only and unchanged.

The sheet is chosen by the same `useIsMobile()` the panel already calls at `ChatPanel.tsx:80`, which
is unit 1's single 1024px breakpoint. No new breakpoint is introduced by this unit.

## Tests

The repo has 58 test files outside `vendor/` and **not one of them renders a component**. The
established pattern is three shapes, and this unit uses all three rather than introducing a fourth:
pure logic in a plain `.test.ts` (`chatBubbleStyles.test.ts`), hooks through
`renderHook` (`use-is-mobile.test.ts`), and a class or marker pinned by reading the source file with
`// @vitest-environment node` (`design-tokens.test.ts`, and unit 6's `dialog-breakpoints.test.ts`).

That constraint shapes the code, not just the tests. Two pure modules come out of it, and both earn
their place:

- **`components/ui/sheetKeyboard.ts`** — `shouldExpandForKeyboard()` answers whether a shrink of the
  visible viewport is an on-screen keyboard or a browser toolbar sliding away. Tested in
  `sheetKeyboard.test.ts`: a large shrink is a keyboard; a toolbar-sized one is not; a viewport
  growing back does not collapse the sheet.

  The module is deliberately that small, because two things that look like they belong in it do not.
  Which detent a release lands on, the velocity that dismisses instead of snapping, and whether a
  fast flick may skip a detent are vaul's `closeThreshold`, `snapToSequentialPoint` and drag
  handling. The sheet's full height is `--app-height` minus 12px minus `env(safe-area-inset-top)`,
  three values the stylesheet already holds, so it is a `--meet-sheet-max-height` token beside the
  meeting's other tokens. Either one rewritten in TypeScript would be a second source for something
  the running sheet gets elsewhere: the tests would pin the copy, and pass while the sheet drew from
  the original. What this unit owns is the two snap points it hands vaul and the keyboard question
  vaul has no opinion about.
- **`components/meeting/chatSurface.ts`** — `chatSurfaceFor(isMobile, stuck, elevated)` returns
  `'sheet' | 'dock' | 'overlay'`. The three-way choice is currently four booleans read across
  `ChatPanel.tsx:80-86`, and this unit adds a fourth case to it. Tested in `chatSurface.test.ts`.

Then two source-text tests, in the node environment:

- `components/ui/bedrudSheet.test.ts` — the fixed values are actually fixed: `rounded-t-3xl`, the
  `--meet-sidebar` container, `env(safe-area-inset-bottom)`, the 32 × 4 `rounded-full` handle and its
  48px strip. Also that the file carries no `sm:` or `md:` prefix, the same guard unit 6 used.
- `components/meeting/chatMarkers.test.ts` — the regression test for the CSS coupling above. It reads
  `meeting.css`, extracts every selector naming a chat marker, and asserts each one matches the
  element `ChatPanel.tsx` renders for the sheet. It also asserts `mobileMeetingBar` is gone, so the
  strip cannot come back by accident.

## Verification on a device

Beyond the suite, and stated as observed rather than assumed:

- Android emulator Chrome against the dev server, installed to the home screen and launched
  standalone, in a room with a second participant, so there is something to see behind the sheet.
- Open chat: it rests at half, tiles visible above it.
- Drag up: full, with the room name band still showing above it.
- Drag down twice: half, then gone.
- Tap the handle at each height: it toggles to the other.
- Focus the composer: the sheet goes full and the composer sits above the keyboard.
- Send a message with the sheet half open and confirm it arrives on the second client.
- Both themes, and a landscape rotation, which halves the available height.

## Out of scope, tracked separately

- The controls pill (unit 3) and the invite/participants sheet (unit 4) are the other two callers of
  `BedrudSheet`. This unit ships the primitive with one caller.
- `ParticipantsList` stays a full-screen phone surface until unit 4.
- The meeting's own radius tokens in `meeting.css` (`--meet-controls-bar-radius`,
  `--meet-btn-leave-radius`, `--meet-gallery-icon-radius`) keep their current values; unit 3 moves
  them to the `xxl` corner.
- Chat message actions — reactions, the message menu, polls, the image lightbox — are unchanged.
  They are content inside the panel and do not care what surface the panel is on.
