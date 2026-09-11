// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dashboardRouteSource = readFileSync(new URL('./dashboard.index.tsx', import.meta.url), 'utf8')

describe('the dashboard route', () => {
  // The bar was hidden below 768px, which left a phone with no way to reach a room it did not
  // already own. No source-reading test can observe a viewport, but it can pin the absence of every
  // prefix that could hide the bar again.
  it('should carry no breakpoint prefix that hides content', () => {
    expect(dashboardRouteSource).not.toMatch(/max-(sm|md):hidden/)
  })

  it('should resolve the join field through the shared parser', () => {
    expect(dashboardRouteSource).toContain('parseJoinInput')
  })

  // The old code slugified whatever was in the field, so a pasted URL became a room name that could
  // never resolve. The parser returns null for that input and the bar has to handle the null rather
  // than pass it on.
  it('should report input that names no room rather than navigating', () => {
    expect(dashboardRouteSource).toMatch(/parseJoinInput\(value\)\s*\n\s*if \(!roomName\)/)
  })
})
