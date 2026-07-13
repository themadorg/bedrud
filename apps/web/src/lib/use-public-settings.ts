// TODO oncoming feature
import { api } from '#/lib/api'

export interface PublicSettings {
  serverName: string
  registrationEnabled: boolean
  tokenRegistrationOnly: boolean
  guestLoginEnabled: boolean
  passkeysEnabled: boolean
  oauthProviders: string[]
  requireEmailVerification: boolean
  chatMaxMessageCount: number
  chatMessageTTLHours: number
  /** Hard per-file chat image limit (bytes). Default 10 MiB. */
  chatUploadMaxBytes: number
  /** Max image width/height in pixels. Default 8192. */
  chatUploadMaxDimension: number
  // TODO oncoming feature - always disabled
  recordingsEnabled: boolean
  /** Admin enabled RNNoise for this instance (users may then select it). Default false. */
  rnnoiseEnabled?: boolean
  /** Admin enabled Krisp for this instance (users may then select it). Default false. */
  krispEnabled?: boolean
}

let cached: PublicSettings | null = null
let promise: Promise<PublicSettings> | null = null

export function getPublicSettings(): Promise<PublicSettings> {
  if (cached) return Promise.resolve(cached)
  if (promise) return promise
  promise = api.get<PublicSettings>('/api/auth/settings').then((s: PublicSettings) => {
    cached = s
    return s
  })
  return promise
}

export function refreshPublicSettings() {
  cached = null
  promise = null
}

export function usePublicSettings() {
  return { get: getPublicSettings, refresh: refreshPublicSettings }
}
