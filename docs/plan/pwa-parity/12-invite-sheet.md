# 12 — The invite sheet

Unit 4 of the [PWA parity plan](./01-overview-and-units.md). It turns the phone's full-screen
participants list into a bottom sheet that is both the roster and the invite, the way the Android
client has always had it.

**Today.** Tapping the participants icon on a phone opens `ParticipantsList` as a full-screen
overlay over the call: a 48px header, then one row per person carrying an avatar, a name, an
Admin/Mod/Guest badge and live mic, camera and deafen icons. The call disappears behind it. There is
no invite anything on the phone — the room link is reachable only through the options panel's
"Copy room link" row, which copies silently and offers no other way to hand the link to someone.

**Target.** The same icon opens a bottom sheet over the live call. The sheet leads with everyone in
the room as an avatar grid, and under it the share targets and the room link itself. The call keeps
rendering behind it.

After this unit the phone meeting has no full-screen surface left. That was the point of the whole
plan.

## The surface

`MeetingInviteSheet` is a [`BedrudSheet`](./08-meeting-chat-sheet.md), unit 2's bottom sheet, and
takes nothing from it but its children. Snap points, corner, handle, backdrop, bottom inset and
gutter are the sheet's; this unit does not restate them and must not override them.

| | |
|---|---|
| Snap points | `[0.5, 1]`, from `SHEET_SNAP_POINTS` |
| Opens at | half |
| Width | full, edge to edge |
| Applies below | 1024px — `useIsMobile()`, the plan's one breakpoint |
| Label | `Participants and invite` |

Android's equivalent is `MeetingInviteSheet.kt`, which is a `BedrudBottomSheet` for the same reason:
it is a sheet like every other sheet, and the shared scaffold owns its chrome.

## The entry point

The top-right participants icon, which already carries the count. It opens the sheet instead of the
full-screen list. There is no second way in.

Android has three entries — the top bar, the video grid's "+N" overflow tile, and an
"Invite a friend" row in the more options panel. The web takes only the first. The web's
`ParticipantGrid` has no overflow tile to hang the second on, and the third would need a row that is
phone-only or it leaks into the desktop `⋯` menu, the trap unit 3 recorded in `apps/web/AGENTS.md`.
One surface with one way in stays in sync with itself.

## The roster grid

Four columns. Each cell is a 44px circular avatar with the person's name under it, centred, one line,
ellipsised. The local participant's name reads `You`.

The grid is capped at **280px** and scrolls past that. The cap is the load-bearing part: without it a
busy room pushes the share targets and the room link off the bottom, and the half-height sheet the
user actually sees would carry a roster and nothing else. Android caps it at the same
`Dimens.inviteGridMaxHeight` for the same reason.

`ParticipantAvatar` renders the circle — it already resolves the avatar URL, falls back to initials
on a palette background, and carries `meet-avatar-circle`. The grid sizes it from
`--meet-invite-avatar-size`.

## The status on the avatar

Android shows only a speaking ring. The web's list shows more, and dropping it to match would be a
regression in the surface people open to check who is muted. The information stays; it moves onto
the avatar rather than needing a row of its own.

| Signal | Where it goes | Source |
|---|---|---|
| Speaking | a ring around the avatar | `--meet-speaking-ring`, the same token the tile uses |
| Mic off | a badge on the bottom-right corner of the avatar | `shouldShowMicMutedIndicator` |
| Deafened | the same corner badge, headphones glyph, taking precedence over mic off | `isParticipantDeafened` |
| Admin | a ring in the accent colour | `identity === adminId` |
| Moderator | the same ring, emerald | `metadata.accesses` includes `moderator` |

Guest is the absence of the other two and gets no ring — a badge on every cell that is not an admin
or a moderator is noise, and the list's `Guest` pill existed only because a row had the width to
spare.

Camera state is dropped. It is the one signal with no consequence for anyone else in the call, and
the tile behind the sheet already shows it.

## The invite block

Under the grid, separated by a divider: the share targets, then the room link.

### The share targets

A horizontally scrolling row of 56px circles, each with a 24px icon and a label beneath it —
Android's `InviteTarget`, which uses `Dimens.inviteTargetSize` and `Dimens.iconMd`.

| Target | Action |
|---|---|
| Share | `navigator.share({ url })` |
| Copy link | `navigator.clipboard.writeText` then the `Meeting link copied` toast |
| QR code | toggles the code above the row |

Android lists six, adding Email, Telegram and WhatsApp as explicit package intents. The web carries
three, because on a phone `navigator.share` opens the OS sheet that already lists every installed
app, and it hands the link to the installed Telegram rather than to `t.me` in a browser tab. An
explicit Telegram button would be a worse version of a target the platform already provides.

`navigator.share` is not everywhere. Where it is absent — a desktop browser at phone width, an
older engine — the Share target is replaced by three link targets so the row is never a dead end:

| Fallback target | Href |
|---|---|
| Email | `mailto:?body=<link>` |
| Telegram | `https://t.me/share/url?url=<link>` |
| WhatsApp | `https://wa.me/?text=<link>` |

Which targets are shown is the one decision here that is neither React's nor CSS's, so it is a pure
module: `inviteShareTargets(roomLink, canShare)` returns the list, and its test needs no DOM.

### The QR code

Tapping the QR target toggles a 240px code above the target row, matching Android's
`Dimens.inviteQrSize`. It renders on a white plate regardless of theme, because a scanner needs the
contrast and a dark-mode QR is a QR that does not scan.

QR is the only one of Android's six that does something `navigator.share` cannot: handing the room
to somebody standing next to you.

### The room link

The raw link in a monospace row, one line, ellipsised, with a copy glyph at the end. The whole row is
the button and copies. `window.location.href` is the link, which is what the options panel's
"Copy room link" row already copies — one definition of what the room's address is.

## What happens to `ParticipantsList`

It becomes desktop-only. The `lg:` sidebar at 288px is untouched; the mobile class block, the
`createPortal` to body and the `useFocusTrap` go, since all three exist to make a full-screen phone
overlay behave.

`MeetingPanels` loses `mobileChromeHidden = participantsOpen` with them. That constant exists because
a full-screen list covered the header band; a sheet stops short of it, so the top-right cluster stays
put and keeps showing the participants icon in its active state. Unit 2 made exactly this change for
chat and left the constant behind for the list, which is now the last caller.

## Where this diverges from Android

1. **Three share targets, not six.** `navigator.share` covers what the other three did, and covers
   it better. The three appear only as a fallback where it is missing.
2. **Status rides the avatar.** Android shows a speaking ring and nothing else, because its
   per-participant detail lives in `MeetingParticipantSheet`. The web has no such sheet, so the
   signals its list carried today stay, on the avatar rather than in a row.
3. **One entry point, not three.** The web grid has no overflow tile, and the options-panel row
   would cost a phone-only exclusion for a third door into one surface.

## What the phone surface loses

- **Camera state per participant.** Dropped deliberately; see above.
- **The `Guest` pill.** Guest is now the default reading of a cell with no ring.
- **A focus trap.** vaul's `modal` handles focus for the sheet, the way it does for chat.

## Dependency

One new package: **`qrcode.react@4.2.0`**. Zero runtime dependencies, declares React 19 in its peer
range, and renders `<QRCodeSVG>` as real SVG elements. The smaller `uqr` returns an SVG string, which
would need `dangerouslySetInnerHTML` in a repo that runs CodeQL on every push, and is at 0.1.3.

## Tokens

Four new, in `meeting.css` beside the rest of the meeting's token layer. Each is a size this unit
takes from Android, and a size taken from somewhere else is a decision, not a number — it belongs
where the other meeting sizes are and not inlined at a call site:

| Token | Value | Android source |
|---|---|---|
| `--meet-invite-grid-max-height` | `280px` | `Dimens.inviteGridMaxHeight` |
| `--meet-invite-avatar-size` | `44px` | `Dimens.avatarLg` |
| `--meet-invite-target-size` | `56px` | `Dimens.inviteTargetSize` |
| `--meet-invite-qr-size` | `240px` | `Dimens.inviteQrSize` |

## Files

| File | Responsibility |
|---|---|
| `components/meeting/MeetingInviteSheet.tsx` | the sheet, its snap state, the count header, the divider |
| `components/meeting/MeetingInviteGrid.tsx` | the four-column roster and the avatar's rings and badges |
| `components/meeting/MeetingInviteTargets.tsx` | the target row, the QR toggle, the room link row |
| `components/meeting/inviteShareTargets.ts` | pure — the target list for a link and a `canShare` flag |
| `components/meeting/ParticipantsList.tsx` | phone classes, portal and focus trap deleted |
| `components/meeting/MeetingPanels.tsx` | the icon opens the sheet; `mobileChromeHidden` goes |
| `components/meeting/meeting.css` | the two tokens |
| `package.json` | `qrcode.react` |

## Tests

House style: every test file is `.test.ts`, descriptions are imperative and start with `should`.

| File | Shape | Covers |
|---|---|---|
| `inviteShareTargets.test.ts` | pure | three targets with `canShare`, the three fallbacks without it, and that Share never appears alongside its own fallback |
| `meetingInviteSheet.test.ts` | source text, `// @vitest-environment node` | the sheet is a `BedrudSheet` and declares no snap points, corner or handle of its own; the grid carries the max-height token; `ParticipantsList` no longer contains the phone class block, the portal or the focus trap; `MeetingPanels` no longer contains `mobileChromeHidden` |

The second file's last two assertions are the ones worth having. Unit 3 shipped two defects that no
test caught because every test asserted what the new surface contained and none asserted what the old
one had stopped containing.

## Verification on a device

1. Android emulator Chrome against the dev server, installed to the home screen, launched standalone.
2. Two participants, so the grid has more than one cell and a speaking ring has something to track.
3. Tap the participants icon: the sheet opens at half with the call still rendering behind it.
4. Drag to full, drag down to half, drag down again to dismiss.
5. Mute the second participant and confirm the badge appears on their avatar without the sheet moving.
6. Tap Share and confirm the OS sheet lists installed apps. Tap Copy and confirm the toast.
7. Tap QR and scan the code with the second device; it should open the room.
8. At 1280px, confirm the left sidebar is unchanged and the sheet never appears.

## Out of scope

- **Per-participant moderation.** Android's `MeetingParticipantSheet` has no web counterpart in use;
  `ParticipantContextMenu.tsx` is 666 lines with no importer on any branch, `master` included. That
  is a separate question and a separate issue.
- **The desktop sidebar.** Untouched beyond the deletions above.
- **The grid's "+N" overflow tile.** The web `ParticipantGrid` has none, and adding one is a change
  to the video grid, not to this sheet.
- **Sharing anything but the link.** No invite text, no room name in the share payload.
