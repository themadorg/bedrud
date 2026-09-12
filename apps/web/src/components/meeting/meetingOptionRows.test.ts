import { describe, expect, it } from 'vitest'
import { type MeetingOptionsInput, meetingOptionRows } from './meetingOptionRows'

/** Everything off, so each test turns on only what it is about. */
const nothingAvailable: MeetingOptionsInput = {
  videoSidebarAvailable: false,
  videoSidebarOpen: false,
  roomAccessAvailable: false,
  isPublic: false,
  roomId: undefined,
  linkCopied: false,
  isSelfDeafened: false,
  fullscreenAvailable: false,
  isFullscreen: false,
  whiteboardEnabled: false,
  isWhiteboardOnStage: false,
  isWhiteboardHost: false,
  youtubeEnabled: false,
  isYoutubeOnStage: false,
  isYoutubeHost: false,
  webxdcEnabled: false,
  isWebxdcOnStage: false,
  stageTakenByOther: false,
}

/** Every capability on, for the order and the full-length cases. */
const everythingAvailable: MeetingOptionsInput = {
  ...nothingAvailable,
  videoSidebarAvailable: true,
  roomAccessAvailable: true,
  roomId: 'room-1',
  fullscreenAvailable: true,
  whiteboardEnabled: true,
  youtubeEnabled: true,
  webxdcEnabled: true,
}

function idsOf(input: MeetingOptionsInput): string[] {
  return meetingOptionRows(input).map((row) => row.id)
}

describe('meetingOptionRows', () => {
  it('should always offer the link, the audio rows and settings', () => {
    expect(idsOf(nothingAvailable)).toEqual(['copy-link', 'deafen', 'audio-devices', 'noise', 'settings'])
  })

  it('should order the rows room, then audio, then stage', () => {
    expect(idsOf(everythingAvailable)).toEqual([
      'videos',
      'access',
      'info',
      'copy-link',
      'deafen',
      'audio-devices',
      'noise',
      'settings',
      'fullscreen',
      'whiteboard',
      'youtube',
      'app-gallery',
    ])
  })

  it('should omit each optional row when its capability is absent', () => {
    expect(idsOf(nothingAvailable)).not.toContain('videos')
    expect(idsOf(nothingAvailable)).not.toContain('access')
    expect(idsOf(nothingAvailable)).not.toContain('info')
    expect(idsOf(nothingAvailable)).not.toContain('fullscreen')
    expect(idsOf(nothingAvailable)).not.toContain('whiteboard')
    expect(idsOf(nothingAvailable)).not.toContain('youtube')
    expect(idsOf(nothingAvailable)).not.toContain('app-gallery')
  })

  it('should mark deafen and videos as toggles and everything else as actions', () => {
    const byId = new Map(meetingOptionRows(everythingAvailable).map((row) => [row.id, row.kind]))
    expect(byId.get('deafen')).toBe('toggle')
    expect(byId.get('videos')).toBe('toggle')
    expect(byId.get('copy-link')).toBe('action')
    expect(byId.get('settings')).toBe('action')
    expect(byId.get('whiteboard')).toBe('action')
  })

  it('should check the deafen toggle while deafened', () => {
    const rows = meetingOptionRows({ ...nothingAvailable, isSelfDeafened: true })
    expect(rows.find((row) => row.id === 'deafen')?.checked).toBe(true)
    expect(meetingOptionRows(nothingAvailable).find((row) => row.id === 'deafen')?.checked).toBe(false)
  })

  it('should name the room access row for the state the room is in', () => {
    const publicRows = meetingOptionRows({ ...everythingAvailable, isPublic: true })
    expect(publicRows.find((row) => row.id === 'access')?.label).toBe('Public room')
    const privateRows = meetingOptionRows({ ...everythingAvailable, isPublic: false })
    expect(privateRows.find((row) => row.id === 'access')?.label).toBe('Private room')
  })

  it('should name the video row for the action it performs, not the state', () => {
    const openRows = meetingOptionRows({ ...everythingAvailable, videoSidebarOpen: true })
    expect(openRows.find((row) => row.id === 'videos')?.label).toBe('Hide videos')
    const closedRows = meetingOptionRows({ ...everythingAvailable, videoSidebarOpen: false })
    expect(closedRows.find((row) => row.id === 'videos')?.label).toBe('Show videos')
  })

  it('should say the link was copied for as long as the caller reports it', () => {
    const rows = meetingOptionRows({ ...nothingAvailable, linkCopied: true })
    expect(rows.find((row) => row.id === 'copy-link')?.label).toBe('Copied!')
  })

  it('should offer to close a stage feature it is hosting and to open one it is not', () => {
    const hosting = meetingOptionRows({
      ...everythingAvailable,
      isWhiteboardOnStage: true,
      isWhiteboardHost: true,
    })
    expect(hosting.find((row) => row.id === 'whiteboard')?.label).toBe('Close whiteboard')
    expect(meetingOptionRows(everythingAvailable).find((row) => row.id === 'whiteboard')?.label).toBe('Open whiteboard')
  })

  it('should disable a stage feature while someone else holds the stage', () => {
    const rows = meetingOptionRows({ ...everythingAvailable, stageTakenByOther: true })
    expect(rows.find((row) => row.id === 'whiteboard')?.disabled).toBe(true)
    expect(rows.find((row) => row.id === 'youtube')?.disabled).toBe(true)
    expect(rows.find((row) => row.id === 'copy-link')?.disabled).toBe(false)
  })
})
