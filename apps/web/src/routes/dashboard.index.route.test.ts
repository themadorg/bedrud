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

  // The My Rooms chip can empty the list while recents still exist. Keyed on whether any room
  // exists anywhere, the empty state then reported `No rooms match ""` against an empty filter box.
  // The query is what decides which of the two empty states applies.
  it('should choose its empty state by the query rather than by what exists', () => {
    expect(dashboardRouteSource).toMatch(/\{normalizedQuery \? \(/)
  })

  // Unit 1 made 1024px the app's one phone breakpoint. The dashboard tree was never swept.
  it('should carry no breakpoint prefix outside the shared scale', () => {
    expect(dashboardRouteSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })

  // Deleting a room dropped it from the server list but left its name in this device's history, and
  // the list merges the two — so it returned as a recent card one frame after the "Room deleted"
  // toast, reading as a delete that did not work. Every room created or joined from this page is in
  // that history, so it reached nearly every delete. The mutation carries the name for this reason.
  it('should clear the local history entry when a room is deleted', () => {
    expect(dashboardRouteSource).toContain('removeRecent(name)')
    expect(dashboardRouteSource).toMatch(/deleteRoom\.mutate\(\{ id: entry\.room\.id, name: entry\.name \}\)/)
  })
})
