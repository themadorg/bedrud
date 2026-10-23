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

const passkeyUser: ServerUser = {
  id: 'user-1',
  email: 'someone@example.test',
  name: 'Someone',
  provider: 'passkey',
  accesses: ['user'],
  passwordChangedAt: null,
}

function passwordRow() {
  return screen.getByText('Password', { selector: 'span' }).nextElementSibling?.textContent
}

async function submitNewPassword() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('New password'), NEW_PASSWORD)
  await user.type(screen.getByLabelText('Confirm new password'), NEW_PASSWORD)
  await user.click(screen.getByRole('button', { name: /password/i }))
}

describe('SecuritySettingsPanel', () => {
  beforeEach(() => {
    changePasswordAndSignBackIn.mockReset()
  })

  afterEach(() => {
    cleanup()
    useUserStore.getState().clear()
  })

  describe('for a passkey account with no password', () => {
    beforeEach(() => {
      useUserStore.getState().setUser(toUser(passkeyUser))
      render(<SecuritySettingsPanel />)
    })

    it('should not ask for a current password', () => {
      expect(screen.queryByLabelText('Current password')).toBeNull()
    })

    it('should say the password is not set', () => {
      expect(passwordRow()).toBe('Not set')
    })

    it('should offer to set a password', () => {
      expect(screen.getByRole('button', { name: 'Set password' })).toBeTruthy()
    })
  })

  describe('for an account with a password', () => {
    beforeEach(() => {
      useUserStore.getState().setUser(toUser({ ...passkeyUser, provider: 'local' }))
      render(<SecuritySettingsPanel />)
    })

    it('should ask for the current password', () => {
      expect(screen.getByLabelText('Current password')).toBeTruthy()
    })

    it('should say the password is set', () => {
      expect(passwordRow()).toBe('Set')
    })
  })

  describe('after a password change', () => {
    it('should send the current password and store the fresh session', async () => {
      changePasswordAndSignBackIn.mockResolvedValue({
        user: { ...passkeyUser, provider: 'local' },
        tokens: { accessToken: 'fresh-access', refreshToken: 'fresh-refresh' },
      })
      useUserStore.getState().setUser(toUser({ ...passkeyUser, provider: 'local' }))
      render(<SecuritySettingsPanel />)

      await userEvent.setup().type(screen.getByLabelText('Current password'), CURRENT_PASSWORD)
      await submitNewPassword()

      expect(changePasswordAndSignBackIn).toHaveBeenCalledWith('someone@example.test', CURRENT_PASSWORD, NEW_PASSWORD)
      expect(await screen.findByText('Password updated.')).toBeTruthy()
      expect(useAuthStore.getState().tokens?.accessToken).toBe('fresh-access')
    })
  })

  describe('after a password is set', () => {
    it('should store the fresh session and report success', async () => {
      const stamp = '2026-10-20T10:00:00Z'
      changePasswordAndSignBackIn.mockResolvedValue({
        user: { ...passkeyUser, passwordChangedAt: stamp },
        tokens: { accessToken: 'fresh-access', refreshToken: 'fresh-refresh' },
      })
      useUserStore.getState().setUser(toUser(passkeyUser))
      render(<SecuritySettingsPanel />)

      await submitNewPassword()

      expect(changePasswordAndSignBackIn).toHaveBeenCalledWith('someone@example.test', '', NEW_PASSWORD)
      expect(await screen.findByText('Password updated.')).toBeTruthy()
      expect(useAuthStore.getState().tokens?.accessToken).toBe('fresh-access')
      expect(useUserStore.getState().user?.passwordChangedAt).toBe(stamp)
    })

    it('should show why when signing back in fails', async () => {
      changePasswordAndSignBackIn.mockRejectedValue(
        new Error('Your password was changed. Sign in with your new password.'),
      )
      useUserStore.getState().setUser(toUser(passkeyUser))
      render(<SecuritySettingsPanel />)

      await submitNewPassword()

      expect(await screen.findByText('Your password was changed. Sign in with your new password.')).toBeTruthy()
      expect(screen.queryByText('Password updated.')).toBeNull()
    })
  })
})
