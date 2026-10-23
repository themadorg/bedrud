import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { RoomSettingsDialog } from './RoomSettingsDialog'

const room = {
  id: 'room-1',
  name: 'standup',
  isPublic: false,
  maxParticipants: 10,
  settings: { allowChat: true, allowVideo: true, allowAudio: true, requireApproval: false, e2ee: false },
}

describe('RoomSettingsDialog', () => {
  afterEach(cleanup)

  // Android #220: the dialog could be dismissed with the save still on its way.
  it('should stay open on Escape while the save is on its way', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(<RoomSettingsDialog room={room} open onOpenChange={onOpenChange} onSave={() => new Promise(() => {})} />)

    await user.click(screen.getByRole('button', { name: /Save changes/ }))
    await user.keyboard('{Escape}')

    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('should close on Escape when nothing is saving', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(<RoomSettingsDialog room={room} open onOpenChange={onOpenChange} onSave={() => new Promise(() => {})} />)

    await user.keyboard('{Escape}')

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
