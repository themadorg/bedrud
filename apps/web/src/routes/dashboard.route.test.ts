// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dashboardLayoutSource = readFileSync(new URL('./dashboard.tsx', import.meta.url), 'utf8')

describe('the dashboard layout', () => {
  // Signing out is neither an error nor irreversible, so Android #223 drew it as a plain ghost
  // button; the destructive hover said otherwise.
  it('should not colour sign-out as destructive', () => {
    const signOutButton = /<Button[^>]*?onClick=\{onLogout\}[\s\S]*?aria-label="Sign out"/.exec(
      dashboardLayoutSource,
    )?.[0]

    expect(signOutButton).toBeDefined()
    expect(signOutButton).not.toMatch(/destructive/)
  })

  // Restricted is a status, not an error, so it wears the warning colour (Android #219).
  it('should not draw the restricted admin label in the error red', () => {
    const restrictedLabel = /<span[^>]*>\s*Restricted/.exec(dashboardLayoutSource)?.[0]

    expect(restrictedLabel).toBeDefined()
    expect(restrictedLabel).not.toMatch(/destructive/)
  })
})
