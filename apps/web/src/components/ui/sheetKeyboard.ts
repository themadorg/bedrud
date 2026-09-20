/**
 * The one question about a bottom sheet that neither CSS nor vaul answers.
 *
 * Which detent a release lands on, the velocity that dismisses rather than snapping, and whether a
 * fast flick may skip a detent all belong to vaul's own drag handling. The sheet's height belongs to
 * the `--meet-sheet-max-height` token. Reimplementing either here would produce code the tests
 * exercise and the running sheet never calls.
 */

/**
 * A visible viewport shrinking by more than this is an on-screen keyboard rather than a browser
 * toolbar sliding away. Mobile Safari's toolbar is roughly 60px and Chrome's is roughly 56px, so the
 * threshold sits clear of both while staying well under the shortest keyboard.
 */
export const KEYBOARD_SHRINK_THRESHOLD_PX = 120

/**
 * Whether a viewport change should take the sheet to full. Only a shrink counts: a reader who
 * expanded to type is reading what they typed, so the sheet does not collapse when the keyboard
 * closes again.
 */
export function shouldExpandForKeyboard(previousViewportHeight: number, nextViewportHeight: number): boolean {
  return previousViewportHeight - nextViewportHeight > KEYBOARD_SHRINK_THRESHOLD_PX
}
