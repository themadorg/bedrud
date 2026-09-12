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

  it('should leave the chat bubble radii following the two above', () => {
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius')).toBe('var(--meet-controls-bar-radius)')
    expect(tokenValue(meetingCss, '--meet-chat-bubble-radius-near')).toBe('var(--meet-btn-leave-radius)')
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
