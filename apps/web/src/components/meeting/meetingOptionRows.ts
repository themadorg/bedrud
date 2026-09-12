/** A selectable audio device, as the panel needs to know it. */
export interface MeetingDeviceOption {
  deviceId: string
  label: string
}

/** A noise suppression mode the instance allows. */
export interface MeetingNoiseModeOption {
  value: string
  label: string
  /** True when the instance offers the mode but this browser cannot run it. */
  disabled?: boolean
}

/** The rows whose identity is fixed. Device rows carry the device id instead. */
export type MeetingFixedRowId =
  | 'videos'
  | 'access'
  | 'info'
  | 'copy-link'
  | 'deafen'
  | 'settings'
  | 'fullscreen'
  | 'whiteboard'
  | 'youtube'
  | 'app-gallery'
  | 'heading:microphone'
  | 'heading:speaker'
  | 'heading:noise'

/**
 * Every row the options panel can show. Audio devices cannot be enumerated ahead of time, so their
 * ids carry the device they select; the prefix is what the panel switches on for an icon.
 */
export type MeetingOptionRowId = MeetingFixedRowId | `microphone:${string}` | `speaker:${string}` | `noise:${string}`

export interface MeetingOptionRow {
  id: MeetingOptionRowId
  label: string
  /**
   * Toggles keep the panel open and carry a check. Actions close it on the way. Headings are not
   * interactive and only name the group beneath them.
   */
  kind: 'toggle' | 'action' | 'heading'
  checked: boolean
  disabled: boolean
}

export interface MeetingOptionsInput {
  videoSidebarAvailable: boolean
  videoSidebarOpen: boolean
  roomAccessAvailable: boolean
  isPublic: boolean
  roomId: string | undefined
  linkCopied: boolean
  isSelfDeafened: boolean
  microphones: MeetingDeviceOption[]
  activeMicrophoneId: string | undefined
  speakers: MeetingDeviceOption[]
  activeSpeakerId: string | undefined
  noiseModes: MeetingNoiseModeOption[]
  activeNoiseMode: string
  fullscreenAvailable: boolean
  isFullscreen: boolean
  whiteboardEnabled: boolean
  isWhiteboardOnStage: boolean
  isWhiteboardHost: boolean
  youtubeEnabled: boolean
  isYoutubeOnStage: boolean
  isYoutubeHost: boolean
  webxdcEnabled: boolean
  isWebxdcOnStage: boolean
  stageTakenByOther: boolean
}

/** True for the rows that make up the three audio groups, headings included. */
function isAudioRow(id: MeetingOptionRowId): boolean {
  return (
    id === 'heading:microphone' ||
    id === 'heading:speaker' ||
    id === 'heading:noise' ||
    id.startsWith('microphone:') ||
    id.startsWith('speaker:') ||
    id.startsWith('noise:')
  )
}

/** Rows the desktop bar already exposes somewhere else, and the control that exposes them. */
const DESKTOP_CONTROLS_ELSEWHERE: ReadonlyArray<[MeetingFixedRowId, string]> = [
  ['videos', 'the video sidebar toggle in the left chrome'],
  ['access', 'the RoomAccessBadge in the left chrome'],
  ['info', 'the room info button in the meeting header'],
  ['deafen', 'its own button in the controls bar'],
]

/**
 * True for a row the phone panel must carry but the desktop `⋯` menu must not.
 *
 * The phone panel is the only surface that has the audio devices: it replaced a full-screen dialog,
 * so they have nowhere else to live. The desktop keeps a separate audio menu beside `⋯`, and the
 * four rows above each have their own desktop control — listing them again in `⋯` is the
 * duplication this unit removes elsewhere.
 */
export function isPhoneOnlyRow(id: MeetingOptionRowId): boolean {
  return isAudioRow(id) || DESKTOP_CONTROLS_ELSEWHERE.some(([rowId]) => rowId === id)
}

/** Fills in the two fields most rows do not care about. */
function action(id: MeetingOptionRowId, label: string, disabled = false): MeetingOptionRow {
  return { id, label, kind: 'action', checked: false, disabled }
}

/** A row whose tap flips or selects something in place rather than going somewhere. */
function toggle(id: MeetingOptionRowId, label: string, checked: boolean): MeetingOptionRow {
  return { id, label, kind: 'toggle', checked, disabled: false }
}

/** A non-interactive label naming the group of rows beneath it. */
function heading(id: MeetingFixedRowId, label: string): MeetingOptionRow {
  return { id, label, kind: 'heading', checked: false, disabled: false }
}

/**
 * Appends one audio group: its heading, then one selectable row per entry. An empty group
 * contributes nothing, heading included — a section naming devices that do not exist reads as a
 * bug rather than as an empty state.
 */
function pushAudioGroup(
  rows: MeetingOptionRow[],
  headingId: MeetingFixedRowId,
  headingLabel: string,
  entries: { id: MeetingOptionRowId; label: string; checked: boolean; disabled?: boolean }[],
): void {
  if (entries.length === 0) return
  rows.push(heading(headingId, headingLabel))
  for (const entry of entries) {
    rows.push({ ...toggle(entry.id, entry.label, entry.checked), disabled: entry.disabled ?? false })
  }
}

/**
 * Builds the options panel's rows from what this room and this client can do. The order — room,
 * then audio, then the app, then whatever is on the stage — is part of the contract: the panel is
 * anchored to the controls and read bottom-up, so the rows nearest the thumb are the ones about
 * the room the user is in.
 *
 * The audio devices are rows rather than a link to a second surface. They used to live in a
 * full-screen dialog that covered the call, which is the shape this unit exists to remove.
 */
export function meetingOptionRows(input: MeetingOptionsInput): MeetingOptionRow[] {
  const rows: MeetingOptionRow[] = []

  if (input.videoSidebarAvailable) {
    rows.push(toggle('videos', input.videoSidebarOpen ? 'Hide videos' : 'Show videos', input.videoSidebarOpen))
  }
  if (input.roomAccessAvailable) {
    rows.push(action('access', input.isPublic ? 'Public room' : 'Private room'))
  }
  if (input.roomId) {
    rows.push(action('info', 'Room info'))
  }

  rows.push(action('copy-link', input.linkCopied ? 'Copied!' : 'Copy room link'))
  rows.push(toggle('deafen', input.isSelfDeafened ? 'Undeafen' : 'Deafen', input.isSelfDeafened))

  pushAudioGroup(
    rows,
    'heading:microphone',
    'Microphone',
    input.microphones.map((device) => ({
      id: `microphone:${device.deviceId}` as const,
      label: device.label,
      checked: device.deviceId === input.activeMicrophoneId,
    })),
  )
  pushAudioGroup(
    rows,
    'heading:speaker',
    'Speaker',
    input.speakers.map((device) => ({
      id: `speaker:${device.deviceId}` as const,
      label: device.label,
      checked: device.deviceId === input.activeSpeakerId,
    })),
  )
  pushAudioGroup(
    rows,
    'heading:noise',
    'Noise suppression',
    input.noiseModes.map((mode) => ({
      id: `noise:${mode.value}` as const,
      label: mode.label,
      checked: mode.value === input.activeNoiseMode,
      disabled: mode.disabled,
    })),
  )

  rows.push(action('settings', 'Settings'))

  if (input.fullscreenAvailable) {
    rows.push(action('fullscreen', input.isFullscreen ? 'Exit fullscreen' : 'Fullscreen'))
  }

  const hostsWhiteboard = input.isWhiteboardOnStage && input.isWhiteboardHost
  if (hostsWhiteboard) {
    rows.push(action('whiteboard', 'Close whiteboard'))
  } else if (input.whiteboardEnabled) {
    rows.push(
      action(
        'whiteboard',
        input.isWhiteboardOnStage ? 'Whiteboard on stage' : 'Open whiteboard',
        input.stageTakenByOther,
      ),
    )
  }

  const hostsYoutube = input.isYoutubeOnStage && input.isYoutubeHost
  if (hostsYoutube) {
    rows.push(action('youtube', 'Stop YouTube'))
  } else if (input.youtubeEnabled) {
    rows.push(action('youtube', input.isYoutubeOnStage ? 'YouTube on stage' : 'Share YouTube', input.stageTakenByOther))
  }

  if (input.webxdcEnabled) {
    rows.push(action('app-gallery', input.isWebxdcOnStage ? 'Gallery (on stage)' : 'App gallery'))
  }

  return rows
}
