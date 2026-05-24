---
name: bedrud-fe-ui-foundation
description: Shared UI primitives — design system, theme, shadcn compliance, generic utilities.
license: Apache License
---

# Bedrud Frontend UI Foundation

React 19 SPA. `apps/web/`. TailwindCSS v4 + shadcn/ui.

---

## Shadcn/UI Compliance

All 4 phases complete. Key rules:
- **Prefer shadcn wrappers** over raw HTML: `Button`, `Input`, `Label`, `Select`, `Switch`, `Tabs`, `Dialog`, `RadioGroup`, `Card`, `Badge`, `Separator`, `Skeleton` from `@/components/ui/`
- **No inline `style={}`** for static values. Use Tailwind. Keep inline only for: `color-mix`, palette-based colors, computed dimensions from props
- **Use `cn()`** from `@/lib/utils` for dynamic className composition — no template-literal classNames
- **Gradient text banned** (`bg-clip-text text-transparent`) — use `text-primary`
- **Aurora blobs banned** — max one static radial glow per page
- **No hardcoded hex** for structural UI — use CSS var tokens

See `apps/web/AGENTS.md` for full design system reference.

---

## Styles / Theme

### `src/theme.css`

- **Light:** Rose primary (rose-600 `#E11D48`), teal accent, stone foreground/background
- **Dark:** Rose-400 primary, stone dark bg (`#0C0A09`)
- Semantic tokens: `--background`, `--foreground`, `--card`, `--primary`, `--muted`, `--accent`, `--destructive`, `--border`, `--input`, `--ring`
- Status: `--success-500` (#16A34A), `--destructive-500` (#DC2626)

### `src/styles.css`

- Imports TailwindCSS v4 + `theme.css`
- Dark mode: `&:where(.dark, .dark *)`
- Maps CSS vars → Tailwind theme tokens via `@theme inline`
- **Zero border-radius:** `border-radius: 0 !important` globally
- Keyframes: `meet-speaker-glow`, `meet-speak-bar`, `meet-panel-in`, `meet-tile-in`, `meet-ptt-pulse`, `meet-connecting-spin`, `chat-toast-in`, `hero-float-a/b/c`
- Utility classes: `.meet-speaking`, `.meet-panel`, `.meet-tile`, `.meet-ptt`, `.meet-connecting`, `.strip-scroll`, `.chat-toast`, `.hero-blob-a/b/c`, `.feature-card`

### `src/theme.example-blue.css`

Alternative blue+rose theme template. Copy to `theme.css` to rebrand.

---

## Generic Utilities

### `src/lib/utils.ts`

`cn(...inputs)` — Merge Tailwind classes. `clsx` + `twMerge`.

### `src/lib/errors.ts`

`getErrorMessage(error, fallback)` — Extract human-readable msg. Strips HTTP status prefix, parses JSON `message`/`error`/`detail`.

### `src/lib/participant-palette.ts`

`PALETTES` — 8 color palettes: `{ tile, avatar, glow }` each.
`getPalette(name)` — Deterministic palette by name hash → `PALETTES[0..7]`.

---

## UI Primitives — `src/components/ui/`

25 shadcn/ui files (Radix-based). No custom logic.

| File | Primitive |
|------|-----------|
| `avatar.tsx` | `@radix-ui/react-avatar` |
| `badge.tsx` | cva variants: default, secondary, destructive, outline |
| `button.tsx` | `@radix-ui/react-slot` (asChild). Variants × sizes |
| `card.tsx` | Plain div |
| `checkbox.tsx` | `@radix-ui/react-checkbox` |
| `context-menu.tsx` | `@radix-ui/react-context-menu` |
| `dialog.tsx` | `@radix-ui/react-dialog` |
| `dropdown-menu.tsx` | `@radix-ui/react-dropdown-menu` |
| `input.tsx` | Plain input |
| `label.tsx` | `@radix-ui/react-label` |
| `radio-group.tsx` | `@radix-ui/react-radio-group` |
| `scroll-area.tsx` | `@radix-ui/react-scroll-area` |
| `select.tsx` | `@radix-ui/react-select` |
| `separator.tsx` | `@radix-ui/react-separator` |
| `sheet.tsx` | `@radix-ui/react-dialog`. Side: top/bottom/left/right |
| `skeleton.tsx` | Plain div |
| `switch.tsx` | `@radix-ui/react-switch` |
| `table.tsx` | Plain HTML table |
| `tabs.tsx` | `@radix-ui/react-tabs` |
| `tooltip.tsx` | `@radix-ui/react-tooltip` |

Add new: `cd apps/web && bunx shadcn@latest add <name>`.
