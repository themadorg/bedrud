import { LICENSE_REPORT_FILE } from '#/lib/third-party-licenses'

/**
 * Links to the open-source licence report the production build writes, the web's reading of the
 * Android client's "Open-source licenses" row. It sits at the foot of both settings pages. The
 * report exists only in a build, so the link answers 404 under the dev server.
 */
export function OpenSourceLicensesLink() {
  return (
    <a
      href={`/${LICENSE_REPORT_FILE}`}
      target="_blank"
      rel="noopener noreferrer"
      className="text-xs text-muted-foreground transition-colors hover:text-foreground"
    >
      Open-source licenses
    </a>
  )
}
