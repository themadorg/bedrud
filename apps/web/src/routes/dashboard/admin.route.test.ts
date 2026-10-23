// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const adminRouteSource = readFileSync(new URL('./admin.tsx', import.meta.url), 'utf8')

// The server's /admin group answers superadmins only, so anything wider opens a page whose every
// request is refused.
describe('the admin route gate', () => {
  it('should let only a superadmin through the loader and the guard', () => {
    expect(adminRouteSource.match(/canOpenAdmin\(user\)/g)).toHaveLength(2)
  })

  it('should not admit admins or moderators', () => {
    expect(adminRouteSource).not.toMatch(/canAccess = .*accesses\.includes/)
  })

  // A hard load runs the loader during SSR, where it returns early, so a deep link reaches the
  // guard with no user loaded. Rendering the outlet then shows the admin page to anyone.
  it('should load a missing user in the guard before deciding', () => {
    expect(adminRouteSource).toMatch(/if \(user === null\) \{\s*loadRegisteredUser\(\)/)
  })

  it('should render nothing until access is confirmed', () => {
    expect(adminRouteSource).toContain('if (!canAccess) return null')
  })
})
