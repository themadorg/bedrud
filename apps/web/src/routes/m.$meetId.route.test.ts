// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingRouteSource = readFileSync(new URL('./m.$meetId.tsx', import.meta.url), 'utf8')

describe('the meeting route', () => {
  // A closed room or a deleted account removes the person from the call; nothing on their side
  // failed, so Android #283 shows a removal as a warning.
  it.each(['Room closed', 'Account deleted'])('should report "%s" as a warning', (message) => {
    expect(meetingRouteSource).not.toMatch(new RegExp(`toast\\.(error|success)\\('${message}'`))
    expect(meetingRouteSource).toContain(`toast.warning('${message}'`)
  })
})
