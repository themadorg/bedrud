// @vitest-environment node
//
// This file reads a component's source from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const sheetSource = readFileSync(new URL('./BedrudSheet.tsx', import.meta.url), 'utf8')
const handleSource = readFileSync(new URL('./BedrudSheetHandle.tsx', import.meta.url), 'utf8')

describe('BedrudSheet', () => {
  it('should wear the sheet corner, and only that corner', () => {
    expect(sheetSource).toMatch(/\brounded-t-3xl\b/)
    expect(sheetSource.match(/\brounded-(?!t-3xl)[a-z0-9-]+\b/g)).toBeNull()
  })

  it('should lift off the call behind it with the meeting sidebar container', () => {
    expect(sheetSource).toContain('var(--meet-sidebar)')
  })

  it('should clear the bottom safe area itself', () => {
    expect(sheetSource).toContain('env(safe-area-inset-bottom')
  })

  it('should take its ceiling from the meeting token rather than a second copy of the arithmetic', () => {
    expect(sheetSource).toContain('max-h-[var(--meet-sheet-max-height)]')
    expect(sheetSource).not.toContain('maxHeight')
  })

  it('should not introduce a breakpoint other than the shared one', () => {
    expect(sheetSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})

describe('BedrudSheetHandle', () => {
  it('should draw the pill at the metrics Android uses', () => {
    expect(handleSource).toMatch(/\bh-1\b/)
    expect(handleSource).toMatch(/\bw-8\b/)
    expect(handleSource).toMatch(/\brounded-full\b/)
  })

  it('should give the pill a target that meets the touch minimum', () => {
    expect(handleSource).toMatch(/\bh-12\b/)
  })

  it("should be vaul's own handle, so the tap and the drag stay with vaul", () => {
    expect(handleSource).toContain('Drawer.Handle')
  })

  it('should carry an accessible name, because vaul renders the handle as a bare div', () => {
    expect(handleSource).toContain('aria-label={label}')
  })

  it('should let a sheet with one height opt out of cycling', () => {
    expect(handleSource).toContain('preventCycle')
  })
})

describe('the ui folder', () => {
  it('should carry exactly one sheet primitive', () => {
    expect(() => readFileSync(new URL('./sheet.tsx', import.meta.url))).toThrow()
  })
})
