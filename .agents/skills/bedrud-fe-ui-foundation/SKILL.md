---
name: bedrud-fe-ui-foundation
description: Shared UI primitives — design system, theme, shadcn compliance, settings panels, generic utilities.
license: Apache License
---

# Bedrud Frontend UI Foundation

React SPA under `apps/web/`. TailwindCSS v4 + shadcn/ui (Radix) + `cn()`.

**Path aliases:** `#/*` and `@/*` both map to `./src/*`. Prefer `#/*` for app code; UI primitives often import `@/components/ui/*` and `@/lib/utils` (shadcn default). Either is valid.

**Design sources of truth (do not contradict):**
- Project: `DESIGN.md` — hard rules (zero radius, tokens, a11y)
- Web: `apps/web/AGENTS.md` — shadcn compliance, meeting conventions
- Tokens: `apps/web/src/theme.css` + `styles.css`

---

## Hard rules

| Rule | Detail |
|------|--------|
| **Zero border-radius** | Global `* { border-radius: 0 !important }` in `styles.css`. `--radius` / `--radius-sm/md/lg/xl` = `0px`. Do **not** add `rounded-*` for layout. Exception: circular avatars via `.meet-avatar-circle` (`border-radius: 50% !important`) and intentional circles. |
| **No gradient text** | Ban `bg-clip-text text-transparent`. Use `text-primary`. |
| **No aurora / multi-blob** | Max **one** static radial glow per page (`hsl(var(--primary))` or token). No animated aurora meshes. Landing hero blobs (`.hero-blob-*`) are the only multi-float exception already in CSS. |
| **No hardcoded hex** for structural UI | Colors live in `theme.css`. Brand accents only when no token exists (e.g. emerald live dot). |
| **`cn()` only** | Dynamic classNames via `cn()` from `@/lib/utils` (`clsx` + `twMerge`). No template-literal class strings. |
| **Prefer shadcn wrappers** | `Button`, `Input`, `Label`, `Select`, `Switch`, `Tabs`, `Dialog`, `RadioGroup`, `Card`, `Badge`, `Separator`, `Skeleton`, `Checkbox`, `Slider`, `Sheet`, `Popover`, `Command`, `Table`, `Tooltip`, etc. over raw HTML. |
| **No static `style={}`** | Tailwind for static layout. Inline only: `color-mix`, palette colors, computed dimensions, progress transform, dynamic intensity. |
| **Destructive reserved** | Leave call / delete / irreversible only — never emphasis. |
| **Color never sole signal** | Pair status with icon, label, ring, or pattern. |
| **Icons** | Lucide React only. |

Add primitive: `cd apps/web && bunx shadcn@latest add <name>`.

---

## Shadcn/UI compliance

4-phase overhaul complete (2026-05-16). See `apps/web/AGENTS.md`.

**Prefer from `@/components/ui/`:** Button, Input, Label, Select, Switch, Tabs, Dialog, RadioGroup, Card, Badge, Separator, Skeleton, Checkbox, Slider, Sheet, Popover, Command, Table, Tooltip, ScrollArea, Avatar, ContextMenu, DropdownMenu, Pagination, Progress, Alert.

**Remaining debt (low priority):** some meeting files still use inline styles for `color-mix` / dark glass (acceptable).

---

## Theme / styles

### `src/theme.css` — brand tokens (Rose + Teal, Retro)

Sharp corners. Bold colors. **No purple.**

| Role | Light | Dark |
|------|-------|------|
| Primary | rose `#E11D48` (`--primary` / `--primary-600`) | rose-400 `#FB7185` |
| Primary hover | `#BE123C` | `#FDA4AF` |
| Accent | teal scale `--accent-50…900`; semantic `--accent` wash | muted dark surfaces |
| Background | `#fafafa` (neutral light) | `#0C0A09` stone-dark |
| Card / popover | `#ffffff` | `#1C1917` |
| Destructive | `#DC2626` | `#F87171` |
| Success | `--success-500` `#16A34A` | (shared scale) |
| Warning | `--warning-500` `#D97706` | |
| Ring | primary | primary dark |
| Radius | `--radius: 0px` | |

**Semantic tokens:** `--background`, `--foreground`, `--card`, `--card-foreground`, `--popover`, `--popover-foreground`, `--primary`, `--primary-foreground`, `--primary-hover`, `--primary-active`, `--secondary`, `--muted`, `--muted-foreground`, `--accent`, `--accent-foreground`, `--destructive`, `--destructive-foreground`, `--border`, `--input`, `--ring`.

**Chrome helpers:** `--fg-1/2/3`, `--bg`, `--bg-alt`, `--line`.

**Spotlight (hero glows):** `--spotlight-a/b/c`.

**Focus:** `--focus-ring: 0 0 0 3px color-mix(in oklab, var(--primary-500) 45%, transparent)` — applied globally via `:focus-visible` in `styles.css`.

Full rose scale: `--primary-50…900`. Full teal scale: `--accent-50…900` → Tailwind `primary-*` and `teal-*`.

### `src/styles.css`

- Imports: Tailwind v4 → `theme.css` → `components/meeting/meeting.css`
- Dark mode: `@custom-variant dark (&:where(.dark, .dark *))` — class on `<html>`
- `@theme inline` maps CSS vars → Tailwind color/radius tokens (`--radius-*` = 0)
- **Farsi:** Shabnam `@font-face`; `:lang(fa)` font stack
- Global base: border-color, **zero radius**, body bg/fg
- View Transitions stubs for theme switch circle
- **A11y:** global `:focus-visible` → `var(--focus-ring)`; `prefers-reduced-motion` kills animations

**Keyframes / utilities:**

| Class / keyframe | Use |
|------------------|-----|
| `meet-speaker-glow` / `.meet-speaking` | Active speaker glow |
| `meet-speak-bar` | Waveform bars |
| `meet-panel-in` / `.meet-panel` | Side panel slide-in |
| `meet-tile-in` / `.meet-tile` | Tile appear |
| `meet-ptt-pulse` / `.meet-ptt` | Push-to-talk pulse |
| `meet-connecting-spin` / `.meet-connecting` | Connecting spinner |
| `.strip-scroll` | Hide scrollbar (participant strip) |
| `chat-toast-in` / `.chat-toast` | Chat toast enter |
| `hero-float-a/b/c` / `.hero-blob-*` | Landing ambient motion |
| `.feature-card` | Landing card hover lift |
| `.meet-avatar-circle` | Force circular avatar (overrides zero-radius) |

Meeting shared styles also in `components/meeting/meeting.css` (tile, chat scroll, meet tokens).

### `src/theme.example-blue.css`

Blue primary + rose accent rebrand template. Copy over `theme.css` to apply. Partial token override example for self-hosters.

---

## Generic utilities

### `src/lib/utils.ts`

```ts
cn(...inputs: ClassValue[]) // clsx + twMerge
```

### `src/lib/errors.ts`

```ts
getErrorMessage(error: unknown, fallback: string): string
```

Strips leading `NNN: ` HTTP status; parses JSON `message` / `error` / `detail`. Never show raw JSON error blobs in UI.

### `src/lib/participant-palette.ts`

```ts
type ParticipantPalette = { tile: string; avatar: string; glow: string }
PALETTES: ParticipantPalette[]  // 8 deterministic palettes
getPalette(name: string)        // hash name → palette
```

Avatar gradients here are **participant identity only**, not structural UI chrome (gradient-text ban still applies to headings/CTAs).

### Related (not primitives, often paired)

| File | Role |
|------|------|
| `lib/theme.store.ts` | Theme mode + `resolveTheme`; class on `<html>` |
| `components/ThemeToggle.tsx` | Light/dark toggle; optional View Transition circle clip |
| `lib/avatar-photo-palette.ts` | Avatar photo color helpers |

---

## UI primitives — `src/components/ui/`

**26 files.** Mostly shadcn/Radix wrappers. Import: `@/components/ui/<name>` or `#/components/ui/<name>`.

| File | Base | Exports / notes |
|------|------|-----------------|
| `alert.tsx` | **Custom** (not Radix) | `Alert` — props: `type: 'success' \| 'error'`, `message`, `className`. Inline form feedback; icon + border tint. |
| `avatar.tsx` | `@radix-ui/react-avatar` | `Avatar`, `AvatarImage`, `AvatarFallback` |
| `badge.tsx` | cva | `Badge`, `badgeVariants`. Variants: `default`, `secondary`, `destructive`, `outline` |
| `button.tsx` | `@radix-ui/react-slot` | `Button`, `buttonVariants`. **Variants:** `default`, `destructive`, `outline`, `secondary`, `ghost`, `link`. **Sizes:** `default` (h-10), `sm`, `lg`, `icon`. `asChild` supported. Primary uses `hover:bg-primary-hover`. |
| `card.tsx` | div | `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter` |
| `checkbox.tsx` | `@radix-ui/react-checkbox` | `Checkbox` |
| `command.tsx` | `cmdk` + Dialog | `Command`, `CommandDialog`, `CommandInput`, `CommandList`, `CommandEmpty`, `CommandGroup`, `CommandItem`, `CommandSeparator`, `CommandShortcut` |
| `context-menu.tsx` | `@radix-ui/react-context-menu` | Full suite: Root/Trigger/Content/Item/CheckboxItem/RadioItem/Label/Separator/Shortcut/Group/Portal/Sub/SubContent/SubTrigger/RadioGroup |
| `dialog.tsx` | `@radix-ui/react-dialog` | `Dialog`, `DialogTrigger`, `DialogPortal`, `DialogOverlay`, `DialogClose`, `DialogContent`, `DialogHeader`, `DialogFooter`, `DialogTitle`, `DialogDescription` |
| `dropdown-menu.tsx` | `@radix-ui/react-dropdown-menu` | Same pattern as context-menu (DropdownMenu* names) |
| `input.tsx` | native `<input>` | `Input` |
| `label.tsx` | `@radix-ui/react-label` | `Label` (+ cva base styles) |
| `pagination.tsx` | Button variants | `Pagination`, `PaginationContent`, `PaginationItem`, `PaginationLink` (`isActive` → outline), `PaginationPrevious`, `PaginationNext`, `PaginationEllipsis` |
| `popover.tsx` | `@radix-ui/react-popover` | `Popover`, `PopoverTrigger`, `PopoverContent`, `PopoverAnchor` |
| `progress.tsx` | plain div | `Progress` — `value?: number`, `indicatorClassName?`. Width via `transform: translateX(...)`. |
| `radio-group.tsx` | `@radix-ui/react-radio-group` | `RadioGroup`, `RadioGroupItem` |
| `scroll-area.tsx` | `@radix-ui/react-scroll-area` | `ScrollArea`, `ScrollBar` |
| `select.tsx` | `@radix-ui/react-select` | `Select`, `SelectGroup`, `SelectValue`, `SelectTrigger`, `SelectContent`, `SelectLabel`, `SelectItem`, `SelectSeparator`, `SelectScrollUpButton`, `SelectScrollDownButton` |
| `separator.tsx` | `@radix-ui/react-separator` | `Separator` |
| `sheet.tsx` | `@radix-ui/react-dialog` | `Sheet`, `SheetTrigger`, `SheetClose`, `SheetPortal`, `SheetOverlay`, `SheetContent` (**side:** `top` \| `bottom` \| `left` \| `right`, default `right`), `SheetHeader`, `SheetFooter`, `SheetTitle`, `SheetDescription` |
| `skeleton.tsx` | div | `Skeleton` — pulse placeholder |
| `slider.tsx` | `@radix-ui/react-slider` | `Slider` |
| `switch.tsx` | `@radix-ui/react-switch` | `Switch` |
| `table.tsx` | HTML table | `Table`, `TableHeader`, `TableBody`, `TableFooter`, `TableHead`, `TableRow`, `TableCell`, `TableCaption` |
| `tabs.tsx` | `@radix-ui/react-tabs` | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` (supports vertical via Radix `orientation`) |
| `tooltip.tsx` | `@radix-ui/react-tooltip` | `Tooltip`, `TooltipTrigger`, `TooltipContent`, `TooltipProvider` |

**Note:** Components may still contain `rounded-*` class strings from shadcn defaults; global CSS zeroes them. Do not fight the system by re-introducing radius for “polish.”

---

## Settings panels — `src/components/settings/`

Shared user/meeting settings UI (not under `ui/`, but foundational for app chrome).

| File | Export | Role |
|------|--------|------|
| `BedrudSettingsDialog.tsx` | `BedrudSettingsDialog` | Meeting settings dialog: vertical `Tabs` + panels. Props: `open`, `onOpenChange`. Tabs: profile, appearance, audio, video, security, experimental. |
| `ProfileSettingsPanel.tsx` | `ProfileSettingsPanel` | Profile; `tone?: SettingsPanelTone` |
| `AppearanceSettingsPanel.tsx` | `AppearanceSettingsPanel` | Theme/UI prefs; `tone?` |
| `AudioSettingsPanel.tsx` | `AudioSettingsPanel` | Mic, noise suppression, gain, gate, PTT, muted beep; `tone?` |
| `VideoSettingsPanel.tsx` | `VideoSettingsPanel` | Camera device/prefs; `tone?`, optional `onCameraDeviceChange` |
| `SecuritySettingsPanel.tsx` | `SecuritySettingsPanel` | Password / security (no tone prop) |
| `ExperimentalSettingsPanel.tsx` | `ExperimentalSettingsPanel` | Feature flags; `tone?` |
| `PushToTalkKeyCapture.tsx` | `PushToTalkKeyCapture` | Keyboard capture for PTT key |
| `settingsPanelTone.ts` | tone helpers | See below |

### Tone system (`settingsPanelTone.ts`)

```ts
type SettingsPanelTone = 'default' | 'meeting'

isMeetingTone(tone)
panelSurfaceClass(tone)           // card surface for default vs meeting tokens
meetingSliderClass                // Slider track/range/thumb → --meet-* vars
meetingPanelScopeClass            // Scope overrides for bg-card / muted / combobox inside meeting dialog
settingsDialogScrollClass         // 'meet-scroll'
settingsSidebarTabClass           // vertical tab active/hover using --meet-* 
```

- **`default`** — dashboard/settings routes (`bg-card`, standard tokens)
- **`meeting`** — `BedrudSettingsDialog` and in-call UI; uses CSS vars from meeting theme (`--meet-border`, `--meet-fg`, `--meet-surface-muted`, etc.)

When building new settings rows: accept `tone`, branch with `isMeetingTone` / `panelSurfaceClass`, reuse `Slider` + `meetingSliderClass` in meeting context.

---

## Token → Tailwind cheat sheet

| Token | Classes |
|-------|---------|
| Page bg | `bg-background` |
| Surfaces | `bg-card`, `bg-muted` |
| CTA | `bg-primary text-primary-foreground hover:bg-primary-hover` |
| Secondary text | `text-muted-foreground` |
| Borders | `border-border`, `border-input` |
| Errors | `text-destructive`, `bg-destructive/10`, `border-destructive/30` |
| Hover chrome | `hover:bg-accent hover:text-accent-foreground` |
| Focus | `focus-visible:ring-2 focus-visible:ring-ring` (global box-shadow also applies) |
| Rose scale | `bg-primary-50` … `bg-primary-900` |
| Teal scale | `bg-teal-50` … `bg-teal-900` |
| Warning | `text-warning` / `bg-warning` (mapped from `--warning-500`) |

---

## Patterns (aligned with DESIGN.md + AGENTS.md)

**Buttons:** Use `<Button variant="…">` — not raw `<button>` for app chrome. No gradient CTAs. No `active:scale-95`.

**Forms:** `<Label>` + `<Input>` / `<Select>` / `<Checkbox>` / `<Switch>` / `<Slider>` / `<RadioGroup>`. Compound URL fields: border container + `focus-within:ring-2 focus-within:ring-ring`.

**Dialogs / sheets:** Prefer `Dialog` for centered modals; `Sheet` for side drawers (mobile nav, etc.).

**Feedback:** Prefer `Alert` for success/error banners; or the AGENTS pattern (`border-destructive/30 bg-destructive/10` + icon). Parse errors with `getErrorMessage`.

**One glow:** Optional single radial glow using primary token — not centered dead-center; no multi-layer aurora.

**Meeting room:** Always-dark atmosphere; prefer meeting CSS vars / `meeting.css` conventions from `AGENTS.md`. Settings inside call use `tone="meeting"`.

**Destructive:** only irreversible actions.

---

## Do / Don't (quick)

**Do**
- Semantic tokens + shadcn wrappers + `cn()`
- Sharp corners (global enforcement)
- Lucide icons; color + icon/label for status
- Edit `theme.css` only for rebrand; copy `theme.example-blue.css` as template

**Don't**
- Add `rounded-*` for visual radius
- Gradient text / aurora meshes / purple brand chrome
- Hardcoded hex outside `theme.css` for structure
- Raw HTML controls when a `ui/` primitive exists
- Use destructive for non-destructive emphasis
- White text on teal accent-500 (use ink `--fg-1`)

---

## Related skills / docs

- `apps/web/AGENTS.md` — full web design system + meeting rules
- `DESIGN.md` — project design system
- `apps/web/DESIGN.md` — web-specific notes
- Skill `bedrud-frontend` — routes, components map, app architecture
- Skill `bedrud-api` — API shapes for forms that call backend
)
