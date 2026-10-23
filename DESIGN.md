# Bedrud Design System

## Aesthetic — Rose + Teal (Rounded)

Rounded corners on the Android client's scale. Bold colors. No purple.

- **Primary** — rose: brand CTAs, links, focus rings
- **Accent** — teal: highlights, badges, secondary actions

Accessibility-first. Every status color pairs with an icon, label, or pattern — never color alone.

## Brand Tokens (Rose)

| Token | Hex | Use |
|-------|-----|-----|
| `--primary-50` | `#FFF1F2` | Light wash, hover bg |
| `--primary-100` | `#FFE4E6` | Subtle fills, chips |
| `--primary-200` | `#FECDD3` | Borders, dividers |
| `--primary-300` | `#FDA4AF` | Muted accents |
| `--primary-400` | `#FB7185` | Links (dark mode) |
| `--primary-500` | `#F43F5E` | Focus rings, info |
| `--primary-600` | `#E11D48` | Primary CTA (6.1:1 on white) |
| `--primary-700` | `#BE123C` | CTA hover, headings |
| `--primary-800` | `#9F1239` | Deep rose |
| `--primary-900` | `#881337` | Darkest rose |

## Accent Tokens (Teal)

| Token | Hex | Use |
|-------|-----|-----|
| `--accent-50` | `#F0FDFA` | Highlight wash |
| `--accent-100` | `#CCFBF1` | Chip bg, selection |
| `--accent-200` | `#99F6E4` | Badges |
| `--accent-300` | `#5EEAD4` | Raised hand bg |
| `--accent-400` | `#2DD4BF` | Accent borders |
| `--accent-500` | `#14B8A6` | Accent (always paired with ink text) |
| `--accent-600` | `#0D9488` | Accent text on light |
| `--accent-700` | `#0F766E` | Accent hover |
| `--accent-800` | `#115E59` | Deep teal |
| `--accent-900` | `#134E4A` | Darkest teal |

## Status Tokens

| Token | Hex | Use | Required pairing |
|-------|-----|-----|-----------------|
| `--success-500` | `#16A34A` | Connected, speaking | Check icon or audio-bar |
| `--warning-500` | `#B45309` light / `#FBBF24` dark | Non-critical cautions, a banned or restricted status | Warning icon + label |
| `--destructive` | `#B91C1C` light / `#EF4444` dark | Errors, leave call, delete (irreversible only) | Words, optionally an icon |
| `--recording` | `#B91C1C` light / `#EF4444` dark | The dot that says a room is being recorded | "Recording" label |

**The error red never speaks alone.** Rose and red share a hue family, so red on screen does not say
by itself whether something is selected or something failed. The error red sits a step darker than
the ramp's middle in light theme and a step lighter in dark theme to keep its distance from the rose
primary, but distance is only half of it: an error always comes with words — the message itself, or a
field's error text — and may add an error icon beside them. A destructive control carries its own
label or icon. Recording is red by convention, not because anything failed, so it has its own token
and stays put if the error red moves again. All three values match the Android client (`Theme.kt`,
`ExtendedColors.kt`).

## Containers & Inverse Surface

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| `--accent-container` / `-foreground` | `#CCFBF1` / `#134E4A` | `#115E59` / `#CCFBF1` | Badge beside a name, a live or active status pill |
| `--inverse-surface` / `--inverse-foreground` | `#1C1917` / `#F5F0EE` | `#F5F5F4` / `#1C1917` | Toasts |
| `--inverse-success` / `-warning` / `-error` | `#5EEAD4` / `#FBBF24` / `#EF4444` | `#0D9488` / `#B45309` / `#B91C1C` | A toast's status icon |

The inverse status colours are the other theme's tone of each role, as Material's `inversePrimary`
is: the theme's own red sits too close to the inverse surface to read. Each clears 3:1 against it.

## Foreground & Chrome

| Token | Hex | Use |
|-------|-----|-----|
| `--fg-1` | `#1C1917` | Body text (14.5:1 on white) |
| `--fg-2` | `#57534E` | Muted/secondary text |
| `--fg-3` | `#A8A29E` | Disabled, placeholders |
| `--bg` | `#FFFBF9` | Page background (warm white) |
| `--bg-alt` | `#FFF1F2` | Cards, alt sections |
| `--line` | `#E7E5E4` | Borders, dividers |

## Dark Mode Overrides

| Token | Dark value |
|-------|-----------|
| `--bg` | `#0C0A09` |
| `--bg-alt` | `#1C1917` |
| `--fg-1` | `#FAFAF9` |
| `--fg-2` | `#A8A29E` |
| `--line` | `#292524` |
| `--primary-500` | `#FB7185` (lifted for AA on dark) |
| `--primary-600` | `#F43F5E` |

## Semantic Mapping

| UI element | Token | Style |
|-----------|-------|-------|
| CTA / primary button | `--primary` (`--primary-600`) | bg: primary, text: white, hover: primary-hover |
| Link / inline action | `--primary-500` | text: primary-500, underline on hover |
| "You" tile in call | `--primary-500` | 3px ring + "YOU" label badge |
| Active speaker | `--success-500` | 3px ring + audio-bar icon |
| Raised hand | `--accent-500` | Circle with hand icon, ink border |
| End / leave call | `--destructive` | bg: destructive, white icon |
| Connected / OK | `--success-500` | Dot + check icon + label |
| Warning | `--warning-500` | Warning icon + label |
| Banned / restricted status | `--warning-500` | Label; red is for real errors only |
| Info | `--primary-500` | Info icon |

## Accessibility Rules (Non-Negotiable)

1. **Color is never the only signal.** Every status must also have an icon, label, ring, or pattern.
2. **Body text** on `--bg` uses `--fg-1` (14.5:1). Muted text uses `--fg-2` (4.7:1).
3. **Primary CTA** is `--primary-600` (6.1:1 on white). Hover goes to `--primary-700`.
4. **Destructive** (`--destructive`) is reserved for errors and for leave call, delete and irreversible actions. Never for emphasis, never for a status, and never without words.
5. **Accent teal** (`--accent-500`) ALWAYS pairs with ink text (`--fg-1`) — never white.
6. **Focus ring**: 3px `--primary-500` at 45% opacity on all interactive elements.

### Verification Checklist

- [ ] All text/bg combos pass WCAG AA
- [ ] Disable color in DevTools (grayscale) — every UI state still readable
- [ ] Test under protanopia + deuteranopia (Chrome DevTools → Rendering → Emulate vision)
- [ ] No raw hex literals outside `theme.css`

## Border Radius

The web shares the Android client's shape scale (`apps/android`, `Shape.kt`), applied through
the Tailwind radius tokens in `styles.css`. Use the class, never an arbitrary `rounded-[Npx]`.

| Web class | px | Android token | Used for |
|---|---|---|---|
| `rounded` | 4 | `xs` | brand mark, checkbox square, tooltip |
| `rounded-sm` | 8 | `sm` | filter chips, icon-button hover fills, swatches |
| `rounded-md` | 12 | `md` | buttons |
| `rounded-lg` | 12 | `md` | fields, inline banners, row containers |
| `rounded-xl` | 16 | `lg` | cards, menus, toasts |
| `rounded-2xl` | 20 | `xl` | |
| `rounded-3xl` | 28 | `xxl` | sheet top, video tile, controls bar |
| `rounded-full` | pill | `full` | avatars, badges, pills, FAB |

`--radius` in `theme.css` is the default corner (12px) that self-hosters may retune: `rounded-md`
and `rounded-lg` both resolve to it, so retuning the one value moves every button and field. The
rest of the scale lives in `styles.css`. `src/design-tokens.test.ts` pins both. The shadcn primitives in
`apps/web/src/components/ui/` carry their corner class themselves (button `rounded-md`, input
`rounded-lg`, card `rounded-xl`, alert `rounded-lg`, badge `rounded-full`, dialog `rounded-3xl`,
menus `rounded-xl`), so a page never has to add one. A badge is a pill on the teal accent container,
the Android client's `BedrudBadge`; a filter chip takes the badge's `shape="chip"` for the 8px chip
corner, as Android's `FilterChip` does. Menus (`DropdownMenu`, `ContextMenu`) drop open on the
raised popover surface with one shadow, `shadow-lg`. Alert is the inline banner and shares the
banner corner with the hand-rolled ones beside it. Each primitive carries exactly one corner per
rendered element — `cn` runs the string through tailwind-merge, which keeps the last radius and
drops the rest.

## Token Architecture

The palette is defined in `src/theme.css` — a single file self-hosters can edit or swap.

### Semantic Tokens → Tailwind Classes

| CSS variable | Tailwind class | Usage |
|-------------|---------------|-------|
| `--primary` | `bg-primary`, `text-primary` | Buttons, links, active states |
| `--foreground` | `text-foreground` | Body text |
| `--background` | `bg-background` | Page bg |
| `--muted-foreground` | `text-muted-foreground` | Secondary text |
| `--destructive` | `bg-destructive`, `text-destructive` | Error/delete states |
| `--warning-500` | `bg-warning`, `text-warning` | Cautions, banned status |
| `--success-500` | `bg-success`, `text-success` | Connected, live |
| `--recording` | `bg-recording` | Recording dot |
| `--accent-container` | `bg-accent-container`, `text-accent-container-foreground` | Badges, active pills |
| `--inverse-surface` | `bg-inverse-surface`, `text-inverse-foreground` | Toasts |
| `--border` | `border-border` | Borders |
| `--ring` | `ring-ring` | Focus rings |

### Color Scale Utilities

The full rose and teal scales are available as Tailwind utilities:

| Prefix | Source |
|--------|--------|
| `bg-primary-{50-900}` | Rose scale via `--primary-*` |
| `bg-teal-{50-900}` | Teal scale via `--accent-*` |

## Typography

- **Font stack**: `font-sans` (system default via Tailwind)
- **Monospace**: `font-mono` — room codes, step numbers, technical labels
- **Heading weights**: `font-bold` (700), `font-semibold` (600)
- **Body weights**: `font-medium` (500), `font-normal` (400)
- **Label style**: `text-[10px] tracking-widest uppercase font-semibold` — section headers, nav categories

## Spacing & Layout

- **Radius**: the scale above; `--radius` 12px is the default corner
- **Page padding**: `px-4 sm:px-8 md:px-16 lg:px-24`
- **Section spacing**: `space-y-20` between page sections
- **Component spacing**: `space-y-4` for list items, `gap-2` for inline groups
- **Sidebar width**: `w-52` (208px) — dashboard layout
- **Content max-width**: `max-w-xl` (pages), `max-w-md` (forms), `max-w-[360px]` (auth forms)

## Component Patterns

### Buttons
- Primary: `bg-primary text-primary-foreground hover:bg-primary-hover`
- Secondary: `variant="outline"` — border + transparent bg
- Destructive: `text-destructive hover:bg-destructive/10`
- Sign-out: a plain ghost button — signing out is neither an error nor irreversible

### Confirmation dialogs
- `AlertConfirmDialog`: Cancel is a ghost button. Confirm is `destructive` for an action that loses
  something (delete, ban, kick) and `secondary` for one that loses nothing (promote, demote).
- The title names who or what the action is about: "Kick Sam?", not "Kick participant?".
- Asks first: banning a user, kicking or banning a participant, deleting a user.
  Single tap: re-activating a user, since it only restores access.
- A dialog that saves to the server holds still while it saves: Escape and the scrim do not close it.

### Toasts
- `@/components/ui/sonner` is the app's only toaster: one inverse bar for every kind, 16px corners,
  and an icon in the inverse status colour, named for a screen reader, that says the kind.
- Only an error carries a close button.
- Kinds: a failure is `toast.error`; validation and "can't do that here" are `toast.warning`, since
  nothing failed; being removed from a call (room closed, account deleted) is `toast.warning`;
  saved, copied and sent are `toast.success`; a notice is `toast.info`.

### Navigation
- Sidebar: fixed left, `bg-card` with `border-r`
- Mobile: `Sheet` slide-out from left
- Active state: `bg-primary/10 text-primary`
- Inactive: `text-muted-foreground hover:bg-accent`

### Cards
- Border-only, no shadow on rest state
- Hover: subtle lift (`hover:-translate-y-0.5`) with faint shadow

### Focus
- All interactive elements: `focus-visible:ring-2 focus-visible:ring-ring`
- Ring: 3px `--primary-500` at 45% opacity

### Inputs
- Bare: `border` only, no background fill
- Focus: `ring-2 ring-ring`
- Corner: `rounded-lg` (12px, the field size)

## Dark Mode

- Class-based: `.dark` on `<html>`
- Anti-flash script inlined in `<head>` (reads localStorage)
- All tokens have `:root` (light) and `.dark` overrides
- Meeting room UI is always dark regardless of theme
- Auth left panel is always dark

## Responsive Breakpoints

| Breakpoint | Width | Usage |
|-----------|-------|-------|
| Default | 0–1023px | Phone layout: bottom navigation, sheets, no sidebar (`MOBILE_BREAKPOINT_PX` in `lib/use-is-mobile.ts`) |
| `sm` | 640px+ | Larger text, hostname prefix |
| `md` | 768px+ | Full headline size |
| `lg` | 1024px+ | Desktop: sidebar, auth brand panel, meeting side panels |

## Self-Hosting Customization

Edit `src/theme.css` to rebrand. One file controls all colors. See `theme.example-blue.css` for an example of a complete brand swap.

## What NOT to Do

- Do NOT write `rounded-[Npx]` — pick the class from the Border Radius table.
- Do NOT use color alone for status signals — always pair with icons or labels.
- Do NOT use `--destructive` for emphasis or a status — it's reserved for errors and irreversible actions.
- Do NOT put white text on `--accent-500` — always use ink text (`--fg-1`).
- Do NOT add hardcoded hex colors outside `theme.css`.
- Do NOT change the meeting room always-dark theme.
