// @vitest-environment node
//
// This file reads a component from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dialogSource = readFileSync(new URL('./BedrudSettingsDialog.tsx', import.meta.url), 'utf8')

describe('the settings dialog breakpoint', () => {
  it('should not switch layout at the sm breakpoint', () => {
    expect(dialogSource).not.toMatch(/\bmax-sm:/)
    expect(dialogSource).not.toMatch(/(?<![\w-])sm:/)
  })

  it('should switch layout at the shared phone breakpoint', () => {
    expect(dialogSource).toMatch(/(?<![\w-])lg:hidden/)
    expect(dialogSource).toMatch(/\bmax-lg:/)
  })
})
