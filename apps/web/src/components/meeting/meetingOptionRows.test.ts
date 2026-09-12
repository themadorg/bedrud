import { describe, expect, it } from 'vitest'
import { isPhoneOnlyRow, type MeetingOptionsInput, meetingOptionRows } from './meetingOptionRows'

/** Everything off, so each test turns on only what it is about. */
const nothingAvailable: MeetingOptionsInput = {
  videoSidebarAvailable: false,
  videoSidebarOpen: false,
  roomAccessAvailable: false,
  isPublic: false,
  roomId: undefined,
  linkCopied: false,
  isSelfDeafened: false,
  microphones: [],
  activeMicrophoneId: undefined,
  speakers: [],
  activeSpeakerId: undefined,
  noiseModes: [],
  activeNoiseMode: 'browser',
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
  microphones: [{ deviceId: 'mic-1', label: 'Built-in microphone' }],
  activeMicrophoneId: 'mic-1',
  speakers: [{ deviceId: 'speaker-1', label: 'Built-in speaker' }],
  activeSpeakerId: 'speaker-1',
  noiseModes: [{ value: 'browser', label: 'Browser' }],
  fullscreenAvailable: true,
  whiteboardEnabled: true,
  youtubeEnabled: true,
  webxdcEnabled: true,
}

function idsOf(input: MeetingOptionsInput): string[] {
  return meetingOptionRows(input).map((row) => row.id)
}

describe('meetingOptionRows', () => {
  it('should always offer the link, deafen and settings', () => {
    expect(idsOf(nothingAvailable)).toEqual(['copy-link', 'deafen', 'settings'])
  })

  it('should order the rows room, then audio, then the app, then stage', () => {
    expect(idsOf(everythingAvailable)).toEqual([
      'videos',
      'access',
      'info',
      'copy-link',
      'deafen',
      'heading:microphone',
      'microphone:mic-1',
      'heading:speaker',
      'speaker:speaker-1',
      'heading:noise',
      'noise:browser',
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

describe('meetingOptionRows audio groups', () => {
  it('should list every microphone under one heading', () => {
    const rows = meetingOptionRows({
      ...nothingAvailable,
      microphones: [
        { deviceId: 'mic-1', label: 'Built-in microphone' },
        { deviceId: 'mic-2', label: 'Headset' },
      ],
      activeMicrophoneId: 'mic-2',
    })
    expect(rows.map((row) => row.id)).toEqual([
      'copy-link',
      'deafen',
      'heading:microphone',
      'microphone:mic-1',
      'microphone:mic-2',
      'settings',
    ])
    expect(rows.find((row) => row.id === 'microphone:mic-2')?.checked).toBe(true)
    expect(rows.find((row) => row.id === 'microphone:mic-1')?.checked).toBe(false)
  })

  it('should carry the device label so the panel never renders a raw id', () => {
    const rows = meetingOptionRows({
      ...nothingAvailable,
      speakers: [{ deviceId: 'speaker-9', label: 'External speakers' }],
      activeSpeakerId: 'speaker-9',
    })
    expect(rows.find((row) => row.id === 'speaker:speaker-9')?.label).toBe('External speakers')
  })

  it('should check the noise mode the client is using', () => {
    const rows = meetingOptionRows({
      ...nothingAvailable,
      noiseModes: [
        { value: 'browser', label: 'Browser' },
        { value: 'rnnoise', label: 'RNNoise' },
      ],
      activeNoiseMode: 'rnnoise',
    })
    expect(rows.find((row) => row.id === 'noise:rnnoise')?.checked).toBe(true)
    expect(rows.find((row) => row.id === 'noise:browser')?.checked).toBe(false)
  })

  it('should drop a heading whose group is empty rather than name nothing', () => {
    expect(idsOf(nothingAvailable)).not.toContain('heading:microphone')
    expect(idsOf(nothingAvailable)).not.toContain('heading:speaker')
    expect(idsOf(nothingAvailable)).not.toContain('heading:noise')
  })

  it('should disable a noise mode this browser cannot run', () => {
    const rows = meetingOptionRows({
      ...nothingAvailable,
      noiseModes: [
        { value: 'browser', label: 'Browser' },
        { value: 'krisp', label: 'Krisp', disabled: true },
      ],
      activeNoiseMode: 'browser',
    })
    expect(rows.find((row) => row.id === 'noise:krisp')?.disabled).toBe(true)
    expect(rows.find((row) => row.id === 'noise:browser')?.disabled).toBe(false)
  })

  it('should leave the desktop menu exactly the rows it carried before the pill', () => {
    const desktop = meetingOptionRows(everythingAvailable)
      .filter((row) => !isPhoneOnlyRow(row.id))
      .map((row) => row.id)
    expect(desktop).toEqual(['copy-link', 'settings', 'fullscreen', 'whiteboard', 'youtube', 'app-gallery'])
  })

  it('should treat every audio row as phone-only', () => {
    const audio = meetingOptionRows(everythingAvailable)
      .filter((row) => isPhoneOnlyRow(row.id))
      .map((row) => row.id)
    expect(audio).toEqual([
      'videos',
      'access',
      'info',
      'deafen',
      'heading:microphone',
      'microphone:mic-1',
      'heading:speaker',
      'speaker:speaker-1',
      'heading:noise',
      'noise:browser',
    ])
  })

  it('should mark a heading as neither a toggle nor an action', () => {
    const rows = meetingOptionRows(everythingAvailable)
    expect(rows.find((row) => row.id === 'heading:microphone')?.kind).toBe('heading')
    expect(rows.find((row) => row.id === 'microphone:mic-1')?.kind).toBe('toggle')
  })
})
