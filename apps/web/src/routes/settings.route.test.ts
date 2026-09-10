// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS } from '../components/settings/settingsSections'

const settingsRouteSource = readFileSync(new URL('./settings.tsx', import.meta.url), 'utf8')

describe('the settings route', () => {
  it('should build its sections from the shared model rather than its own list', () => {
    expect(settingsRouteSource).toContain('SETTINGS_SECTIONS')
  })

  it('should render every panel inline', () => {
    for (const section of SETTINGS_SECTIONS) {
      const panelName = `${section.label}SettingsPanel`
      expect(settingsRouteSource).toContain(`<${panelName} />`)
    }
  })

  it('should scroll to the section the current path names', () => {
    expect(settingsRouteSource).toContain('sectionIdFromPathname')
  })

  // `MobileOnlyGate` renders nothing until its own effect confirms the viewport. An effect placed in
  // the layout therefore runs while the sections are still absent, finds nothing, and never runs
  // again, which is exactly the defect this page shipped once. The scroll has to mount inside the
  // gate, alongside the sections it looks for. No source-reading test can observe a scroll, but it
  // can pin the structure that makes one possible.
  it('should mount the scroll inside the gate rather than beside it', () => {
    expect(settingsRouteSource).toMatch(/<MobileOnlyGate[^>]*>[\s\S]{0,200}?<SettingsSectionScroll/)
  })

  // Stated as a positive ordering claim rather than "the layout holds no effect", so that a component
  // added below the layout does not fail a test about the layout.
  it('should declare every effect above the layout component', () => {
    expect(settingsRouteSource).toMatch(/useEffect[\s\S]*function SettingsLayout\(\)/)
  })

  // The other half of the original defect was the dependency array: keyed on the pathname alone, the
  // effect could not re-run for a hash change.
  it('should key the scroll effect on both the path and the hash', () => {
    expect(settingsRouteSource).toMatch(/}, \[pathname, hash\]\)/)
  })

  it('should give every heading the id its route segment uses', () => {
    expect(settingsRouteSource).toContain('<h2 id={id}')
  })

  // The heading is the scroll target, so the scroll margin has to sit on it rather than the section.
  it('should put the scroll margin on the heading', () => {
    expect(settingsRouteSource).toMatch(/<h2 id={id} className={`scroll-mt-4 /)
  })

  it('should carry the agreed section header class', () => {
    expect(settingsRouteSource).toContain("'text-xs font-semibold uppercase tracking-wide text-primary'")
  })

  it('should no longer push to a sub-page', () => {
    expect(settingsRouteSource).not.toContain('ChevronRight')
    expect(settingsRouteSource).not.toContain('ChevronLeft')
  })
})
