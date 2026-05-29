# Bedrud Marketing & Docs Design System

This document defines the design guidelines and brand standards for the Bedrud static marketing and documentation site (`apps/site`). It is adapted from the project-wide visual design system.

---

## Brand Aesthetic

The marketing and documentation site aligns with the core product: **Retro, Sharp, Minimalist**.

- **Primary Brand Color**: Rose (for primary CTAs, links, interactive highlights)
- **Accent Color**: Teal (for secondary badges, highlights, platform indicators)
- **Corner Style**: **Zero border-radius** (`border-radius: 0px`). All buttons, cards, containers, input boxes, and sections MUST have sharp, 90-degree corners.
- **Background Contrast**: Alternate sections between `bg-background` and `bg-muted/30` with `border-y border-border/40` to create structured landing layouts.

---

## Typography

- **Headings**: Semibold or Bold tracking-tight headings.
- **Body Text**: Clean system-ui or Inter sans-serif stack.
- **Monospace Text**: Code blocks, path locations, CLI commands use monospace tracking-normal fonts.
- **Category Labels**: Small, uppercase, letter-spaced trackings (`text-[11px] tracking-wider uppercase font-semibold text-muted-foreground`).

---

## Spacing & Structure

- **Section Margins**: Vertical padding uses `py-12 lg:py-16` or `py-20` on section blocks.
- **Grid Layout**: Max default container width uses the standard `.section-container` (centered `max-w-7xl` or equivalent with responsive horizontal paddings).
- **Page Layout**:
  - Landing pages use `Landing.astro` wrapper with full headers and footers.
  - Docs use `DocsLayout.astro` with interactive sidebar, table of contents (TOC), and the unified landing footer.

---

## Components

### Buttons
- **Primary CTA**: Clean filled block using `--primary` background (`bg-primary text-primary-foreground`) with no rounded corners.
- **Secondary Action**: Border-only outlines (`border border-border bg-transparent text-foreground hover:bg-accent`).

### Cards & Blocks
- **Landing Grid Cards**: Sharp, thin borders (`border border-border`) with subtle shadows and card backgrounds (`bg-card`).
- **Feature Icons**: Placed in soft tint boxes (`bg-primary/10` or color-mix highlights), always aligned with accessibility standards (text/icon paired with descriptive label).

---

## API Docs Integration (Scalar)

The Scalar API Reference at `/api-docs` matches the site aesthetics perfectly:
- **Theme**: Disabled Scalar default themes (`theme: "none"`) to use custom Bedrud CSS theme overrides.
- **Layout**: Uses `modern` layout, framed in a clean container (`section-container`) inside a styled section block with vertical padding.
- **Border**: Sharp borders (`1px solid var(--border)`) and `zero border-radius`.
- **Dark Mode**: Scalar internal theme toggle is disabled (`hideDarkModeToggle: true`). The reference automatically observes and synchronizes with the main website's dark mode toggle.
