/** The licence report's file name, written at the root of the browser build and linked from Settings. */
export const LICENSE_REPORT_FILE = 'third-party-licenses.txt'

/** A bundled package or asset and its licence, as the licence report lists it. */
export interface LicensedWork {
  name: string | null
  version?: string | null
  license: string | null
  licenseText: string | null
}

const ENTRY_SEPARATOR = `\n\n${'-'.repeat(72)}\n\n`
const NO_LICENSE_TEXT = 'No license text was published with this package.'

/** Renders one work as a heading line, its licence identifier, then the licence text itself. */
function formatEntry(work: LicensedWork): string {
  const heading = [work.name ?? 'Unnamed package', work.version].filter(Boolean).join(' ')
  return `${heading}\nLicense: ${work.license ?? 'Unknown'}\n\n${work.licenseText?.trim() || NO_LICENSE_TEXT}`
}

/**
 * Builds the open-source licence report the app serves at `/third-party-licenses.txt`: every
 * package the bundler put in the client build, by name, then the files the app ships by hand,
 * such as its fonts, which the bundler cannot see.
 */
export function formatLicenseReport(packages: readonly LicensedWork[], bundledAssets: readonly LicensedWork[]): string {
  const sortedPackages = [...packages].sort((left, right) => (left.name ?? '').localeCompare(right.name ?? ''))
  return [...sortedPackages, ...bundledAssets].map(formatEntry).join(ENTRY_SEPARATOR)
}
