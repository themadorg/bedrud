import { cleanup, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { ExperimentalSettingsPanel } from './ExperimentalSettingsPanel'
import { renderSettingsPanel, switchState } from './settingsPanelTestHarness'

afterEach(() => {
  cleanup()
})

describe('ExperimentalSettingsPanel', () => {
  it.each(['Shared whiteboard', 'YouTube watch party'])('should toggle %s from its label', async (title) => {
    renderSettingsPanel(<ExperimentalSettingsPanel />)
    const name = new RegExp(title)
    const before = switchState(name)

    await userEvent.click(screen.getByText(title))

    expect(switchState(name)).not.toBe(before)
  })

  // The server reports no WebXDC support in tests, so the switch is disabled; its label still names it.
  it('should name the WebXDC switch by its label', () => {
    renderSettingsPanel(<ExperimentalSettingsPanel />)

    expect(screen.getByRole('switch', { name: /WebXDC mini-apps/ })).toBeTruthy()
  })
})
