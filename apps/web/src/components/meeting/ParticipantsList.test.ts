// @vitest-environment node
//
// This file reads the component source from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const listSource = readFileSync(new URL('./ParticipantsList.tsx', import.meta.url), 'utf8')

describe('ParticipantsList', () => {
  it('should carry no breakpoint overrides, because it only mounts at desktop width', () => {
    // Below 1024px the roster is MeetingInviteSheet, so an unprefixed size paired with an `lg:` one
    // can never render.
    expect(listSource).not.toMatch(/(^|[\s"'`])lg:/)
  })

  it('should keep the desktop header and close button sizes', () => {
    expect(listSource).toContain('h-[52px]')
    expect(listSource).toContain('px-4')
    expect(listSource).toContain('h-7 w-7')
    expect(listSource).toContain('<X size={15} />')
  })
})
