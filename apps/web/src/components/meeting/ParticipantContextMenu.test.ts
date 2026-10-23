// @vitest-environment node
//
// This file reads a component module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const menuSource = readFileSync(new URL('./ParticipantContextMenu.tsx', import.meta.url), 'utf8')

describe('ParticipantContextMenu', () => {
  // The title asked "Kick participant?" without saying who (Android #220).
  it('should name the person in the confirmation title', () => {
    expect(menuSource).toMatch(
      /title=\{pendingAdminAction \? `\$\{pendingAdminAction\.label\} \$\{pendingAdminAction\.name\}\?`/,
    )
  })

  // Promoting or demoting loses nothing, so only kick and ban confirm in the destructive colour.
  it('should confirm only kick and ban in the destructive colour', () => {
    expect(menuSource).toMatch(/pendingAdminAction\?\.key === 'kick' \|\| pendingAdminAction\?\.key === 'ban'/)
  })
})
