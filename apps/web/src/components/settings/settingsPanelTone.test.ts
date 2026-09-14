// The helper returns a panel surface, the same shape `admin/settings/shared.tsx` builds, so both
// tones owe the panel corner. The class string is the only place that corner can come from: no
// stylesheet rule targets these panels.

import { describe, expect, it } from 'vitest'
import { panelSurfaceClass, type SettingsPanelTone } from './settingsPanelTone'

const tones: SettingsPanelTone[] = ['default', 'meeting']

describe('panelSurfaceClass', () => {
  it.each(tones)('should give the %s tone the panel corner', (tone) => {
    expect(panelSurfaceClass(tone).split(' ')).toContain('rounded-xl')
  })
})
