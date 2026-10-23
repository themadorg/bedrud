import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '#/lib/auth.store'
import { type ServerUser, toUser, useUserStore } from '#/lib/user.store'

export interface AuthResponse {
  user: ServerUser
  tokens: {
    accessToken: string
    refreshToken: string
  }
}

/**
 * Hook that returns a function to handle a successful auth response:
 * stores tokens, stores user, and navigates to the given path.
 */
export function useHandleAuthSuccess() {
  const navigate = useNavigate()
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)

  return (res: AuthResponse, redirectTo?: string) => {
    setTokens(res.tokens)
    setUser(toUser(res.user))
    navigate({ to: redirectTo ?? '/dashboard' })
  }
}
