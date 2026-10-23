import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put, post } = vi.hoisted(() => ({ put: vi.fn(), post: vi.fn() }))

vi.mock('#/lib/api', () => ({ api: { put, post } }))

import { changePasswordAndSignBackIn } from '#/lib/change-password'

const authResponse = {
  user: { id: 'user-1', email: 'someone@example.test', name: 'Someone', provider: 'local', accesses: ['user'] },
  tokens: { accessToken: 'fresh-access', refreshToken: 'fresh-refresh' },
}

describe('changePasswordAndSignBackIn', () => {
  beforeEach(() => {
    put.mockReset()
    post.mockReset()
  })

  it('should change the password, then sign in with the new one', async () => {
    put.mockResolvedValue({})
    post.mockResolvedValue(authResponse)

    const result = await changePasswordAndSignBackIn('someone@example.test', 'old-password-1', 'new-password-12')

    expect(put).toHaveBeenCalledWith('/api/auth/password', {
      currentPassword: 'old-password-1',
      newPassword: 'new-password-12',
    })
    expect(post).toHaveBeenCalledWith('/api/auth/login', {
      email: 'someone@example.test',
      password: 'new-password-12',
    })
    expect(put.mock.invocationCallOrder[0]).toBeLessThan(post.mock.invocationCallOrder[0])
    expect(result).toBe(authResponse)
  })

  it('should not sign in when the server refuses the change', async () => {
    put.mockRejectedValue(new Error('current password is incorrect'))

    await expect(
      changePasswordAndSignBackIn('someone@example.test', 'wrong-password', 'new-password-12'),
    ).rejects.toThrow('current password is incorrect')
    expect(post).not.toHaveBeenCalled()
  })

  it('should tell the caller the password changed when signing back in fails', async () => {
    put.mockResolvedValue({})
    post.mockRejectedValue(new Error('network down'))

    await expect(
      changePasswordAndSignBackIn('someone@example.test', 'old-password-1', 'new-password-12'),
    ).rejects.toThrow('Your password was changed. Sign in with your new password.')
  })
})
