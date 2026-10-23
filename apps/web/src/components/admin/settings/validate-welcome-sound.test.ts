import { describe, expect, it } from 'vitest'
import { validateLocalSettings } from './shared'
import type { SystemSettings } from './types'

function settingsWith(overrides: Partial<SystemSettings>): SystemSettings {
  return {
    jwtSecret: '',
    corsAllowedOrigins: '',
    livekitHost: '',
    welcomeSoundMaxBytes: 32768,
    welcomeSoundMaxDurationSeconds: 7,
    welcomeSoundMaxPeakDbfs: -1,
    welcomeSoundMaxLoudnessLufs: -20,
    ...overrides,
  } as unknown as SystemSettings
}

describe('validateLocalSettings — welcome sound limits', () => {
  it('should accept the built-in defaults', () => {
    const errors = validateLocalSettings(settingsWith({}))
    expect(Object.keys(errors).filter((key) => key.startsWith('welcomeSound'))).toEqual([])
  })

  it('should accept zero as "use config.yaml" for every limit', () => {
    const errors = validateLocalSettings(
      settingsWith({
        welcomeSoundMaxBytes: 0,
        welcomeSoundMaxDurationSeconds: 0,
        welcomeSoundMaxPeakDbfs: 0,
        welcomeSoundMaxLoudnessLufs: 0,
      }),
    )
    expect(Object.keys(errors).filter((key) => key.startsWith('welcomeSound'))).toEqual([])
  })

  it.each([
    ['welcomeSoundMaxBytes', 1048577],
    ['welcomeSoundMaxDurationSeconds', 31],
    ['welcomeSoundMaxPeakDbfs', 1],
    ['welcomeSoundMaxPeakDbfs', -31],
    ['welcomeSoundMaxLoudnessLufs', -51],
  ] as const)('should reject %s = %d', (field, value) => {
    expect(validateLocalSettings(settingsWith({ [field]: value }))[field]).toBeDefined()
  })
})
