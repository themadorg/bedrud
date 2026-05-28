# Bedrud Web — UI & Design System

This is the source of truth for all frontend design decisions.
Every agent or engineer touching the web app must follow these guidelines.

---

## Overhaul Complete (2026-05-16)

All 4 phases of shadcn/ui compliance overhaul are done:

| Phase | Scope | Status |
|-------|-------|--------|
| 1 — Mechanical Swaps | Replace raw `<button>`/`<input>`/`<label>`/`<select>`/`<div class="card...">` with Button, Input, Label, Select, Card, Badge, Skeleton from `ui/` | ✅ Done |
| 2 — Structural | Switch, RadioGroup, Tabs, Dialog, Separator, `cn()` for template-literals | ✅ Done |
| 3 — Meeting Room | 17 meeting files converted from 100% inline styles → Tailwind classes. `meeting.css` for shared meeting styles | ✅ Done |
| 4 — Cleanup | Gradient text removed, aurora blobs removed, extra glow pruned, inline `<style>` moved, hardcoded hex replaced | ✅ Done |

**Remaining (low priority):** `ParticipantContextMenu.tsx` — 55 inline styles, already uses shadcn components, mostly `color-mix` values for dark meeting theme.

---

## Core Philosophy

- **Semantic tokens over hardcoded hex.** Use `bg-primary`, `text-muted-foreground`, `border-input`, etc. — not `#6366f1` or `rgba(...)` for structural UI.
- **Prefer shadcn wrappers over raw HTML elements.** Use `<Button>`, `<Input>`, `<Label>`, `<Select>`, `<Switch>`, `<Tabs>`, `<Dialog>`, `<RadioGroup>`, `<Card>`, `<Badge>`, `<Separator>` from `@/components/ui/`.
- **No inline `style={}` except for truly dynamic values** (width/height computed from props, palette colors, `color-mix` that can't be Tailwind). Static layout must use Tailwind classes.
- **One focal point per page.** The primary action (a form, a button) owns the visual hierarchy. Everything else is subordinate.
- **Left-aligned, top-anchored layouts.** Avoid dead-center vertical layouts. Content should flow from a natural reading anchor.
- **Minimal decoration.** A single subtle background glow is enough atmosphere. No aurora meshes, no animated blobs, no stacked gradients.
- **Compact but breathable.** Use tight sizing (`h-9`, `h-10`) with enough `space-y-*` between sections to let content breathe.

---

## Colors

Use shadcn CSS variable tokens for all structural colors:

| Token | Use |
|---|---|
| `bg-background` | Page background |
| `bg-card` | Surfaces, sidebars |
| `bg-muted` | Tab bars, skeleton backgrounds |
| `bg-primary` / `text-primary-foreground` | CTAs, logo marks, active nav |
| `text-muted-foreground` | Secondary text, labels, placeholders |
| `border-input` / `border-border` | Form borders, dividers |
| `text-destructive` / `bg-destructive/10` | Errors |
| `bg-accent` / `text-accent-foreground` | Hover states |

**Hardcoded hex is only acceptable for:**
- Brand accent decorations where no token exists (e.g. emerald live dot: `bg-emerald-500`)
- The single background glow using `hsl(var(--primary))`

---

## Background Glow

One radial gradient glow per page, using the primary token:

```tsx
<div
  className="pointer-events-none absolute h-[500px] w-[500px] rounded-full opacity-[0.15] dark:opacity-[0.10] blur-[100px]"
  style={{ background: 'radial-gradient(circle, hsl(var(--primary)) 0%, transparent 70%)' }}
  aria-hidden
/>
```

Position it off to one side (top-right, top-left) — not dead center.
**No aurora mesh. No animated blobs. No grid overlays. No multiple layered glows.**

---

## Typography

- Page titles: `text-xl font-semibold tracking-tight` (dashboard) or `text-4xl font-semibold tracking-tight` (landing)
- Section labels: `text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50`
- Body / subtitles: `text-sm text-muted-foreground`
- Monospace (room slugs, codes, URLs): `font-mono text-sm`
- **No gradient text.** No `bg-clip-text text-transparent` with brand gradients on headings.

---

## Buttons

Use `bg-primary text-primary-foreground` with `rounded-md` or `rounded-lg`:

```tsx
// Primary
<button className="inline-flex items-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90">

// Ghost / secondary
<button className="rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground">
```

**No gradient buttons.** No `linear-gradient(135deg, #6366f1 ...)` on CTAs.
**No `active:scale-95`** — it feels cheap.

---

## Forms & Inputs

Plain border with focus ring via Tailwind utilities:

```tsx
<input className="h-10 rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring" />
```

For a compound input (URL prefix + input + button), wrap in a border container and apply `focus-within:ring-2 focus-within:ring-ring`:

```tsx
<form className="flex items-center gap-0 rounded-lg border border-input bg-background focus-within:ring-2 focus-within:ring-ring">
  <span className="pl-4 font-mono text-sm text-muted-foreground/50 select-none">prefix/</span>
  <input className="h-10 flex-1 bg-transparent px-2 font-mono text-sm outline-none" />
  <button className="m-1 rounded-md bg-primary px-3 ...">Go</button>
</form>
```

**No focused gradient borders. No `boxShadow` glow on inputs.**

---

## Navigation (Sidebar)

Active nav item uses primary tint — no hardcoded hex:

```tsx
className={cn(
  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
  active
    ? 'bg-primary/10 text-primary'
    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
)}
```

Logo mark: `bg-primary` square/circle with `text-primary-foreground` icon inside.

---

## Cards & Surfaces

```tsx
// Standard card
<div className="rounded-xl border bg-card p-5">

// Empty state
<div className="rounded-xl border border-dashed py-20 text-center">

// Stat row
<div className="grid grid-cols-N divide-x divide-border rounded-lg border text-center">
```

No gradient card backgrounds. No colored border stripes.
Hover lift (`hover:-translate-y-0.5`) is acceptable on room cards only.

---

## Error & Status Feedback

```tsx
// Inline error (form, banner)
<div className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
  <AlertCircle className="h-4 w-4 shrink-0" />
  {message}
</div>
```

Parse raw JSON error bodies before displaying. Show the `error` or `message` field — never raw `{"error":"..."}` strings in the UI.

---

## Status Indicators

| State | Pattern |
|---|---|
| Live / active | `<span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />` |
| Inactive | `<span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/30" />` |
| Admin badge | `border border-destructive/30 bg-destructive/10 text-destructive` |

---

## Icons

Use **Lucide React** exclusively.

- `Radio` — bedrud logo mark
- `Lock` — encryption / private
- `Server` — self-hosted
- `AlertCircle` — errors
- `Plus` — create actions
- `ArrowRight` — navigation / submit

---

## Avatars

```tsx
<AvatarFallback className="bg-primary text-[10px] font-semibold text-primary-foreground">
  {initials}
</AvatarFallback>
```

No gradient avatar backgrounds.

---

## Animations

Avoid animations in the dashboard. On landing pages, a single subtle keyframe is acceptable if purposeful.

**Never use:**
- `aurora-drift`
- `beacon` pulse rings
- `float` vertical bob
- `blob` morph
- `bg-clip-text text-transparent` (gradient text — banned)

**Acceptable:**
- `animate-pulse` on skeleton loaders
- `animate-spin` on loading spinners
- `@keyframes meet-speak-bar` for meeting waveform animation
- `@keyframes meet-connecting-spin` for meeting spinner
- `@keyframes wave` for auth layout waveform

---

## Meeting Room — Dark Theme Conventions

The meeting room has a distinct dark atmosphere regardless of the rest of the app:

### File: `meeting.css`

Shared meeting styles at `components/meeting/meeting.css`:
- `.meet-tile` — base participant tile (position: relative, overflow: hidden)
- `.meet-tile.meet-speaking` — speaking glow via `box-shadow` with `var(--primary)`
- `@keyframes meet-speak-bar` — waveform bar animation
- `@keyframes meet-connecting-spin` — loading spinner
- `.meet-chat-scroll` — chat scrollbar styling

### Rules
- Use `bg-[#0c0c16]/90` or `bg-[#0f0f1c]/98` for dark surfaces — not `bg-background` (which may be light)
- Use `text-white/*` opacity variants for text on dark backgrounds
- Use `border-white/[0.07]` for subtle borders
- `color-mix(in oklab, var(--primary) X%, transparent)` is acceptable inline for dynamic intensity — no Tailwind equivalent exists
- Use `backdrop-blur-xl` / `backdrop-blur-lg` for glass effects
- Keep `style={{}}` for: `color-mix` backgrounds/borders, palette-based colors, computed dimensions (avatarPx), and `isSpeaking` waveform animation delays
- Convert everything else to Tailwind classes

### CtrlBtn convention (ControlsBar)
Use `btnIconCn(active, danger, isMobile)` helper that returns Tailwind classes:
```tsx
function btnIconCn(active = false, danger = false, isMobile = false) {
  return cn(
    'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color] duration-150',
    isMobile ? 'h-[38px] w-[38px] rounded-[10px]' : 'h-11 w-11 rounded-xl',
    danger
      ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
      : active
        ? 'bg-primary/25 text-sky-300 hover:bg-primary/30'
        : 'bg-white/[0.07] text-white/75 hover:bg-white/[0.12]',
  )
}
```

## Do / Don't

**Do:**
- Use CSS variable tokens for all structural colors
- Use `bg-primary` for all primary actions and logo marks
- Use `font-mono` for room names, codes, and URL prefixes
- Keep pages left-aligned with a single action in focus
- Show one subtle background glow per full-page route
- Parse error JSON before showing it in the UI
- Use shadcn wrappers (`Button`, `Input`, `Label`, `Dialog`, etc.) over raw HTML
- Use `cn()` from `@/lib/utils` for dynamic className composition
- Import shadcn components from `@/components/ui/` and local utils from `#/lib/`

**Don't:**
- Use `linear-gradient(135deg, #6366f1 ...)` for buttons, backgrounds, or text
- Use aurora mesh, animated blobs, or grid overlays
- Use gradient text (`bg-clip-text text-transparent`)
- Center content vertically in the middle of the screen with nothing around it
- Hardcode hex colors for structural UI
- Add decorative elements that compete with the primary action
- Show raw JSON error strings to users
- Use inline `style={}` for static values — use Tailwind classes instead
