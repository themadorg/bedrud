import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { RoomSettingsDialog } from './RoomSettingsDialog'

const baseRoom = {
  id: 'room-1',
  name: 'standup',
  isPublic: true,
  maxParticipants: 20,
  settings: { allowChat: true, allowVideo: true, allowAudio: true, requireApproval: false, e2ee: false },
}

function renderDialog(settings: Partial<typeof baseRoom.settings & { allowWelcomeSounds: boolean }> = {}) {
  const onSave = vi.fn(async () => {})
  render(
    <RoomSettingsDialog
      room={{ ...baseRoom, settings: { ...baseRoom.settings, ...settings } }}
      open
      onOpenChange={() => {}}
      onSave={onSave}
    />,
  )
  return onSave
}

async function savedSettings(onSave: ReturnType<typeof vi.fn>) {
  fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
  await waitFor(() => expect(onSave).toHaveBeenCalled())
  return onSave.mock.calls[0]?.[1]?.settings
}

describe('RoomSettingsDialog welcome sounds', () => {
  it('should save welcome sounds as on for a room that never set them', async () => {
    const onSave = renderDialog()

    expect((await savedSettings(onSave)).allowWelcomeSounds).toBe(true)
  })

  it('should save welcome sounds as off after the host clicks the chip', async () => {
    const onSave = renderDialog()

    fireEvent.click(screen.getByText('Welcome'))

    expect((await savedSettings(onSave)).allowWelcomeSounds).toBe(false)
  })

  it('should keep a room that has welcome sounds off as off', async () => {
    const onSave = renderDialog({ allowWelcomeSounds: false })

    expect((await savedSettings(onSave)).allowWelcomeSounds).toBe(false)
  })
})
