import { cleanup, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { AppearanceSettingsPanel } from './AppearanceSettingsPanel'
import { renderSettingsPanel, switchState } from './settingsPanelTestHarness'

afterEach(() => {
  cleanup()
})

describe('AppearanceSettingsPanel', () => {
  it('should toggle the welcome screen from its label', async () => {
    renderSettingsPanel(<AppearanceSettingsPanel />)
    const before = switchState(/Welcome screen/)

    await userEvent.click(screen.getByText('Welcome screen'))

    expect(switchState(/Welcome screen/)).not.toBe(before)
  })
})
