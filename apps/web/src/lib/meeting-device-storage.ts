export const MEETING_DEVICE_STORAGE_KEYS = {
  audioinput: 'bedrud_mic_device',
  videoinput: 'bedrud_cam_device',
  audiooutput: 'bedrud_speaker_device',
} as const

export type MeetingDeviceKind = keyof typeof MEETING_DEVICE_STORAGE_KEYS

export const DEFAULT_DEVICE_SELECT_VALUE = '__bedrud_default_device__'

export function deviceIdToSelectValue(deviceId: string): string {
  return deviceId || DEFAULT_DEVICE_SELECT_VALUE
}

export function selectValueToDeviceId(value: string): string {
  return value === DEFAULT_DEVICE_SELECT_VALUE ? '' : value
}

export function readMeetingDeviceId(kind: MeetingDeviceKind): string {
  if (typeof localStorage === 'undefined') return ''
  return localStorage.getItem(MEETING_DEVICE_STORAGE_KEYS[kind]) ?? ''
}

export function writeMeetingDeviceId(kind: MeetingDeviceKind, deviceId: string) {
  localStorage.setItem(MEETING_DEVICE_STORAGE_KEYS[kind], deviceId)
}
