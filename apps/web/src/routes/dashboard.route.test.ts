// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dashboardLayoutSource = readFileSync(new URL('./dashboard.tsx', import.meta.url), 'utf8')

describe('the dashboard sidebar', () => {
  // The server's /admin group answers superadmins only; an admin shown the entry lands on a page
  // whose every request is refused.
  it('should show the admin nav only to a superadmin', () => {
    expect(dashboardLayoutSource).toContain('{canOpenAdmin(user) && (')
    expect(dashboardLayoutSource).not.toContain('{user?.isAdmin && (')
  })
})
