// @vitest-environment node
//
// This file reads stylesheets from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const stylesCss = readFileSync(new URL('./styles.css', import.meta.url), 'utf8')
const themeCss = readFileSync(new URL('./theme.css', import.meta.url), 'utf8')

/** Reads the first `--name: value;` declaration from a stylesheet. */
function tokenValue(css: string, name: string): string | undefined {
  return new RegExp(`${name}:\\s*([^;]+);`).exec(css)?.[1].trim()
}

/** Reads a `--name: value;` declaration from the dark theme block of theme.css. */
function darkTokenValue(name: string): string | undefined {
  return tokenValue(themeCss.slice(themeCss.indexOf('.dark {')), name)
}

/** Reads a shadcn primitive's source so the test can pin the corner class it carries. */
function primitiveSource(fileName: string): string {
  return readFileSync(new URL(`./components/ui/${fileName}`, import.meta.url), 'utf8')
}

/**
 * Checks whether a Tailwind class name is present in a primitive's source. A bare class name
 * (`rounded`) cannot use a `\b` boundary on both sides, because `\b` sits between a word
 * character and a non-word character, and the hyphen in `rounded-md` is a non-word character
 * too — so `/\brounded\b/` still matches inside `rounded-md`. Requiring whitespace or a quote
 * character on both sides instead rules out the hyphenated classes.
 */
function hasClass(source: string, className: string): boolean {
  return className.includes('-')
    ? new RegExp(`\\b${className}\\b`).test(source)
    : new RegExp(`[\\s'"\`]${className}[\\s'"\`]`).test(source)
}

describe('radius tokens', () => {
  it.each([
    ['--radius-sm', '8px'],
    ['--radius-md', 'var(--radius)'],
    ['--radius-lg', 'var(--radius)'],
    ['--radius-xl', '16px'],
    ['--radius-2xl', '20px'],
    ['--radius-3xl', '28px'],
  ])('should set %s to %s', (name, value) => {
    expect(tokenValue(stylesCss, name)).toBe(value)
  })

  it('should set the default corner to the button and field size', () => {
    expect(tokenValue(themeCss, '--radius')).toBe('12px')
  })

  it('should not flatten every element with a universal border-radius', () => {
    expect(stylesCss).not.toMatch(/\*\s*\{[^}]*border-radius/)
  })
})

describe('primitive corners', () => {
  it.each([
    ['button.tsx', 'rounded-md'],
    ['input.tsx', 'rounded-lg'],
    ['select.tsx', 'rounded-lg'],
    ['card.tsx', 'rounded-xl'],
    ['alert.tsx', 'rounded-lg'],
    ['dialog.tsx', 'rounded-3xl'],
    ['badge.tsx', 'rounded-full'],
    ['tabs.tsx', 'rounded-lg'],
    ['dropdown-menu.tsx', 'rounded-xl'],
    ['context-menu.tsx', 'rounded-xl'],
    ['popover.tsx', 'rounded-md'],
    ['command.tsx', 'rounded-md'],
    ['tooltip.tsx', 'rounded'],
    ['skeleton.tsx', 'rounded-md'],
    ['checkbox.tsx', 'rounded'],
  ])('should give %s the %s corner', (file, className) => {
    expect(hasClass(primitiveSource(file), className)).toBe(true)
  })

  // A filter chip keeps the 8px chip corner while the badge beside a name becomes a pill, as the
  // Android client's FilterChip and BedrudBadge do.
  it('should keep the chip corner as a badge shape for filter chips', () => {
    expect(hasClass(primitiveSource('badge.tsx'), 'rounded-sm')).toBe(true)
  })

  it.each(['dropdown-menu.tsx', 'context-menu.tsx'])('should drop %s and its submenu with one shadow', (file) => {
    expect(primitiveSource(file)).not.toMatch(/\bshadow-md\b/)
  })
})

describe('colour roles', () => {
  // The error red sat in the rose primary's hue family; Android #176 moved it a step darker in
  // light theme and a step lighter in dark theme so the two read apart.
  it.each([
    ['--destructive-500', '#B91C1C'],
    ['--destructive', '#B91C1C'],
    ['--recording', '#B91C1C'],
    ['--warning-500', '#B45309'],
    ['--accent-container', '#CCFBF1'],
    ['--accent-container-foreground', '#134E4A'],
    ['--inverse-surface', '#1C1917'],
    ['--inverse-foreground', '#F5F0EE'],
    ['--inverse-error', '#EF4444'],
    ['--inverse-warning', '#FBBF24'],
    ['--inverse-success', '#5EEAD4'],
  ])('should set light %s to %s', (name, value) => {
    expect(tokenValue(themeCss, name)).toBe(value)
  })

  it.each([
    ['--destructive', '#EF4444'],
    ['--destructive-foreground', '#0C0A09'],
    ['--recording', '#EF4444'],
    ['--warning-500', '#FBBF24'],
    ['--accent-container', '#115E59'],
    ['--accent-container-foreground', '#CCFBF1'],
    ['--inverse-surface', '#F5F5F4'],
    ['--inverse-foreground', '#1C1917'],
    ['--inverse-error', '#B91C1C'],
    ['--inverse-warning', '#B45309'],
    ['--inverse-success', '#0D9488'],
  ])('should set dark %s to %s', (name, value) => {
    expect(darkTokenValue(name)).toBe(value)
  })

  it.each([
    ['--color-success', 'var(--success-500)'],
    ['--color-recording', 'var(--recording)'],
    ['--color-accent-container', 'var(--accent-container)'],
    ['--color-accent-container-foreground', 'var(--accent-container-foreground)'],
    ['--color-inverse-surface', 'var(--inverse-surface)'],
    ['--color-inverse-foreground', 'var(--inverse-foreground)'],
    ['--color-inverse-error', 'var(--inverse-error)'],
    ['--color-inverse-warning', 'var(--inverse-warning)'],
    ['--color-inverse-success', 'var(--inverse-success)'],
  ])('should expose %s to Tailwind as %s', (name, value) => {
    expect(tokenValue(stylesCss, name)).toBe(value)
  })

  it('should put the badge on the teal accent container', () => {
    expect(primitiveSource('badge.tsx')).toContain('bg-accent-container text-accent-container-foreground')
  })
})

describe('toasts', () => {
  it('should draw every toast on the inverse surface', () => {
    expect(tokenValue(stylesCss, '--normal-bg')).toBe('var(--inverse-surface)')
    expect(tokenValue(stylesCss, '--normal-text')).toBe('var(--inverse-foreground)')
  })

  it('should round a toast like the cards it floats over', () => {
    expect(tokenValue(stylesCss, '--border-radius')).toBe('var(--radius-xl)')
  })

  // Sonner draws the close button on every toast once `closeButton` is set; only an error keeps it.
  it('should hide the close button on every kind but an error', () => {
    expect(stylesCss).toMatch(
      /\[data-sonner-toast\]:not\(\[data-type='error'\]\) \[data-close-button\]\s*\{\s*display: none;/,
    )
  })
})
