/**
 * Milliseconds between capture samples. With {@link WINDOW_SAMPLES} this makes a two-second
 * window, the same as the Android client.
 */
export const SAMPLE_INTERVAL_MS = 50

/** Samples the noise floor is the lowest of: two seconds, long enough to span a pause for breath. */
export const WINDOW_SAMPLES = 40

/** How far above the floor a level must be to count as speech: 9 dB on the 60 dB capture scale. */
export const MARGIN_ABOVE_FLOOR = 0.15

/** The highest the floor may go, -27 dBFS, so continuous loud noise cannot push speech out of reach. */
export const FLOOR_CAP = 0.55

/** Quietest level the capture scale shows; anything at or below reads as zero. */
const METER_FLOOR_DB = -60

/** Maps an RMS amplitude (0..1 full scale) onto the 0..1 capture scale, linear over 60 dB. */
export function captureLevel(rms: number): number {
  if (rms <= 0) return 0
  const levelDb = 20 * Math.log10(rms)
  return Math.min(1, Math.max(0, (levelDb - METER_FLOOR_DB) / -METER_FLOOR_DB))
}

/**
 * Decides whether a capture level is someone talking, judged against the microphone's own noise
 * floor rather than a fixed bar, which no single value holds across hardware and rooms.
 *
 * The floor is the lowest level of the last {@link WINDOW_SAMPLES} samples: it drops the moment the
 * room goes quieter and rises only once a louder noise has filled the window. Nothing is judged
 * until the window has filled once, so starting mid-sentence does not set the floor on a voice.
 */
export class SpeechLevelTracker {
  private readonly window: number[] = []

  /** Folds one sample into the floor and says whether it is speech. A null level is a stalled capture, not a lesson. */
  isSpeech(level: number | null): boolean {
    if (level === null) return false
    // A fresh capture reads silent until its stream flows; a floor of nothing would turn room noise into speech.
    if (level === 0 && this.window.length === 0) return false
    this.window.push(level)
    if (this.window.length > WINDOW_SAMPLES) this.window.shift()
    if (this.window.length < WINDOW_SAMPLES) return false
    const floor = Math.min(Math.min(...this.window), FLOOR_CAP)
    return level >= floor + MARGIN_ABOVE_FLOOR
  }
}
