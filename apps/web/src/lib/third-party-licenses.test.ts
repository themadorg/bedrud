import { describe, expect, it } from 'vitest'
import { formatLicenseReport } from './third-party-licenses'

const LIVEKIT = {
  name: 'livekit-client',
  version: '2.0.0',
  license: 'Apache-2.0',
  licenseText: 'Apache License text',
}
const REACT = { name: 'react', version: '19.0.0', license: 'MIT', licenseText: 'MIT License text' }
const SHABNAM = { name: 'Shabnam', license: 'OFL-1.1', licenseText: 'SIL Open Font License text' }

describe('formatLicenseReport', () => {
  it('should list every package with its version, licence and licence text', () => {
    const report = formatLicenseReport([LIVEKIT], [])

    expect(report).toContain('livekit-client 2.0.0')
    expect(report).toContain('License: Apache-2.0')
    expect(report).toContain('Apache License text')
  })

  it('should order packages by name, whatever order the bundler found them in', () => {
    const report = formatLicenseReport([REACT, LIVEKIT], [])

    expect(report.indexOf('livekit-client')).toBeLessThan(report.indexOf('react 19.0.0'))
  })

  // Fonts are files the app ships, not dependencies the bundler can see.
  it('should list bundled assets after the packages', () => {
    const report = formatLicenseReport([LIVEKIT], [SHABNAM])

    expect(report).toContain('Shabnam')
    expect(report).toContain('SIL Open Font License text')
    expect(report.indexOf('Shabnam')).toBeGreaterThan(report.indexOf('livekit-client'))
  })

  it('should say when a package ships no licence text', () => {
    const report = formatLicenseReport([{ ...REACT, licenseText: null }], [])

    expect(report).toContain('No license text was published with this package.')
  })
})
