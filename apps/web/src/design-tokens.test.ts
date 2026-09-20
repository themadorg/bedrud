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
    ['badge.tsx', 'rounded-sm'],
    ['tabs.tsx', 'rounded-lg'],
    ['dropdown-menu.tsx', 'rounded-md'],
    ['context-menu.tsx', 'rounded-md'],
    ['popover.tsx', 'rounded-md'],
    ['command.tsx', 'rounded-md'],
    ['tooltip.tsx', 'rounded'],
    ['skeleton.tsx', 'rounded-md'],
    ['checkbox.tsx', 'rounded'],
  ])('should give %s the %s corner', (file, className) => {
    expect(hasClass(primitiveSource(file), className)).toBe(true)
  })
})
