import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '#/lib/auth.store'
import { useUserStore } from '#/lib/user.store'

export interface AuthResponse {
  user: {
    id: string
    email: string
    name: string
    provider: string
    accesses: string[] | null
    avatarUrl?: string
  }
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
  const storeSession = useStoreAuthSession()

  return (res: AuthResponse, redirectTo?: string) => {
    storeSession(res)
    navigate({ to: redirectTo ?? '/dashboard' })
  }
}

/**
 * Hook that returns a function storing a successful auth response's tokens and user without
 * navigating, for pages that have one more step to show before leaving.
 */
export function useStoreAuthSession() {
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)

  return (res: AuthResponse) => {
    setTokens(res.tokens)
    setUser({
      id: res.user.id,
      email: res.user.email,
      name: res.user.name,
      provider: res.user.provider,
      isSuperAdmin: res.user.accesses?.includes('superadmin') ?? false,
      isAdmin: (res.user.accesses?.includes('admin') || res.user.accesses?.includes('superadmin')) ?? false,
      accesses: res.user.accesses ?? [],
      avatarUrl: res.user.avatarUrl,
    })
  }
}
