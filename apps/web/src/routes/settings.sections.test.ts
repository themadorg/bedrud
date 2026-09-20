// @vitest-environment node
//
// This file reads route modules from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS } from '../components/settings/settingsSections'

/** Reads one settings sub-route module. */
function sectionRouteSource(id: string): string {
  return readFileSync(new URL(`./settings.${id}.tsx`, import.meta.url), 'utf8')
}

describe('the settings sub-routes', () => {
  it.each(SETTINGS_SECTIONS.map((section) => [section.id, section.label]))('should keep the %s title', (id, label) => {
    expect(sectionRouteSource(id)).toContain(`title: '${label} — Bedrud'`)
  })

  it.each(
    SETTINGS_SECTIONS.map((section) => [section.id, section.label]),
  )('should not render the %s panel a second time', (id, label) => {
    expect(sectionRouteSource(id)).not.toContain(`<${label}SettingsPanel`)
  })
})
