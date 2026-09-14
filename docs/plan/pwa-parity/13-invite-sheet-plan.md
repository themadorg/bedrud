# The invite sheet — implementation plan

> **For agentic workers:** use the subagent-driven-development or executing-plans skill to implement
> this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the phone's full-screen participants list with a bottom sheet that is both the
roster and the invite, so the phone meeting has no full-screen surface left.

**Architecture:** One pure module decides which share targets exist. Three components render the
sheet: `MeetingInviteSheet` composes, `MeetingInviteGrid` draws the roster, `MeetingInviteTargets`
draws the share row, the QR and the room link. The sheet is a `BedrudSheet` and adds no chrome of its
own. `MeetingPanels` picks the sheet on a phone and the existing `ParticipantsList` sidebar on
desktop; `ParticipantsList` loses everything that existed only to make a phone overlay behave.

**Tech stack:** React 19, TypeScript, TailwindCSS v4, vaul (through `BedrudSheet`), LiveKit
components, sonner, `qrcode.react@4.2.0`, Vitest 4, Bun, Biome.

**Spec:** [12 — The invite sheet](./12-invite-sheet.md). Read it before Task 1.

## Global constraints

- Verify command, from `apps/web`: `bun run check && bun run test`. `check` is `biome check . && tsc --noEmit`.
- Package manager is **bun**. `bun add`, never `npm install`. `bun.lock` is generated — commit it, never edit it.
- Test files are `.test.ts`, never `.test.tsx`. Descriptions are imperative and start with `should`.
- Commit subjects on this branch family read `<action> <what> for <why>` — `add`, `update`, `delete`,
  `merge`. No conventional-commit prefixes. No attribution of any kind.
- No abbreviated identifiers. `participant`, not `p`. `index`, not `i`.
- Comments sit **above** what they describe, never trailing. They explain why.
- Every size this unit takes from Android is a token in `meeting.css`, never an inline literal.
- The one breakpoint is `lg` / 1024px, through `useIsMobile()`. No `sm:` or `md:` anywhere.
- Branch is `feat/pwa-parity-invite-sheet`, already cut from `feat/pwa-parity-foundation`.
- **Do not commit without asking.** Each task's commit step is a stop: report, then wait for the word.

## Decisions this plan makes that the spec left open

Four of these change what the spec says. Read them before Task 1; if any is wrong, fix the plan and
the spec before writing code.

1. **`ParticipantsList` keeps its `createPortal`.** The spec says the portal goes with the phone
   overlay. It is wrong: the portal is load-bearing on **desktop** too. The sidebar is `lg:fixed` at
   `z-40` and the comment at `ParticipantsList.tsx:85` records why — it has to sit above the
   body-portalled stage WebXDC at `z-15`. Removing it would break desktop stacking, and no test would
   catch it. Only the focus trap and the phone classes go. Task 6 amends the spec.
2. **The desktop sidebar stops being a dialog.** `role="dialog"` and `aria-modal="true"` existed for
   the full-screen phone overlay. A persistent sidebar that does not trap focus must not claim to be
   a modal dialog — a screen reader would tell the user the rest of the page is inert when it is not.
   It becomes a plain labelled `<aside>`.
3. **A cell's ring shows one thing at a time: speaking outranks role.** The spec lists speaking,
   admin and moderator as three rings, and one avatar has one ring. Speaking is the transient signal
   and the one that answers "who is talking", so it replaces the role ring while it lasts.
4. **Copying the room link moves into a shared module.** `ControlsBar.copyRoomLink` and the sheet's
   copy target would otherwise be two copies of the same clipboard call, the same toast title and the
   same idea of what the room's address is. One `meetingLink.ts` owns both, and `ControlsBar` calls
   it. This is the §5 rule about a literal duplicated across files, applied to the only other caller.
5. **`ChatPanel`'s `participantsOpen` prop is deleted.** Its only use is
   `useFocusTrap({ enabled: (isOverlay || elevated) && !participantsOpen })` — chat yielding the trap
   to the full-screen list. Nothing traps focus after Task 5, so the prop is dead.
6. **`data-participants-overlay` is deleted.** `grep -rn data-participants-overlay src/` returns the
   one line that writes it and no selector that reads it.

## File structure

| File | Status | Responsibility |
|---|---|---|
| `src/components/meeting/inviteShareTargets.ts` | create | pure — which share targets exist for a link and a `canShare` flag |
| `src/components/meeting/inviteShareTargets.test.ts` | create | its test, no DOM |
| `src/components/meeting/meetingLink.ts` | create | the room's address, and the one copy-and-toast |
| `src/components/meeting/MeetingInviteGrid.tsx` | create | the four-column roster, its rings and its badges |
| `src/components/meeting/MeetingInviteTargets.tsx` | create | the target row, the QR toggle, the room-link row |
| `src/components/meeting/MeetingInviteSheet.tsx` | create | the sheet: count header, grid, divider, invite block |
| `src/components/meeting/meetingInviteSheet.test.ts` | create | source-text assertions, including what the old surface stopped containing |
| `src/components/meeting/meeting.css` | modify | four invite tokens |
| `src/components/meeting/MeetingPanels.tsx` | modify | the icon opens the sheet on a phone; `mobileChromeHidden` goes |
| `src/components/meeting/ParticipantsList.tsx` | modify | desktop-only: phone classes, focus trap, dialog role, dead marker |
| `src/components/meeting/ChatPanel.tsx` | modify | the dead `participantsOpen` prop |
| `src/components/meeting/ControlsBar.tsx` | modify | calls the shared copy |
| `src/components/meeting/meetingChrome.test.ts` | modify | the `mobileChromeHidden` assertions invert |
| `package.json`, `bun.lock` | modify | `qrcode.react` |
| `docs/plan/pwa-parity/01-overview-and-units.md` | modify | unit 4 status |
| `docs/plan/pwa-parity/12-invite-sheet.md` | modify | the portal amendment |

---

### Task 1: The share-target module

Pure logic, no DOM, no React. It is the only thing in this unit that decides which targets exist.

**Files:**
- Create: `apps/web/src/components/meeting/inviteShareTargets.ts`
- Test: `apps/web/src/components/meeting/inviteShareTargets.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `type InviteShareTargetId`, `interface InviteShareTarget { id; label; href? }`,
  `function inviteShareTargets(roomLink: string, canShare: boolean): InviteShareTarget[]`.
  Task 3 imports all three.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/components/meeting/inviteShareTargets.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { inviteShareTargets } from './inviteShareTargets'

const ROOM_LINK = 'https://bedrud.test/m/abc-def-ghi'

describe('inviteShareTargets', () => {
  it('should offer the system share sheet, copy and QR where sharing is supported', () => {
    expect(inviteShareTargets(ROOM_LINK, true).map((target) => target.id)).toEqual(['share', 'copy', 'qr'])
  })

  it('should replace sharing with the named services where it is unsupported', () => {
    expect(inviteShareTargets(ROOM_LINK, false).map((target) => target.id)).toEqual([
      'copy',
      'qr',
      'email',
      'telegram',
      'whatsapp',
    ])
  })

  it('should never offer sharing alongside the services that stand in for it', () => {
    const supported = inviteShareTargets(ROOM_LINK, true).map((target) => target.id)
    expect(supported).toContain('share')
    expect(supported).not.toContain('telegram')
  })

  it('should escape the room link into every fallback href', () => {
    const targets = inviteShareTargets('https://bedrud.test/m/a b&c', false)
    const hrefs = targets.filter((target) => target.href !== undefined).map((target) => target.href)
    expect(hrefs).toEqual([
      'mailto:?body=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
      'https://t.me/share/url?url=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
      'https://wa.me/?text=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
    ])
  })

  it('should give the targets the sheet handles itself no href', () => {
    const handled = inviteShareTargets(ROOM_LINK, true).filter((target) => target.href === undefined)
    expect(handled.map((target) => target.id)).toEqual(['share', 'copy', 'qr'])
  })
})
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/web && bun run test inviteShareTargets
```

Expected: fails to resolve `./inviteShareTargets`.

- [ ] **Step 3: Write the module**

Create `apps/web/src/components/meeting/inviteShareTargets.ts`:

```ts
/**
 * The share targets the invite sheet offers, and the only place that decides which ones exist.
 *
 * On a phone `navigator.share` opens the system sheet, which already lists every installed app and
 * hands the link to the installed Telegram rather than to a browser tab, so a named Telegram button
 * beside it would be the worse version of a target the platform already provides. Where sharing is
 * missing — a desktop browser at phone width, an older engine — the row would be a dead end, so the
 * named services take its place there and only there.
 */

/** Identifies a target to the sheet, which decides what activating it does. */
export type InviteShareTargetId = 'share' | 'copy' | 'qr' | 'email' | 'telegram' | 'whatsapp'

export interface InviteShareTarget {
  id: InviteShareTargetId
  label: string
  /** Set on the targets that are plain links. The sheet handles the others itself. */
  href?: string
}

/** Returns the targets for a room link, given whether this engine can open the system share sheet. */
export function inviteShareTargets(roomLink: string, canShare: boolean): InviteShareTarget[] {
  const encodedLink = encodeURIComponent(roomLink)
  const alwaysPresent: InviteShareTarget[] = [
    { id: 'copy', label: 'Copy link' },
    { id: 'qr', label: 'QR code' },
  ]

  if (canShare) {
    return [{ id: 'share', label: 'Share' }, ...alwaysPresent]
  }

  return [
    ...alwaysPresent,
    { id: 'email', label: 'Email', href: `mailto:?body=${encodedLink}` },
    { id: 'telegram', label: 'Telegram', href: `https://t.me/share/url?url=${encodedLink}` },
    { id: 'whatsapp', label: 'WhatsApp', href: `https://wa.me/?text=${encodedLink}` },
  ]
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd apps/web && bun run test inviteShareTargets
```

Expected: 5 passing.

- [ ] **Step 5: Verify, then stop and report**

```bash
cd apps/web && bun run check
```

Report the test output. **Do not commit without approval.** On approval:

```bash
git add apps/web/src/components/meeting/inviteShareTargets.ts apps/web/src/components/meeting/inviteShareTargets.test.ts
git commit -m "add the invite share targets module for one decision about which targets exist"
```

---

### Task 2: The room link and its copy

One module owns what the room's address is and what copying it says, so the sheet and the controls
bar cannot drift apart.

**Files:**
- Create: `apps/web/src/components/meeting/meetingLink.ts`
- Modify: `apps/web/src/components/meeting/ControlsBar.tsx:394-412`

**Interfaces:**
- Produces: `function meetingLink(): string`, `function copyMeetingLink(): Promise<boolean>`.
  Tasks 3 and 4 import both.

- [ ] **Step 1: Read the current copy handler**

Read `apps/web/src/components/meeting/ControlsBar.tsx` around line 394. It holds the clipboard call,
both toasts and a two-second `linkCopied` flag with a timer. The flag and the timer are the controls
bar's own affordance and stay there; the clipboard call and the toasts move.

- [ ] **Step 2: Write the module**

Create `apps/web/src/components/meeting/meetingLink.ts`:

```ts
import { toast } from 'sonner'

/**
 * The room's address is the page the participant is already on. Every affordance that hands the room
 * to somebody else — the controls bar's row, the invite sheet's target, the invite sheet's link row —
 * reads it from here, so there is one answer to what the room's address is.
 *
 * Returns an empty string during a server render, where there is no location to read.
 */
export function meetingLink(): string {
  if (typeof window === 'undefined') return ''
  return window.location.href
}

/** Copies the room link and reports the result, so every copy affordance says the same thing. */
export async function copyMeetingLink(): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(meetingLink())
    toast.success('Meeting link copied', {
      description: 'Share it so others can join this room.',
    })
    return true
  } catch {
    toast.error('Could not copy link', {
      description: 'Check clipboard permissions and try again.',
    })
    return false
  }
}
```

- [ ] **Step 3: Point the controls bar at it**

In `ControlsBar.tsx`, replace the body of `copyRoomLink` so it keeps its own flag and timer and
delegates the rest. Keep the surrounding `useCallback` and its dependency array as they are:

```tsx
  const copyRoomLink = useCallback(() => {
    void copyMeetingLink().then((copied) => {
      if (!copied) return
      setLinkCopied(true)
      if (linkCopiedTimerRef.current) clearTimeout(linkCopiedTimerRef.current)
      linkCopiedTimerRef.current = setTimeout(() => {
        setLinkCopied(false)
        linkCopiedTimerRef.current = null
      }, 2000)
    })
  }, [])
```

Add the import beside the other local imports:

```tsx
import { copyMeetingLink } from './meetingLink'
```

Then delete the now-unused `toast` import **only if** nothing else in `ControlsBar.tsx` still uses
it. Check first:

```bash
cd apps/web && grep -n "toast\." src/components/meeting/ControlsBar.tsx
```

- [ ] **Step 4: Verify**

```bash
cd apps/web && bun run check && bun run test
```

Expected: clean, and the whole suite still green — this step changes behaviour for no test, so a
failure here is a real regression.

- [ ] **Step 5: Stop and report**

**Do not commit without approval.** On approval:

```bash
git add apps/web/src/components/meeting/meetingLink.ts apps/web/src/components/meeting/ControlsBar.tsx
git commit -m "add a shared meeting link module for one definition of the room address"
```

---

### Task 3: The invite tokens and the roster grid

**Files:**
- Modify: `apps/web/src/components/meeting/meeting.css:44` (insert after the sheet-height block)
- Create: `apps/web/src/components/meeting/MeetingInviteGrid.tsx`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `function MeetingInviteGrid({ participants, adminId }: MeetingInviteGridProps)`, where
  `participants` is `ReturnType<typeof useParticipants>` and `adminId` is `string`. Task 5 renders it.

- [ ] **Step 1: Add the tokens**

In `apps/web/src/components/meeting/meeting.css`, immediately after the
`--meet-sheet-max-height` declaration on line 44, insert:

```css
  /*
   * The invite sheet's sizes, all four from Android's `Dimens`, so the two clients' invite surfaces
   * measure the same. The grid's ceiling is the load-bearing one: without it a busy room pushes the
   * share targets and the room link off the bottom, and the half-height sheet the user actually sees
   * carries a roster and nothing else.
   */
  --meet-invite-grid-max-height: 280px;
  --meet-invite-avatar-size: 44px;
  --meet-invite-target-size: 56px;
  --meet-invite-qr-size: 240px;
```

- [ ] **Step 2: Write the grid**

Create `apps/web/src/components/meeting/MeetingInviteGrid.tsx`:

```tsx
import { useIsSpeaking, useParticipants } from '@livekit/components-react'
import { MicOff } from 'lucide-react'
import { useMemo } from 'react'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { getPalette } from '#/lib/participant-palette'
import { shouldShowMicMutedIndicator } from '#/lib/push-to-talk-participant'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { cn } from '@/lib/utils'

type InviteParticipant = ReturnType<typeof useParticipants>[number]

interface ParticipantMeta {
  accesses?: string[]
}

/**
 * The badge that carries a cell's mic or deafen state. It sits on the avatar's corner on a plate in
 * the sheet's own colour, so it reads against a photo as well as against initials.
 */
const STATUS_BADGE_CLASS =
  'absolute -bottom-0.5 -end-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-[var(--meet-sidebar)] text-red-400'

/** The avatar is drawn by a child that clips itself to a circle, so the ring belongs on a wrapper. */
const AVATAR_SIZE_STYLE = {
  width: 'var(--meet-invite-avatar-size)',
  height: 'var(--meet-invite-avatar-size)',
}

function parseMeta(raw: string | undefined): ParticipantMeta {
  try {
    return JSON.parse(raw ?? '{}')
  } catch {
    return {}
  }
}

interface InviteGridCellProps {
  participant: InviteParticipant
  adminId: string
}

function InviteGridCell({ participant, adminId }: InviteGridCellProps) {
  const { isParticipantDeafened, getParticipantDisplayName, getParticipantAvatarUrl } = useMeetingRoomContext()
  const pushToTalkEnabled = useAudioPreferencesStore((state) => state.pushToTalkEnabled)
  const isSpeaking = useIsSpeaking(participant)

  const displayName = getParticipantDisplayName(participant)
  const palette = useMemo(() => getPalette(displayName), [displayName])
  const meta = useMemo(() => parseMeta(participant.metadata), [participant.metadata])

  const isRoomAdmin = participant.identity === adminId
  const isModerator = !isRoomAdmin && (meta.accesses ?? []).includes('moderator')
  const isDeafened = isParticipantDeafened(participant)
  const isMicMuted = shouldShowMicMutedIndicator(participant, pushToTalkEnabled)

  // One avatar has one ring. Speaking is the transient signal and the one that answers who is
  // talking, so while it lasts it replaces the ring the participant's role would otherwise draw.
  const ringClass = isSpeaking
    ? 'ring-2 ring-[var(--meet-speaking-ring)]'
    : isRoomAdmin
      ? 'ring-2 ring-[var(--accent-500)]'
      : isModerator
        ? 'ring-2 ring-emerald-500'
        : undefined

  return (
    <li className="flex min-w-0 flex-col items-center gap-1.5">
      <div className="relative">
        <div className={cn('rounded-full', ringClass)} style={AVATAR_SIZE_STYLE}>
          <ParticipantAvatar
            avatarUrl={getParticipantAvatarUrl(participant)}
            initials={displayName.charAt(0).toUpperCase()}
            paletteBackground={palette.avatar}
            className="h-full w-full text-sm"
          />
        </div>
        {isDeafened ? (
          <span className={STATUS_BADGE_CLASS}>
            <DeafenHeadphonesIcon size={10} off />
          </span>
        ) : isMicMuted ? (
          <span className={STATUS_BADGE_CLASS}>
            <MicOff size={10} />
          </span>
        ) : null}
      </div>
      <span className="w-full overflow-hidden text-ellipsis whitespace-nowrap text-center text-[11px] font-medium text-[var(--meet-fg-strong)]">
        {participant.isLocal ? 'You' : displayName}
      </span>
    </li>
  )
}

interface MeetingInviteGridProps {
  participants: InviteParticipant[]
  adminId: string
}

/**
 * Everyone in the room as a four-column avatar grid, capped and scrolling past the cap so the invite
 * block under it survives a busy room.
 */
export function MeetingInviteGrid({ participants, adminId }: MeetingInviteGridProps) {
  return (
    <ul
      className="meet-scroll grid shrink-0 grid-cols-4 gap-x-2 gap-y-3 overflow-y-auto"
      style={{ maxHeight: 'var(--meet-invite-grid-max-height)' }}
    >
      {participants.map((participant) => (
        <InviteGridCell key={participant.identity} participant={participant} adminId={adminId} />
      ))}
    </ul>
  )
}
```

- [ ] **Step 3: Verify it compiles and nothing regressed**

```bash
cd apps/web && bun run check && bun run test
```

Expected: clean. There is no test for the grid yet — Task 6 asserts the token, and the device pass
in Task 7 is what proves it draws.

- [ ] **Step 4: Stop and report**

**Do not commit without approval.** On approval:

```bash
git add apps/web/src/components/meeting/meeting.css apps/web/src/components/meeting/MeetingInviteGrid.tsx
git commit -m "add the invite roster grid and its tokens for the Android avatar grid on the web"
```

---

### Task 4: The share targets, the QR and the room link row

**Files:**
- Modify: `apps/web/package.json`, `apps/web/bun.lock` (through `bun add`)
- Create: `apps/web/src/components/meeting/MeetingInviteTargets.tsx`

**Interfaces:**
- Consumes: `inviteShareTargets`, `InviteShareTarget`, `InviteShareTargetId` (Task 1);
  `meetingLink`, `copyMeetingLink` (Task 2).
- Produces: `function MeetingInviteTargets({ roomLink }: { roomLink: string })`. Task 5 renders it.

- [ ] **Step 1: Add the dependency**

```bash
cd apps/web && bun add qrcode.react@4.2.0
```

Expected: `package.json` gains `"qrcode.react": "4.2.0"` and `bun.lock` updates. Do not edit either
by hand. If the exact version does not resolve, stop and report rather than picking another.

- [ ] **Step 2: Write the component**

Create `apps/web/src/components/meeting/MeetingInviteTargets.tsx`:

```tsx
import { Copy, Mail, MessageCircle, QrCode, Send, Share2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useMemo, useState } from 'react'
import type { InviteShareTarget, InviteShareTargetId } from './inviteShareTargets'
import { inviteShareTargets } from './inviteShareTargets'
import { copyMeetingLink } from './meetingLink'

/**
 * The QR's own coordinate resolution, which is not a size on screen: the code is stretched to the
 * box `--meet-invite-qr-size` gives it. A power of two keeps the modules on whole pixels at the
 * common display scales.
 */
const QR_RESOLUTION = 256

const TARGET_ICONS: Record<InviteShareTargetId, typeof Share2> = {
  share: Share2,
  copy: Copy,
  qr: QrCode,
  email: Mail,
  telegram: Send,
  whatsapp: MessageCircle,
}

const TARGET_CIRCLE_CLASS =
  'flex items-center justify-center rounded-full bg-[var(--meet-control)] text-[var(--meet-fg-strong)] transition-[background] duration-150 hover:bg-[var(--meet-control-hover)]'

const TARGET_LABEL_CLASS = 'w-16 overflow-hidden text-ellipsis whitespace-nowrap text-center text-[11px] text-[var(--meet-fg-muted)]'

const TARGET_SIZE_STYLE = {
  width: 'var(--meet-invite-target-size)',
  height: 'var(--meet-invite-target-size)',
}

interface MeetingInviteTargetsProps {
  roomLink: string
}

/** The share row, the QR it toggles above itself, and the room link in full underneath. */
export function MeetingInviteTargets({ roomLink }: MeetingInviteTargetsProps) {
  const [canShare, setCanShare] = useState(false)
  const [qrVisible, setQrVisible] = useState(false)

  // `navigator.share` cannot be read during the server render, and reading it in the first client
  // render would make the markup disagree with the server's. It is read once the sheet is mounted.
  useEffect(() => {
    setCanShare(typeof navigator !== 'undefined' && typeof navigator.share === 'function')
  }, [])

  const targets = useMemo(() => inviteShareTargets(roomLink, canShare), [roomLink, canShare])

  const activateTarget = (target: InviteShareTarget) => {
    if (target.id === 'share') {
      // A share the participant dismisses rejects, and a dismissal is not an error worth reporting.
      void navigator.share({ url: roomLink }).catch(() => {})
      return
    }
    if (target.id === 'copy') {
      void copyMeetingLink()
      return
    }
    if (target.id === 'qr') {
      setQrVisible((visible) => !visible)
    }
  }

  return (
    <div className="flex shrink-0 flex-col gap-3">
      {qrVisible && (
        <div className="flex justify-center">
          {/*
            The plate is white in both themes. A scanner needs the contrast, and a QR drawn in the
            dark theme's colours is a QR that does not scan.
          */}
          <div className="rounded-2xl bg-white p-3" style={{ width: 'var(--meet-invite-qr-size)' }}>
            <QRCodeSVG value={roomLink} size={QR_RESOLUTION} className="h-auto w-full" />
          </div>
        </div>
      )}

      <div className="meet-scroll flex gap-4 overflow-x-auto pb-1">
        {targets.map((target) => {
          const Icon = TARGET_ICONS[target.id]
          const icon = <Icon size={24} />

          if (target.href !== undefined) {
            return (
              <a
                key={target.id}
                href={target.href}
                target="_blank"
                rel="noreferrer"
                className="flex shrink-0 flex-col items-center gap-1.5"
              >
                <span className={TARGET_CIRCLE_CLASS} style={TARGET_SIZE_STYLE}>
                  {icon}
                </span>
                <span className={TARGET_LABEL_CLASS}>{target.label}</span>
              </a>
            )
          }

          return (
            <button
              key={target.id}
              type="button"
              onClick={() => activateTarget(target)}
              className="flex shrink-0 cursor-pointer flex-col items-center gap-1.5 border-none bg-transparent p-0"
              aria-pressed={target.id === 'qr' ? qrVisible : undefined}
            >
              <span className={TARGET_CIRCLE_CLASS} style={TARGET_SIZE_STYLE}>
                {icon}
              </span>
              <span className={TARGET_LABEL_CLASS}>{target.label}</span>
            </button>
          )
        })}
      </div>

      <button
        type="button"
        onClick={() => void copyMeetingLink()}
        className="flex w-full cursor-pointer items-center gap-2 rounded-xl border border-[var(--meet-border-subtle)] bg-[var(--meet-control)] px-3 py-2.5 text-start"
        aria-label="Copy room link"
      >
        <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-[var(--meet-fg-muted)]">
          {roomLink}
        </span>
        <Copy size={14} className="shrink-0 text-[var(--meet-fg-muted)]" />
      </button>
    </div>
  )
}
```

- [ ] **Step 3: Verify**

```bash
cd apps/web && bun run check && bun run test
```

Expected: clean. If `tsc` rejects `QRCodeSVG`'s `className`, stop and report — do not reach for
`dangerouslySetInnerHTML`, which is the reason the spec chose this package over `uqr`.

- [ ] **Step 4: Stop and report**

**Do not commit without approval.** On approval, two commits so the dependency is bisectable on its
own:

```bash
git add apps/web/package.json apps/web/bun.lock
git commit -m "add qrcode.react for the invite sheet QR code"
git add apps/web/src/components/meeting/MeetingInviteTargets.tsx
git commit -m "add the invite share targets row for handing the room link to somebody else"
```

---

### Task 5: The sheet, and the phone entry point

**Files:**
- Create: `apps/web/src/components/meeting/MeetingInviteSheet.tsx`
- Modify: `apps/web/src/components/meeting/MeetingPanels.tsx:140`

**Interfaces:**
- Consumes: `MeetingInviteGrid` (Task 3), `MeetingInviteTargets` (Task 4), `meetingLink` (Task 2),
  `BedrudSheet` and `SHEET_SNAP_POINTS` from `@/components/ui/BedrudSheet`.
- Produces: `function MeetingInviteSheet({ open, onClose, adminId }: MeetingInviteSheetProps)`.

- [ ] **Step 1: Re-read the sheet contract**

Read `apps/web/src/components/ui/BedrudSheet.tsx`. Note what it already owns: snap points, the
corner, the handle, the backdrop, the height, the bottom inset and the `px-4` gutter. The sheet in
this task restates none of them. `ChatPanel.tsx:197-208` is the one existing caller — match it.

- [ ] **Step 2: Write the sheet**

Create `apps/web/src/components/meeting/MeetingInviteSheet.tsx`:

```tsx
import { useParticipants } from '@livekit/components-react'
import { Users } from 'lucide-react'
import { useState } from 'react'
import { BedrudSheet, SHEET_SNAP_POINTS } from '@/components/ui/BedrudSheet'
import { MeetingInviteGrid } from './MeetingInviteGrid'
import { MeetingInviteTargets } from './MeetingInviteTargets'
import { meetingLink } from './meetingLink'

interface MeetingInviteSheetProps {
  open: boolean
  onClose: () => void
  adminId: string
}

/**
 * The phone's roster and invite in one bottom sheet over the live call, mirroring Android's
 * `MeetingInviteSheet`. It is a `BedrudSheet` and takes nothing from it but its children: the snap
 * points, corner, handle, backdrop and gutter are the sheet's, and restating any of them here would
 * be a second opinion about what a sheet looks like.
 */
export function MeetingInviteSheet({ open, onClose, adminId }: MeetingInviteSheetProps) {
  const participants = useParticipants()
  const [snapPoint, setSnapPoint] = useState<number | string | null>(SHEET_SNAP_POINTS[0])

  return (
    <BedrudSheet
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose()
      }}
      label="Participants and invite"
      activeSnapPoint={snapPoint}
      onSnapPointChange={setSnapPoint}
      dataMarkers={{ 'data-invite-sheet': 'true' }}
    >
      <div className="flex min-h-0 flex-col gap-4 pb-4">
        <div className="flex shrink-0 items-center gap-[7px]">
          <Users size={14} className="text-[var(--meet-btn-muted-fg)]" />
          <span className="text-[13px] font-semibold text-[var(--meet-fg-strong)]">Participants</span>
          <span className="rounded-md border border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] px-[6px] py-px text-[11px] font-semibold text-[var(--meet-btn-muted-fg)]">
            {participants.length}
          </span>
        </div>

        <MeetingInviteGrid participants={participants} adminId={adminId} />

        <div className="h-px shrink-0 bg-[var(--meet-border-subtle)]" />

        <MeetingInviteTargets roomLink={meetingLink()} />
      </div>
    </BedrudSheet>
  )
}
```

- [ ] **Step 3: Open it from the participants icon**

In `MeetingPanels.tsx`, add the two imports beside the existing ones:

```tsx
import { MeetingInviteSheet } from '@/components/meeting/MeetingInviteSheet'
import { useIsMobile } from '@/lib/use-is-mobile'
```

Add the hook beside the other top-level hooks in `MeetingPanels`, under the `useMeetingStage()` call:

```tsx
  const isMobile = useIsMobile()
```

Replace line 140, which today reads:

```tsx
      {participantsOpen && !infoOpen && <ParticipantsList adminId={adminId} onClose={onCloseParticipants} />}
```

with:

```tsx
      {/*
        One surface per width. The phone gets the sheet over the live call; desktop keeps the sidebar,
        which is not a sheet and never was. A CSS-only split will not do here — the sheet is a modal
        portal that locks the page behind it, so it must not mount at all on desktop.
      */}
      {participantsOpen &&
        !infoOpen &&
        (isMobile ? (
          <MeetingInviteSheet open onClose={onCloseParticipants} adminId={adminId} />
        ) : (
          <ParticipantsList adminId={adminId} onClose={onCloseParticipants} />
        ))}
```

- [ ] **Step 4: Verify**

```bash
cd apps/web && bun run check && bun run test
```

Expected: `check` clean. `meetingChrome.test.ts` still passes — `mobileChromeHidden` has not moved
yet; Task 6 is where it goes.

- [ ] **Step 5: Stop and report**

**Do not commit without approval.** On approval:

```bash
git add apps/web/src/components/meeting/MeetingInviteSheet.tsx apps/web/src/components/meeting/MeetingPanels.tsx
git commit -m "add the invite sheet behind the phone participants icon for a roster over the live call"
```

---

### Task 6: Take the phone surface off `ParticipantsList`

This is the task that removes things, and the one unit 3's defects argue for. Every assertion here
is about what a file has **stopped** containing.

**Files:**
- Test: `apps/web/src/components/meeting/meetingInviteSheet.test.ts` (create)
- Test: `apps/web/src/components/meeting/meetingChrome.test.ts` (modify)
- Modify: `apps/web/src/components/meeting/ParticipantsList.tsx`
- Modify: `apps/web/src/components/meeting/MeetingPanels.tsx`
- Modify: `apps/web/src/components/meeting/ChatPanel.tsx`

- [ ] **Step 1: Write the failing tests**

Create `apps/web/src/components/meeting/meetingInviteSheet.test.ts`:

```ts
// @vitest-environment node
//
// This file reads component sources from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const sheetSource = readFileSync(new URL('./MeetingInviteSheet.tsx', import.meta.url), 'utf8')
const gridSource = readFileSync(new URL('./MeetingInviteGrid.tsx', import.meta.url), 'utf8')
const targetsSource = readFileSync(new URL('./MeetingInviteTargets.tsx', import.meta.url), 'utf8')
const listSource = readFileSync(new URL('./ParticipantsList.tsx', import.meta.url), 'utf8')
const panelsSource = readFileSync(new URL('./MeetingPanels.tsx', import.meta.url), 'utf8')

describe('MeetingInviteSheet', () => {
  it('should be the app sheet rather than a second one', () => {
    expect(sheetSource).toContain('<BedrudSheet')
  })

  it('should leave the sheet chrome to the sheet', () => {
    expect(sheetSource).not.toContain('snapPoints=')
    expect(sheetSource).not.toMatch(/\brounded-t-/)
    expect(sheetSource).not.toContain('BedrudSheetHandle')
    expect(sheetSource).not.toContain('--meet-sheet-max-height')
  })

  it('should not introduce a breakpoint other than the shared one', () => {
    expect(sheetSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
    expect(gridSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
    expect(targetsSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})

describe('the invite sizes', () => {
  it('should come from the meeting tokens rather than from the call sites', () => {
    expect(gridSource).toContain('var(--meet-invite-grid-max-height)')
    expect(gridSource).toContain('var(--meet-invite-avatar-size)')
    expect(targetsSource).toContain('var(--meet-invite-target-size)')
    expect(targetsSource).toContain('var(--meet-invite-qr-size)')
  })
})

describe('ParticipantsList after the sheet', () => {
  it('should no longer trap focus, because it is no longer a modal surface', () => {
    expect(listSource).not.toContain('useFocusTrap')
    expect(listSource).not.toContain('aria-modal')
  })

  it('should no longer carry the full-screen phone sizing', () => {
    expect(listSource).not.toContain('var(--app-height, 100svh)')
    expect(listSource).not.toContain('var(--app-width, 100svw)')
  })

  it('should keep the body portal, which is what puts the desktop sidebar above the stage', () => {
    // The portal was never about the phone overlay: the sidebar is `lg:fixed` at z-40 and has to sit
    // above the body-portalled stage WebXDC at z-15.
    expect(listSource).toContain('createPortal')
  })

  it('should drop the marker attribute nothing selects on', () => {
    expect(listSource).not.toContain('data-participants-overlay')
  })
})

describe('the phone meeting after the sheet', () => {
  it('should no longer hide the top-right cluster for a full-screen list', () => {
    expect(panelsSource).not.toContain('mobileChromeHidden')
  })

  it('should still hide the controls bar beneath an open sheet', () => {
    expect(panelsSource).toContain('const mobileControlsHidden = chatOpen || participantsOpen')
  })

  it('should stop telling chat to yield its focus trap to the list', () => {
    const chatSource = readFileSync(new URL('./ChatPanel.tsx', import.meta.url), 'utf8')
    expect(chatSource).not.toContain('participantsOpen')
    expect(panelsSource).not.toContain('participantsOpen={participantsOpen}')
  })
})
```

- [ ] **Step 2: Invert the two chrome assertions**

In `meetingChrome.test.ts`, delete these two tests, which now assert the opposite of what is true —
the replacements live in `meetingInviteSheet.test.ts`:

```ts
  it('should keep the top-right cluster visible while chat is open', () => { ... })

  it('should hide the top-right cluster under the full-screen participants list', () => { ... })
```

Then widen the file's `MOBILE_CLUSTER_CLASS` constant. It currently carries the single quotes of the
`cn(...)` argument, and Step 5 turns that argument into a double-quoted JSX string, so the constant
would stop matching and the test would pass its `toBeDefined` on nothing:

```ts
/** The top-right cluster on phones, which carries the participants and chat toggles. */
const MOBILE_CLUSTER_CLASS = 'absolute z-[25] flex h-9 items-center gap-2 lg:hidden'
```

Replace the two deleted tests with one, keeping the other two in the file untouched:

```ts
  it('should keep the top-right cluster visible, since no surface covers it any more', () => {
    // Chat and the participants list are both sheets now. Both stop short of the header band, so the
    // cluster stays put and the participants icon keeps showing its active state.
    const clusterLine = meetingPanelsSource.split('\n').find((line) => line.includes(MOBILE_CLUSTER_CLASS))
    expect(clusterLine).toBeDefined()
    expect(clusterLine).not.toContain('mobileChromeHidden')
  })
```

- [ ] **Step 3: Run both files and watch them fail**

```bash
cd apps/web && bun run test meetingInviteSheet meetingChrome
```

Expected: the `ParticipantsList` and phone-chrome assertions fail. The `MeetingInviteSheet` and
token assertions should already pass, since Tasks 3 to 5 built them.

- [ ] **Step 4: Strip `ParticipantsList` to the desktop sidebar**

In `ParticipantsList.tsx`:

- Delete the `useFocusTrap` import and the `const trapRef = useFocusTrap(...)` line.
- On the `<aside>`: delete `ref={trapRef}`, `role="dialog"`, `aria-modal="true"` and
  `data-participants-overlay="true"`. Keep `aria-label="Participants"` and the `zIndex` style.
- Delete the two phone class lines — the `fixed left-[var(--app-offset-left,0px)] …` line and the
  `pt-[env(safe-area-inset-top,0px)] …` line — and the `// Mobile: full-screen …` comment above them.
- Since only desktop classes remain, drop the now-redundant `lg:` prefixes on the sidebar's own
  positioning so the block reads as one sidebar rather than an override of a layout that is gone.
  The class list becomes:

```tsx
      className={cn(
        'z-40 flex flex-col bg-[var(--meet-sidebar)] shadow-2xl backdrop-blur-2xl',
        // Fixed and body-portalled so the sidebar stacks above the stage WebXDC at z-15.
        'fixed inset-y-0 start-0 h-full w-[min(288px,var(--app-width,100svw))] border-e border-[var(--meet-border-subtle)]',
        'pb-[calc(88px+env(safe-area-inset-bottom,0px))]',
      )}
```

- Leave the header's and rows' `lg:` sizing prefixes alone. They are the desktop half of a pair whose
  phone half is only unreachable, not wrong, and rewriting them is a separate change.

- [ ] **Step 5: Drop `mobileChromeHidden` and the chat prop**

In `MeetingPanels.tsx`:

- Delete the `const mobileChromeHidden = participantsOpen` line and the first two sentences of the
  comment above it, leaving the sentence that explains `mobileControlsHidden`.
- In the mobile cluster's `className`, drop the condition:

```tsx
        className="absolute z-[25] flex h-9 items-center gap-2 lg:hidden"
```

  Note this turns the `cn(...)` call into a bare string, so remove the `cn` import if nothing else in
  the file uses it. Check: `grep -n "cn(" src/components/meeting/MeetingPanels.tsx`.

- Delete `participantsOpen={participantsOpen}` from the `<ChatPanel>` props.

In `ChatPanel.tsx`:

- Delete `participantsOpen?: boolean` from `Props`, `participantsOpen = false,` from the destructured
  parameters, and simplify the trap to `useFocusTrap({ enabled: isOverlay || elevated, onClose: handleClose })`.

- [ ] **Step 6: Run the tests and watch them pass**

```bash
cd apps/web && bun run test meetingInviteSheet meetingChrome
```

Expected: all green.

- [ ] **Step 7: Verify the whole suite**

```bash
cd apps/web && bun run check && bun run test
```

Expected: clean. A failure elsewhere means something still depended on the phone overlay — report it
rather than deleting the assertion.

- [ ] **Step 8: Stop and report**

**Do not commit without approval.** On approval, two commits, the test first so the removal is
bisectable against it:

```bash
git add apps/web/src/components/meeting/meetingInviteSheet.test.ts apps/web/src/components/meeting/meetingChrome.test.ts
git commit -m "add tests for what the phone participants surface stopped containing"
git add apps/web/src/components/meeting/ParticipantsList.tsx apps/web/src/components/meeting/MeetingPanels.tsx apps/web/src/components/meeting/ChatPanel.tsx
git commit -m "delete the full-screen phone participants overlay for one surface per width"
```

---

### Task 7: Verify on a device, then the docs

Nothing here is optional. A green suite is not verification, and every assertion in Task 6 is a
source-text read — not one of them proves the sheet draws.

**Files:**
- Modify: `docs/plan/pwa-parity/01-overview-and-units.md`
- Modify: `docs/plan/pwa-parity/12-invite-sheet.md`

- [ ] **Step 1: Bring the stack up**

Three servers, from `.claude/launch.json`: `livekit` (7072), `api` (7071), `web-dev` (7070). The API
binary is built with `go build -o ./tmp/server ./cmd/server/main.go` from `server/`. A guest cannot
create a room, so the room has to come from a signed-in session; reuse an existing room ID rather
than trying to create one.

- [ ] **Step 2: Walk the spec's device list**

Run every numbered step in [12 — Verification on a device](./12-invite-sheet.md#verification-on-a-device),
with two participants, and record what was observed for each — not "passed".

- [ ] **Step 3: Capture the after screenshots**

Phone viewport 375×812, dark and light, the same room and the same participant count as the baseline
in the scratchpad, sheet open at half. Read both images before attaching them anywhere and check the
frame for names, hostnames and room IDs.

- [ ] **Step 4: Amend the spec**

In `12-invite-sheet.md`, under "What happens to `ParticipantsList`", correct the claim that the portal
goes. Replace the sentence listing the three removals with one that lists the focus trap and the
phone classes, and says the body portal stays because the desktop sidebar stacks above the stage
through it.

Under "What the phone surface loses", leave the focus-trap line — that one is right.

- [ ] **Step 5: Mark the unit done**

In `01-overview-and-units.md`, change the unit 4 row's status to:

```
| 4 | Invite sheet | participants and invite as one sheet: avatar grid, count, share targets, room link | done, [12](./12-invite-sheet.md), [13](./13-invite-sheet-plan.md) |
```

And update the header sentence, which still says units 2 to 5 get their own file "when they start".

- [ ] **Step 6: Stop and report**

**Do not commit without approval.** On approval:

```bash
git add docs/plan/pwa-parity/01-overview-and-units.md docs/plan/pwa-parity/12-invite-sheet.md
git commit -m "update the parity docs for the shipped invite sheet"
```

---

## Definition of done

Print this with a real pass or fail per line before calling the unit done:

- [ ] Recommended first; the four spec-changing decisions above were approved, not assumed
- [ ] Test written first, seen failing, then passing — Tasks 1 and 6
- [ ] `bun run check && bun run test` passed from `apps/web`
- [ ] Verified by running it: every step of the spec's device list, with what was observed
- [ ] Docs updated in the same change — Task 7
- [ ] Touched files formatted; the format hook ran clean
- [ ] No inline magic values; the four sizes are tokens
- [ ] Commits split, bisectable, no attribution
- [ ] Leftovers filed as tracked issues

## Leftovers, verified against the code and filed

- [#167](https://github.com/themadorg/bedrud/issues/167) — `ParticipantContextMenu.tsx`, 666 lines
  with no importer on this branch or on `master`. Confirmed with
  `git grep -n ParticipantContextMenu master -- apps/web/src`, which matches only the file itself.
- [#168](https://github.com/themadorg/bedrud/issues/168) — `ParticipantsList`'s header, close button
  and close icon still carry `lg:` overrides whose unprefixed halves can no longer render.
- Issues [#162](https://github.com/themadorg/bedrud/issues/162) and
  [#163](https://github.com/themadorg/bedrud/issues/163) — the duplicated handle and height token —
  are unblocked by the merged base and are not this unit's work.
