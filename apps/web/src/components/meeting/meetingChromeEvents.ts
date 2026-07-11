/** Lightweight bus so stage chrome (e.g. WebXDC left rail) can open meeting panels. */

export const MEETING_OPEN_CHAT = 'bedrud:meeting-open-chat'
export const MEETING_OPEN_SETTINGS = 'bedrud:meeting-open-settings'
export const MEETING_OPEN_ROOM_INFO = 'bedrud:meeting-open-room-info'

/** Broadcast which elevated left-rail panel is active (for rail icon glow). */
export const MEETING_CHROME_STATE = 'bedrud:meeting-chrome-state'

/** Close settings without opening another panel. */
export const MEETING_CLOSE_SETTINGS = 'bedrud:meeting-close-settings'

/**
 * Close all elevated left docks (chat / settings / info) opened from WebXDC expand.
 * Fired when expand collapses so panels do not linger over the meeting.
 */
export const MEETING_CLOSE_ELEVATED_CHROME = 'bedrud:meeting-close-elevated-chrome'

export type MeetingChromePanel = 'chat' | 'settings' | 'info' | null

export type MeetingChromeOpenDetail = {
  /** Opened from expanded WebXDC — keep app open; dock panels on the left above it. */
  source?: 'webxdc-expand'
}

export type MeetingChromeStateDetail = {
  panel: MeetingChromePanel
}

export function requestOpenMeetingChat(detail?: MeetingChromeOpenDetail) {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_OPEN_CHAT, { detail: detail ?? {} }))
}

export function requestOpenMeetingSettings(detail?: MeetingChromeOpenDetail) {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_OPEN_SETTINGS, { detail: detail ?? {} }))
}

export function requestOpenMeetingRoomInfo(detail?: MeetingChromeOpenDetail) {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_OPEN_ROOM_INFO, { detail: detail ?? {} }))
}

export function requestCloseMeetingSettings() {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_CLOSE_SETTINGS))
}

export function requestCloseElevatedChrome() {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_CLOSE_ELEVATED_CHROME))
}

export function publishMeetingChromeState(panel: MeetingChromePanel) {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEETING_CHROME_STATE, { detail: { panel } satisfies MeetingChromeStateDetail }))
}

export function isWebxdcExpandSource(detail: unknown): boolean {
  return (
    !!detail &&
    typeof detail === 'object' &&
    (detail as MeetingChromeOpenDetail).source === 'webxdc-expand'
  )
}
