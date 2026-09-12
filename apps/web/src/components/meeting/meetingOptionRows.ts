/** Every row the options panel can show. The union is the contract the panel maps to icons. */
export type MeetingOptionRowId =
  | 'videos'
  | 'access'
  | 'info'
  | 'copy-link'
  | 'deafen'
  | 'audio-devices'
  | 'noise'
  | 'settings'
  | 'fullscreen'
  | 'whiteboard'
  | 'youtube'
  | 'app-gallery'

export interface MeetingOptionRow {
  id: MeetingOptionRowId
  label: string
  /** Toggles keep the panel open and carry a check. Actions close it on the way. */
  kind: 'toggle' | 'action'
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

/** Fills in the two fields most rows do not care about. */
function action(id: MeetingOptionRowId, label: string, disabled = false): MeetingOptionRow {
  return { id, label, kind: 'action', checked: false, disabled }
}

/** A row whose tap flips a setting in place rather than going somewhere. */
function toggle(id: MeetingOptionRowId, label: string, checked: boolean): MeetingOptionRow {
  return { id, label, kind: 'toggle', checked, disabled: false }
}

/**
 * Builds the options panel's rows from what this room and this client can do. The order — room,
 * then audio, then whatever is on the stage — is part of the contract: the panel is anchored to
 * the controls and read bottom-up, so the rows nearest the thumb are the ones about the room the
 * user is in.
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
  rows.push(action('audio-devices', 'Audio settings'))
  rows.push(action('noise', 'Noise suppression'))
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
