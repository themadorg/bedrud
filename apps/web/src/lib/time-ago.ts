import { defaultLocale } from '#/lib/i18n'

const MILLISECONDS_PER_MINUTE = 60_000
const MINUTES_PER_HOUR = 60
const HOURS_PER_DAY = 24

// Narrow reads "5m ago" in English, the compact form the dense lists that use it need. `always`
// keeps "1d ago" rather than "yesterday", so a day lines up with its neighbours.
const relativeTime = new Intl.RelativeTimeFormat(defaultLocale, { style: 'narrow', numeric: 'always' })

/**
 * Formats how long ago a moment was, in whole minutes, hours or days, in the app's language.
 * `now` is a parameter so the label can be tested without faking the clock.
 */
export function timeAgo(timestamp: number, now: number = Date.now()): string {
  const minutes = Math.floor((now - timestamp) / MILLISECONDS_PER_MINUTE)
  if (minutes < 1) return 'just now'
  if (minutes < MINUTES_PER_HOUR) return relativeTime.format(-minutes, 'minute')

  const hours = Math.floor(minutes / MINUTES_PER_HOUR)
  if (hours < HOURS_PER_DAY) return relativeTime.format(-hours, 'hour')

  return relativeTime.format(-Math.floor(hours / HOURS_PER_DAY), 'day')
}
