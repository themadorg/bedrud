/** Limits the server enforces on welcome sound uploads, from public settings. */
export interface WelcomeSoundLimits {
  maxBytes: number
  maxDurationSeconds: number
}

/** Gain every client plays a welcome sound at, so a clip at the loudness ceiling stays near the join chime. */
export const WELCOME_SOUND_PLAYBACK_GAIN = 0.5

const WELCOME_SOUND_MIME = 'audio/mpeg'
const WELCOME_SOUND_EXTENSION = '.mp3'
const BYTES_PER_KB = 1024

function kilobytes(bytes: number): number {
  return Math.round(bytes / BYTES_PER_KB)
}

/** Returns the limits as one short line for the profile UI, e.g. "MP3, up to 32 KB and 7 seconds". */
export function describeWelcomeSoundLimits({ maxBytes, maxDurationSeconds }: WelcomeSoundLimits): string {
  const unit = maxDurationSeconds === 1 ? 'second' : 'seconds'
  return `MP3, up to ${kilobytes(maxBytes)} KB and ${maxDurationSeconds} ${unit}`
}

function capitalize(sentence: string): string {
  return sentence.charAt(0).toUpperCase() + sentence.slice(1)
}

/**
 * Turns a server rejection reason, written Go-style in lower case with "; " between
 * clauses, into sentences for the UI.
 */
export function formatWelcomeSoundError(message: string): string {
  return message
    .split('; ')
    .map((clause) => `${capitalize(clause.trim())}${clause.trim().endsWith('.') ? '' : '.'}`)
    .join(' ')
}

/**
 * Catches the two mistakes the browser can see before uploading. The server still checks
 * format, length and loudness, and its message is the one that decides.
 */
export function checkWelcomeSoundFile(file: File, maxBytes: number): string | null {
  const isMp3 = file.type === WELCOME_SOUND_MIME || file.name.toLowerCase().endsWith(WELCOME_SOUND_EXTENSION)
  if (!isMp3) return 'Choose an MP3 file.'
  if (file.size > maxBytes) return `The file must be ${kilobytes(maxBytes)} KB or smaller.`
  return null
}
