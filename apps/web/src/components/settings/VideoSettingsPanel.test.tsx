import { cleanup, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { renderSettingsPanel, switchState } from './settingsPanelTestHarness'
import { VideoSettingsPanel } from './VideoSettingsPanel'

afterEach(() => {
  cleanup()
})

describe('VideoSettingsPanel', () => {
  it('should toggle mirroring from its label', async () => {
    renderSettingsPanel(<VideoSettingsPanel />)
    const before = switchState(/Mirror my video/)

    await userEvent.click(screen.getByText('Mirror my video'))

    expect(switchState(/Mirror my video/)).not.toBe(before)
  })
})
