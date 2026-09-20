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
const controlsRow = source('MeetingCallControlsRow.tsx')
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

  it('should be the row element that gives way when the controls do not fit', () => {
    // The five controls plus this pill's reserved label are wider than a 375px phone. Whatever gives
    // way has to be this label: it was the hang-up button, cropped off the end of the row by 11px.
    const pillButtonClass = micPill.match(/'flex h-12[^']*'/)?.[0]
    expect(pillButtonClass).toBeDefined()
    expect(pillButtonClass).not.toContain('shrink-0')
    expect(pillButtonClass).toContain('min-w-0')
    expect(pillButtonClass).toContain('overflow-hidden')
  })

  it('should squeeze the label rather than the icon beside it', () => {
    // The button shrinking is only useful if the glyph holds its size; otherwise the mic icon is the
    // first thing to distort.
    expect(micPill).toContain('<Mic size={18} className="shrink-0" />')
    expect(micPill).toContain('<MicOff size={18} className="shrink-0" />')
  })
})

describe('the call controls row', () => {
  it('should keep every control inside the pill at a phone width', () => {
    // Both side clusters size from a zero basis so the mic slot stays centred under the handle. A
    // zero basis alone lets a cluster be allotted less than its own buttons need, and the overflow
    // lands on the last control in the row — the one nobody can afford to lose.
    const clusters = controlsRow.match(/className="flex [^"]*flex-1[^"]*"/g)
    expect(clusters).toHaveLength(2)
    for (const cluster of clusters ?? []) {
      expect(cluster).toContain('min-w-fit')
    }
  })

  it('should keep the hang-up button at its own width rather than shrinking it', () => {
    expect(controlsRow).toContain('h-12 w-14 shrink-0')
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
