// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const userDetailSource = readFileSync(new URL('./users_.$userId.tsx', import.meta.url), 'utf8')

describe('the admin user detail route', () => {
  // Status colours were hard-coded hex, and a banned user wore the error red although nothing had
  // failed (Android #219).
  it('should carry no hard-coded hex colour', () => {
    expect(userDetailSource).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
  })

  // One tap on the status button banned the user (Android #220).
  it('should ask before deactivating a user', () => {
    expect(userDetailSource).not.toMatch(/onClick=\{\(\) => toggleStatus\.mutate\(!user\.isActive\)\}/)
    expect(userDetailSource).toContain('<AlertConfirmDialog')
  })

  it('should re-activate a user without asking', () => {
    expect(userDetailSource).toMatch(/toggleStatus\.mutate\(true\)/)
  })
})
