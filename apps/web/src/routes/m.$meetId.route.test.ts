// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingRouteSource = readFileSync(new URL('./m.$meetId.tsx', import.meta.url), 'utf8')

describe('the meeting prejoin', () => {
  // The prejoin offered the guest name form on every instance, so on one with registration closed
  // or guest login off the only outcome was a 403 after the visitor typed a name.
  it('should read the public settings through the shared fetch', () => {
    expect(meetingRouteSource).toContain("from '#/lib/use-public-settings'")
    expect(meetingRouteSource).toContain('getPublicSettings()')
  })

  // The server refuses guest join only for closed registration or guest login off; invite-only
  // instances still accept guests, so the prejoin must not block on tokenRegistrationOnly.
  it('should block guests on closed registration or guest login off only', () => {
    expect(meetingRouteSource).toContain('!settings.registrationEnabled || !settings.guestLoginEnabled')
    expect(meetingRouteSource).not.toContain('tokenRegistrationOnly')
  })

  // Hiding the form alone leaves the request path open; the guest-join call itself has to wait
  // until the settings say guests may join.
  it('should gate the guest-join request on the settings', () => {
    expect(meetingRouteSource).toMatch(/guestJoinPolicy\?\.blocked === false && guestName !== null && guestName !== ''/)
  })

  // Rendering the form before the settings resolve would flash it and then swap it out.
  it('should hold the prejoin back until the settings resolve', () => {
    expect(meetingRouteSource).toMatch(/if \(guestJoinPolicy === null\) \{\s*\n\s*return <MeetLoadingScreen \/>/)
  })

  it('should offer sign in back to the room when guests cannot join', () => {
    expect(meetingRouteSource).toContain('Guest access is disabled')
    expect(meetingRouteSource).toMatch(
      /navigate\(\{ to: '\/auth\/login', search: \{ redirect: `\/m\/\$\{meetId\}` \} \}\)/,
    )
  })
})
