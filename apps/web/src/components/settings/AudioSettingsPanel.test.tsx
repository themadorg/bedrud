import { cleanup, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { AudioSettingsPanel } from './AudioSettingsPanel'
import { renderSettingsPanel, switchState } from './settingsPanelTestHarness'

afterEach(() => {
  cleanup()
})

describe('AudioSettingsPanel', () => {
  it.each([
    'Echo cancellation',
    'Enable push to talk',
    'Enable reminder',
  ])('should toggle %s from its label', async (title) => {
    renderSettingsPanel(<AudioSettingsPanel />)
    const name = new RegExp(title)
    const before = switchState(name)

    await userEvent.click(screen.getByText(title))

    expect(switchState(name)).not.toBe(before)
  })
})
