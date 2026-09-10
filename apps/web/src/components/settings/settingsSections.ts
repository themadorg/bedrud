export type SettingsSectionId = 'appearance' | 'audio' | 'video' | 'security' | 'experimental'

export interface SettingsSection {
  id: SettingsSectionId
  label: string
  /** The canonical URL for this section, for an external link or a bookmark. The page itself maps over ids. */
  route: `/settings/${SettingsSectionId}`
}

/**
 * The settings sections in the order the page renders them. Each id is also the section's route
 * segment and the `id` attribute its heading carries, so a link to `/settings/audio` and an anchor
 * to `/settings#audio` reach the same place.
 */
export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { id: 'appearance', label: 'Appearance', route: '/settings/appearance' },
  { id: 'audio', label: 'Audio', route: '/settings/audio' },
  { id: 'video', label: 'Video', route: '/settings/video' },
  { id: 'security', label: 'Security', route: '/settings/security' },
  { id: 'experimental', label: 'Experimental', route: '/settings/experimental' },
]

const SETTINGS_PATH_PREFIX = '/settings/'

/** Reports whether a string names one of the settings sections. */
function isSettingsSectionId(value: string): value is SettingsSectionId {
  return SETTINGS_SECTIONS.some((candidate) => candidate.id === value)
}

/** Returns the section a settings sub-route points at, or null for the index and anything else. */
export function sectionIdFromPathname(pathname: string): SettingsSectionId | null {
  if (!pathname.startsWith(SETTINGS_PATH_PREFIX)) return null
  const segment = pathname.slice(SETTINGS_PATH_PREFIX.length).replace(/\/$/, '')
  return isSettingsSectionId(segment) ? segment : null
}

/**
 * Returns the section a hash names, or null when the hash is not a section. The input is the
 * router's `location.hash`, which arrives with its `#` already stripped; a leading `#` is tolerated
 * so a raw `window.location.hash` works too.
 */
export function sectionIdFromHash(hash: string): SettingsSectionId | null {
  const segment = hash.replace(/^#/, '')
  return isSettingsSectionId(segment) ? segment : null
}
