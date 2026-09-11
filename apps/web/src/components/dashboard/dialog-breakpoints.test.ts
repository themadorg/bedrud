// @vitest-environment node
//
// These files are read from disk, so this runs in Node rather than jsdom: jsdom's URL resolves
// `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const DIALOG_FILE_NAMES = ['CreateRoomDialog.tsx', 'RoomSettingsDialog.tsx']

describe('the dashboard dialogs', () => {
  it.each(DIALOG_FILE_NAMES)('should switch %s at the shared phone breakpoint', (fileName) => {
    const source = readFileSync(new URL(`./${fileName}`, import.meta.url), 'utf8')
    expect(source).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})
