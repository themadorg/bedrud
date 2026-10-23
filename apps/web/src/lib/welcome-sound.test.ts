import { describe, expect, it } from 'vitest'
import { checkWelcomeSoundFile, describeWelcomeSoundLimits, formatWelcomeSoundError } from './welcome-sound'

describe('formatWelcomeSoundError', () => {
  it('should turn a server reason into sentences', () => {
    expect(formatWelcomeSoundError('sound is too loud; lower the volume and export it again')).toBe(
      'Sound is too loud. Lower the volume and export it again.',
    )
  })

  it('should leave a finished sentence as it is', () => {
    expect(formatWelcomeSoundError('Welcome sounds are turned off on this server')).toBe(
      'Welcome sounds are turned off on this server.',
    )
  })

  it('should keep the decimal point in a sample rate list', () => {
    expect(formatWelcomeSoundError('export the sound at 16, 24, 32, 44.1 or 48 kHz')).toBe(
      'Export the sound at 16, 24, 32, 44.1 or 48 kHz.',
    )
  })
})

describe('describeWelcomeSoundLimits', () => {
  it('should state the format, size in KB and duration in seconds', () => {
    expect(describeWelcomeSoundLimits({ maxBytes: 32 * 1024, maxDurationSeconds: 7 })).toBe(
      'MP3, up to 32 KB and 7 seconds',
    )
  })

  it('should use the singular for a one-second limit', () => {
    expect(describeWelcomeSoundLimits({ maxBytes: 20 * 1024, maxDurationSeconds: 1 })).toBe(
      'MP3, up to 20 KB and 1 second',
    )
  })

  it('should round a size that is not a whole number of KB', () => {
    expect(describeWelcomeSoundLimits({ maxBytes: 20_000, maxDurationSeconds: 5 })).toBe(
      'MP3, up to 20 KB and 5 seconds',
    )
  })
})

describe('checkWelcomeSoundFile', () => {
  const limit = 32 * 1024

  it('should accept an MP3 within the size limit', () => {
    const file = new File([new Uint8Array(limit)], 'hello.mp3', { type: 'audio/mpeg' })
    expect(checkWelcomeSoundFile(file, limit)).toBeNull()
  })

  it('should accept an .mp3 file the browser gave no type', () => {
    const file = new File([new Uint8Array(10)], 'hello.mp3', { type: '' })
    expect(checkWelcomeSoundFile(file, limit)).toBeNull()
  })

  it('should reject a file that is not an MP3', () => {
    const file = new File([new Uint8Array(10)], 'hello.wav', { type: 'audio/wav' })
    expect(checkWelcomeSoundFile(file, limit)).toBe('Choose an MP3 file.')
  })

  it('should reject a file one byte over the limit', () => {
    const file = new File([new Uint8Array(limit + 1)], 'hello.mp3', { type: 'audio/mpeg' })
    expect(checkWelcomeSoundFile(file, limit)).toBe('The file must be 32 KB or smaller.')
  })
})
