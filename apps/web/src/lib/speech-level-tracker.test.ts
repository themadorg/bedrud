// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { captureLevel, FLOOR_CAP, MARGIN_ABOVE_FLOOR, SpeechLevelTracker, WINDOW_SAMPLES } from './speech-level-tracker'

function filledTracker(level: number): SpeechLevelTracker {
  const tracker = new SpeechLevelTracker()
  for (let index = 0; index < WINDOW_SAMPLES - 1; index++) tracker.isSpeech(level)
  return tracker
}

describe('captureLevel', () => {
  it('should map full scale to 1 and -60 dBFS or quieter to 0', () => {
    expect(captureLevel(1)).toBe(1)
    expect(captureLevel(0.001)).toBe(0)
    expect(captureLevel(0)).toBe(0)
  })

  it('should map -27 dBFS onto the floor cap', () => {
    expect(captureLevel(10 ** (-27 / 20))).toBeCloseTo(FLOOR_CAP, 5)
  })
})

describe('SpeechLevelTracker', () => {
  describe('isSpeech', () => {
    it('should judge nothing until the window has filled once', () => {
      const tracker = new SpeechLevelTracker()
      for (let index = 0; index < WINDOW_SAMPLES - 1; index++) expect(tracker.isSpeech(0.9)).toBe(false)
    })

    it('should count a level 9 dB above the floor as speech', () => {
      expect(filledTracker(0.1).isSpeech(0.1 + MARGIN_ABOVE_FLOOR)).toBe(true)
    })

    it('should not count a level just under 9 dB above the floor as speech', () => {
      expect(filledTracker(0.1).isSpeech(0.1 + MARGIN_ABOVE_FLOOR - 0.01)).toBe(false)
    })

    it('should hear quiet speech over a quiet room that the old fixed bar missed', () => {
      expect(filledTracker(0.02).isSpeech(0.2)).toBe(true)
    })

    it('should cap the floor so continuous loud noise cannot push speech out of reach', () => {
      expect(filledTracker(0.9).isSpeech(FLOOR_CAP + MARGIN_ABOVE_FLOOR)).toBe(true)
    })

    it('should drop the floor at once when the room goes quieter', () => {
      const tracker = filledTracker(0.4)
      tracker.isSpeech(0.4)
      tracker.isSpeech(0.05)
      expect(tracker.isSpeech(0.25)).toBe(true)
    })

    it('should raise the floor only after a louder noise has filled the window', () => {
      const tracker = filledTracker(0.1)
      tracker.isSpeech(0.1)
      for (let index = 0; index < WINDOW_SAMPLES - 1; index++) expect(tracker.isSpeech(0.3)).toBe(true)
      expect(tracker.isSpeech(0.3)).toBe(false)
    })

    it('should not learn from the silent frames before capture starts flowing', () => {
      const tracker = new SpeechLevelTracker()
      tracker.isSpeech(0)
      for (let index = 0; index < WINDOW_SAMPLES - 2; index++) tracker.isSpeech(0.17)
      expect(tracker.isSpeech(0.17)).toBe(false)
    })

    it('should still learn a true zero once capture has started', () => {
      const tracker = filledTracker(0.17)
      tracker.isSpeech(0)
      expect(tracker.isSpeech(0.17)).toBe(true)
    })

    it('should treat a stalled capture as silence without learning from it', () => {
      const tracker = filledTracker(0.2)
      tracker.isSpeech(0.2)
      expect(tracker.isSpeech(null)).toBe(false)
      expect(tracker.isSpeech(0.3)).toBe(false)
    })
  })
})
