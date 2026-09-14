// The settings page renders one section per entry, in this order, and anchors each by its id.
// The ids double as the route segments under `/settings`, so the mapping below is what keeps a
// bookmarked sub-route pointing at the right section.

import { describe, expect, it } from 'vitest'
import { SETTINGS_SECTIONS, sectionIdFromHash, sectionIdFromPathname } from './settingsSections'

describe('SETTINGS_SECTIONS', () => {
  it('should list the five sections in display order', () => {
    expect(SETTINGS_SECTIONS.map((section) => section.id)).toEqual([
      'appearance',
      'audio',
      'video',
      'security',
      'experimental',
    ])
  })

  it('should give every section a route built from its id', () => {
    for (const section of SETTINGS_SECTIONS) {
      expect(section.route).toBe(`/settings/${section.id}`)
    }
  })

  it('should give every section a non-empty label', () => {
    for (const section of SETTINGS_SECTIONS) {
      expect(section.label.length).toBeGreaterThan(0)
    }
  })
})

describe('sectionIdFromPathname', () => {
  it.each([
    'appearance',
    'audio',
    'video',
    'security',
    'experimental',
  ] as const)('should read %s from its sub-route', (id) => {
    expect(sectionIdFromPathname(`/settings/${id}`)).toBe(id)
  })

  it('should ignore a trailing slash', () => {
    expect(sectionIdFromPathname('/settings/audio/')).toBe('audio')
  })

  it('should return null on the settings index', () => {
    expect(sectionIdFromPathname('/settings')).toBeNull()
    expect(sectionIdFromPathname('/settings/')).toBeNull()
  })

  it('should return null for a segment that is not a section', () => {
    expect(sectionIdFromPathname('/settings/nonsense')).toBeNull()
  })

  it('should return null for a path outside settings', () => {
    expect(sectionIdFromPathname('/dashboard/settings/audio')).toBeNull()
  })
})

describe('sectionIdFromHash', () => {
  it.each([
    'appearance',
    'audio',
    'video',
    'security',
    'experimental',
  ] as const)('should read %s from its hash', (id) => {
    expect(sectionIdFromHash(`#${id}`)).toBe(id)
  })

  it('should accept a hash without its leading marker', () => {
    expect(sectionIdFromHash('video')).toBe('video')
  })

  it('should return null for an empty hash', () => {
    expect(sectionIdFromHash('')).toBeNull()
    expect(sectionIdFromHash('#')).toBeNull()
  })

  it('should return null for an element that is not a section', () => {
    expect(sectionIdFromHash('#settings-email')).toBeNull()
  })
})
