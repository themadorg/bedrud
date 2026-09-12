// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const chatPanelSource = readFileSync(new URL('./ChatPanel.tsx', import.meta.url), 'utf8')
const elevatedDockSource = readFileSync(new URL('./MeetingElevatedLeftDock.tsx', import.meta.url), 'utf8')
const meetingCss = readFileSync(new URL('./meeting.css', import.meta.url), 'utf8')

/**
 * Every chat marker the meeting stylesheet selects on, against the file that renders it. The
 * elevated marker is written by the dock rather than by the panel, so pinning it to `ChatPanel`
 * would assert something that was never true.
 */
const CHAT_MARKERS: ReadonlyArray<[marker: string, source: string]> = [
  ['data-chat-overlay', chatPanelSource],
  ['aria-label="Chat"', chatPanelSource],
  ['data-elevated-chat', elevatedDockSource],
]

describe('the chat markers the meeting stylesheet depends on', () => {
  it.each(CHAT_MARKERS)('should still be rendered: %s', (marker, source) => {
    expect(source).toContain(marker)
  })

  it('should be selected on an element type the sheet actually renders', () => {
    // vaul renders a div, not an aside. A selector list that names only `aside` would stop
    // matching with no error and chat text would fall back to raw `text-white/*`.
    const asideOnly = /aside\[data-chat-overlay="true"\]/g
    const sheetSelector = /\[data-chat-overlay="true"\]/g
    expect(meetingCss.match(sheetSelector)?.length).toBeGreaterThan(meetingCss.match(asideOnly)?.length ?? 0)
  })
})

describe('the phone chat surface', () => {
  it('should no longer carry a meeting controls strip of its own', () => {
    expect(chatPanelSource).not.toContain('mobileMeetingBar')
  })

  it('should choose its surface through the named helper', () => {
    expect(chatPanelSource).toContain('chatSurfaceFor')
  })

  it('should keep a close button on the sheet, which renders the panel body rather than bare chat', () => {
    expect(chatPanelSource).toMatch(/<BedrudSheet[\s\S]*?\{body\}[\s\S]*?<\/BedrudSheet>/)
  })

  it('should drop the participants callbacks the deleted strip was the only caller of', () => {
    expect(chatPanelSource).not.toContain('onOpenParticipantsFromChat')
    expect(chatPanelSource).not.toContain('onCloseParticipants')
  })
})
