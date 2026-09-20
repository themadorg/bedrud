// @vitest-environment node
//
// This file reads a stylesheet from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingCss = readFileSync(new URL('./meeting.css', import.meta.url), 'utf8')

/** Reads the first `--name: value;` declaration from a stylesheet. */
function tokenValue(css: string, name: string): string | undefined {
  return new RegExp(`${name}:\\s*([^;]+);`).exec(css)?.[1].trim()
}

describe('meeting shape tokens', () => {
  it('should give the controls bar the Android xxl corner', () => {
    expect(tokenValue(meetingCss, '--meet-controls-bar-radius')).toBe('28px')
  })

  it('should make the leave button a pill', () => {
    expect(tokenValue(meetingCss, '--meet-btn-leave-radius')).toBe('9999px')
  })

  it('should tuck the chat bubble corners to the Android bubble radii', () => {
    // Android BedrudShapeTokens.chatBubble is lg everywhere, tightening to xs on the corner another
    // bubble from the same sender sits against. That tightening is what makes a run of messages read
    // as one block rather than a stack of separate cards.
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius')).toBe('16px')
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius-near')).toBe('4px')
  })

  it('should give the app gallery cards the Android card corner', () => {
    // Android BedrudShapeTokens.card, the shape for cards and selectable tiles.
    expect(tokenValue(meetingCss, '--meet-gallery-card-radius')).toBe('16px')
  })

  it('should keep the leave button pill out of every other token', () => {
    // The pill belongs to one control. While the gallery card and the near chat bubble corner
    // inherited it, taking the leave button from 3px to 9999px turned the gallery rows into stadiums
    // and rounded off the corner whose whole job is to tighten — with no test on either.
    const inheritors = [...meetingCss.matchAll(/(--meet-[\w-]+):\s*var\(--meet-btn-leave-radius\)/g)]
    expect(inheritors.map((match) => match[1])).toEqual([])
  })
})

describe('meeting panel tokens', () => {
  it('should define a scrim for the options panel', () => {
    expect(tokenValue(meetingCss, '--meet-scrim')).toBe('rgba(23, 23, 23, 0.32)')
  })

  it('should cap the options panel below the status bar', () => {
    expect(tokenValue(meetingCss, '--meet-controls-panel-max-height')).toBe(
      'calc(var(--app-height, 100svh) - 12px - env(safe-area-inset-top, 0px))',
    )
  })

  it('should darken the scrim in the dark theme', () => {
    // The light value is read first by `tokenValue`, so the dark override is checked by hand.
    expect(meetingCss).toContain('--meet-scrim: rgba(0, 0, 0, 0.32);')
  })
})
