import { describe, expect, it } from 'vitest'
import { MASKED_SECRET, validateLocalSettings } from './shared'
import type { SystemSettings } from './types'

// The API returns MASKED_SECRET (8 chars) instead of the stored jwtSecret, so a
// naive length check flagged it and blocked every save on the Authentication tab.
function settingsWithJwt(jwtSecret: string): SystemSettings {
  return { jwtSecret, corsAllowedOrigins: '', livekitHost: '' } as unknown as SystemSettings
}

describe('validateLocalSettings — jwtSecret', () => {
  it('accepts the masked placeholder', () => {
    expect(validateLocalSettings(settingsWithJwt(MASKED_SECRET)).jwtSecret).toBeUndefined()
  })

  it('still rejects a real secret that is too short', () => {
    expect(validateLocalSettings(settingsWithJwt('too-short')).jwtSecret).toBeDefined()
  })
})
