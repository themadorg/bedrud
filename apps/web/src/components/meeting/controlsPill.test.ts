// @vitest-environment node
//
// These assertions read component sources from disk, so they run in Node rather than jsdom:
// jsdom's URL resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(fileName: string): string {
  return readFileSync(new URL(`./${fileName}`, import.meta.url), 'utf8')
}

const pill = source('MeetingControlsPill.tsx')
const micPill = source('MeetingMicPill.tsx')
const controlsBar = source('ControlsBar.tsx')

describe('the controls pill surface', () => {
  it('should carry exactly one corner class', () => {
    expect(pill).toContain('rounded-3xl')
    expect(pill.match(/rounded-(?:sm|md|lg|xl|2xl|3xl|none)\b/g)).toEqual(['rounded-3xl'])
  })

  it('should span the width rather than dock like the desktop bar', () => {
    expect(pill).toContain('inset-x-2')
    expect(pill).not.toContain('meetControlsDockClass')
    expect(pill).not.toContain('-translate-x-1/2')
  })

  it('should sit above the bottom safe area', () => {
    expect(pill).toContain('env(safe-area-inset-bottom, 0px)')
  })

  it('should dim the call with the meeting scrim', () => {
    expect(pill).toContain('var(--meet-scrim)')
  })

  it('should resolve a drag through the shared rule rather than its own threshold', () => {
    expect(pill).toContain('expandedAfterDrag')
    expect(pill).not.toMatch(/\b24\b/)
  })

  it('should close on Escape', () => {
    expect(pill).toContain('Escape')
  })
})

describe('the mic pill', () => {
  it.each(['Speak', 'Muted', 'Push to Talk', 'Talking…'])('should carry the %s label', (label) => {
    expect(micPill).toContain(label)
  })
})

describe('the controls bar after the pill lands', () => {
  it('should no longer open a full-screen sheet on a phone', () => {
    expect(controlsBar).not.toContain('moreOpen && isMobile')
    expect(controlsBar).not.toContain('audioOpen && isMobile')
  })

  it('should no longer carry the more menu sub-page machinery', () => {
    expect(controlsBar).not.toContain('morePage')
    expect(controlsBar).not.toContain('moreNavDir')
  })

  it('should render the pill below the phone breakpoint', () => {
    expect(controlsBar).toContain('MeetingControlsPill')
  })

  it('should keep the desktop dropdown', () => {
    expect(controlsBar).toContain('DropdownMenu')
  })
})
