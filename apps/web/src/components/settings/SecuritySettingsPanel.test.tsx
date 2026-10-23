import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { changePasswordAndSignBackIn } = vi.hoisted(() => ({ changePasswordAndSignBackIn: vi.fn() }))

vi.mock('#/lib/change-password', () => ({ changePasswordAndSignBackIn }))

import { useAuthStore } from '#/lib/auth.store'
import { type ServerUser, toUser, useUserStore } from '#/lib/user.store'
import { SecuritySettingsPanel } from './SecuritySettingsPanel'

const CURRENT_PASSWORD = 'old-password-12'
const NEW_PASSWORD = 'new-password-12'

const localUser: ServerUser = {
  id: 'user-1',
  email: 'someone@example.test',
  name: 'Someone',
  provider: 'local',
  accesses: ['user'],
  passwordChangedAt: null,
}

async function submitPasswordChange() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Current password'), CURRENT_PASSWORD)
  await user.type(screen.getByLabelText('New password'), NEW_PASSWORD)
  await user.type(screen.getByLabelText('Confirm new password'), NEW_PASSWORD)
  await user.click(screen.getByRole('button', { name: /password/i }))
}

describe('SecuritySettingsPanel', () => {
  beforeEach(() => {
    changePasswordAndSignBackIn.mockReset()
    useUserStore.getState().setUser(toUser(localUser))
  })

  afterEach(() => {
    cleanup()
    useUserStore.getState().clear()
  })

  describe('after a password change', () => {
    it('should store the fresh session and report success', async () => {
      const stamp = '2026-10-20T10:00:00Z'
      changePasswordAndSignBackIn.mockResolvedValue({
        user: { ...localUser, passwordChangedAt: stamp },
        tokens: { accessToken: 'fresh-access', refreshToken: 'fresh-refresh' },
      })
      render(<SecuritySettingsPanel />)

      await submitPasswordChange()

      expect(changePasswordAndSignBackIn).toHaveBeenCalledWith('someone@example.test', CURRENT_PASSWORD, NEW_PASSWORD)
      expect(await screen.findByText('Password updated.')).toBeTruthy()
      expect(useAuthStore.getState().tokens?.accessToken).toBe('fresh-access')
      expect(useUserStore.getState().user?.passwordChangedAt).toBe(stamp)
    })

    it('should show why when signing back in fails', async () => {
      changePasswordAndSignBackIn.mockRejectedValue(
        new Error('Your password was changed. Sign in with your new password.'),
      )
      render(<SecuritySettingsPanel />)

      await submitPasswordChange()

      expect(await screen.findByText('Your password was changed. Sign in with your new password.')).toBeTruthy()
      expect(screen.queryByText('Password updated.')).toBeNull()
    })
  })
})
