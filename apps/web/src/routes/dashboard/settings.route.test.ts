// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const desktopSettingsSource = readFileSync(new URL('./settings.tsx', import.meta.url), 'utf8')

describe('the desktop settings route', () => {
  // The whole guarded statement is pinned rather than its fragments. An inverted condition,
  // `if (!isMobileViewport())`, still contains every fragment separately, and it would send desktop
  // visitors to `/settings`, whose own gate sends them straight back: an endless bounce.
  it('should redirect when the viewport is a phone, and not when it is not', () => {
    expect(desktopSettingsSource).toMatch(
      /if \(isMobileViewport\(\)\) navigate\(\{ to: '\/settings', replace: true \}\)/,
    )
  })

  it('should read the viewport inside an effect rather than through the hook', () => {
    expect(desktopSettingsSource).toContain('isMobileViewport')
    expect(desktopSettingsSource).not.toContain('useIsMobile')
  })
})
