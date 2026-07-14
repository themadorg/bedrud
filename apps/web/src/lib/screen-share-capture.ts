import type { ScreenShareCaptureOptions } from 'livekit-client'

/**
 * Options for getDisplayMedia when starting screen share.
 * `audio: true` surfaces Chrome's "Also share tab audio" checkbox for tab capture.
 * `systemAudio: 'include'` allows system audio among offered sources on supported browsers.
 */
export const SCREEN_SHARE_CAPTURE_OPTIONS: ScreenShareCaptureOptions = {
  audio: true,
  systemAudio: 'include',
}
