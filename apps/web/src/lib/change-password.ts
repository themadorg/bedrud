import { api } from '#/lib/api'
import type { AuthResponse } from '#/lib/handle-auth-success'

const SIGN_BACK_IN_FAILED = 'Your password was changed. Sign in with your new password.'

/**
 * Changes the password, then signs in again with the new one.
 *
 * The server ends every session of the account on a password change, this one included, so
 * without a fresh session the next request is refused and the app signs out without a word.
 */
export async function changePasswordAndSignBackIn(
  email: string,
  currentPassword: string,
  newPassword: string,
): Promise<AuthResponse> {
  await api.put('/api/auth/password', { currentPassword, newPassword })
  try {
    return await api.post<AuthResponse>('/api/auth/login', { email, password: newPassword })
  } catch {
    throw new Error(SIGN_BACK_IN_FAILED)
  }
}
