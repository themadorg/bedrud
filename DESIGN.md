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
| `--destructive-500` | `#DC2626` | Leave call, delete (irreversible only) | Warning icon + label |

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
| Warning | `--accent-500` | Warning icon + label |
| Info | `--primary-500` | Info icon |

## Accessibility Rules (Non-Negotiable)

1. **Color is never the only signal.** Every status must also have an icon, label, ring, or pattern.
2. **Body text** on `--bg` uses `--fg-1` (14.5:1). Muted text uses `--fg-2` (4.7:1).
3. **Primary CTA** is `--primary-600` (6.1:1 on white). Hover goes to `--primary-700`.
4. **Destructive** (`--destructive-500`) is reserved for: leave call, delete, irreversible actions. Never for emphasis.
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
| `rounded-sm` | 8 | `sm` | chips, icon-button hover fills, swatches |
| `rounded-md` | 12 | `md` | buttons |
| `rounded-lg` | 12 | `md` | fields, inline banners, row containers |
| `rounded-xl` | 16 | `lg` | cards |
| `rounded-2xl` | 20 | `xl` | |
| `rounded-3xl` | 28 | `xxl` | sheet top, video tile, controls bar |
| `rounded-full` | pill | `full` | avatars, pills, FAB |

`--radius` in `theme.css` is the default corner (12px) that self-hosters may retune: `rounded-md`
and `rounded-lg` both resolve to it, so retuning the one value moves every button and field. The
rest of the scale lives in `styles.css`. `src/design-tokens.test.ts` pins both. The shadcn primitives in
`apps/web/src/components/ui/` carry their corner class themselves (button `rounded-md`, input
`rounded-lg`, card `rounded-xl`, alert `rounded-lg`, badge `rounded-sm`, dialog `rounded-3xl`,
menus `rounded-md`), so a page never has to add one. Badge is the chip, not a pill; alert is the
inline banner and shares the banner corner with the hand-rolled ones beside it. Each primitive
carries exactly one `rounded-*` class — `cn` runs the string through tailwind-merge, which keeps
the last radius and drops the rest.

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
- Do NOT use `--destructive-500` for emphasis — it's reserved for irreversible actions.
- Do NOT put white text on `--accent-500` — always use ink text (`--fg-1`).
- Do NOT add hardcoded hex colors outside `theme.css`.
- Do NOT change the meeting room always-dark theme.
