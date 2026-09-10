# 04 — Mobile settings page

Unit 5 of the PWA parity plan. Builds on unit 1, which set the shape scale and the single 1024px
phone breakpoint. Scope is the settings surface only; the dashboard is checked afterwards and
reported separately.

## One scrolling page

### Today

`/settings` is a drill-down. `routes/settings.tsx:30` computes whether the current path is the
index and renders one of two things: a list of five rows, each pushing to a sub-route, or the
sub-route's panel behind a "‹ Settings" back link. The five rows are Appearance, Audio, Video,
Security and Experimental; Profile is a sibling top-level route reached from the bottom navigation.
Each sub-route file renders an `<h1>` and one panel.

Android has no drill-down anywhere. `SettingsContent` in the Android client is a single
`verticalScroll` column with 16dp gutters and 16dp between cards, holding five outlined cards, each
led by a `CardSectionHeader` in `labelLarge` and the primary colour.

### Target

`/settings` becomes one scrolling page. All five panels render inline, in the order the drill-down
list used, each preceded by a section header in the Android shape: a small label in the primary
colour, above the panel rather than inside it.

The header is `text-xs font-semibold uppercase tracking-wide text-primary`, the web reading of
Android's `labelLarge` in the primary colour. It is the only new visual element this unit adds.

The page keeps the `<h1>Settings</h1>` it has today, so the heading order stays `h1` then one `h2`
per section.

Measured at 390px signed out, the five panels are 472, 1480, 708, 1141 and 587 pixels tall, so the
page is roughly 4.5 screens. That is ordinary for a phone settings screen and is the price of
having one page rather than two navigation models.

### Sections keep their own cards

A section is a header plus whatever the panel already renders. Audio renders five cards, Security
three, Video two, and the rest one each. They are left exactly as they are.

This is a deliberate departure from Android, where a section *is* a card. Matching that literally
would mean rewriting the internals of panels that the in-meeting settings dialog also renders, and
the parity goal is the same shapes and placement, not the same pixels. The section header carries
the parity; the cards beneath it stay the web's own.

### The sub-routes become anchors

`/settings/appearance`, `/settings/audio`, `/settings/video`, `/settings/security` and
`/settings/experimental` keep working and keep their URLs; they do not redirect. Each renders the
same one-page screen and scrolls its section into view, so existing links, bookmarks and the
browser's back button behave as before. The back link at the top of a sub-page goes away with the
drill-down.

Each section is anchored by a stable `id` matching its route segment, so `/settings#audio` works
too.

Each sub-route keeps the document title it sets today, so `/settings/audio` still reads
"Audio — Bedrud" while `/settings` reads "Settings — Bedrud".

## The dialog's breakpoint

### Today

`BedrudSettingsDialog` is the other settings shell: it is what a phone visitor gets in a meeting and
from the home page, and it has its own phone push-list. It branches at Tailwind's `sm`, 640px, in
six places: the mobile list and desktop sidebar at `:331` and `:333`, the overlay height at
`:310-313`, the dialog size at `:321`, two full-bleed variants at `:324-327`, and the hidden close
button at `:328`.

Unit 1 unified every other phone check on `MOBILE_BREAKPOINT_PX`, 1024px, but recorded the `sm:`
prefixes outside the meeting tree as unaudited. These are the settings-surface half of that debt. At
800px the dialog shows its desktop sidebar while the app around it is in phone layout.

### Target

All six move to `lg:` and `max-lg:`, so the dialog switches at the same line as everything else.

Its push-list stays a push-list. The dialog is not the Android settings screen: Android's in-meeting
settings is a bottom sheet, which units 2 to 4 cover. Only the breakpoint changes here.

## The desktop route on a phone

### Today

`/dashboard/settings` has no mobile handling at all. A phone that lands there directly gets the
desktop pill tab bar inside a `max-w-4xl` column. The phone routes redirect the other way already:
`routes/settings.tsx:33` sends desktop visitors to `/dashboard/settings`.

### Target

`/dashboard/settings` redirects phone visitors to `/settings`, mirroring the redirect that already
exists in the other direction. Nothing else about the desktop tree changes.

The two trees offer different sets of sections, Profile, Security, Audio and Video on desktop
against Appearance, Audio, Video, Security and Experimental on phones. That gap is real but is not
this unit's; it is recorded as a follow-up.

## Tests

- The settings page renders all five section headers, in order, each with the `id` its route segment
  uses.
- Visiting a section route renders the same page rather than a lone panel.
- The dialog carries no `sm:` or `max-sm:` prefix, asserted the way the existing token tests read a
  component's class strings.
- A phone-width visit to the desktop settings route redirects to `/settings`.

Each test is written before the change it covers and seen failing first.

## Verification on a device

- Playwright at 390 × 844, both themes, before and after: the settings page scrolls as one column,
  every section header is visible, and each sub-route lands on the right section.
- The same at 800px wide, to prove the dialog now uses the phone layout there.
- Android emulator Chrome against the dev server, installed to the home screen, to confirm the page
  scrolls under the bottom navigation and clears the safe area.

## Out of scope, tracked separately

- Dashboard alignment, the quick-join bar and filter chips. Checked against Android after this unit
  and reported.
- The differing section sets between the phone and desktop settings trees.
- `appScrollClass` and `appScrollYClass` in `settingsPanelTone.ts`, which nothing references.
- `SecuritySettingsPanel` taking no `tone` prop, compensated by a descendant-override class in the
  dialog.
