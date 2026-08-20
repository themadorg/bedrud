import { redirect } from '@tanstack/react-router'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { isGuestToken } from '#/lib/jwt-user'
import type { User } from '#/lib/user.store'
import { isGuestUser, useUserStore } from '#/lib/user.store'

export async function requireRegisteredUser() {
  if (typeof window === 'undefined') return
  await useAuthStore.getState().initialize()
  const tokens = useAuthStore.getState().tokens
  if (!tokens) throw redirect({ to: '/auth' })
  if (isGuestToken(tokens.accessToken)) throw redirect({ to: '/' })
  const user = useUserStore.getState().user
  if (isGuestUser(user)) throw redirect({ to: '/' })
}

export async function loadRegisteredUser() {
  if (typeof window === 'undefined') return
  if (useUserStore.getState().user) return
  const u = await api.get<User & { accesses?: string[] }>('/api/auth/me')
  const mapped = {
    id: u.id,
    email: u.email,
    name: u.name,
    provider: u.provider,
    isSuperAdmin: u.accesses?.includes('superadmin') ?? false,
    isAdmin: (u.accesses?.includes('admin') || u.accesses?.includes('superadmin')) ?? false,
    accesses: u.accesses ?? [],
    avatarUrl: u.avatarUrl,
  }
  useUserStore.getState().setUser(mapped)
  if (isGuestUser(mapped)) throw redirect({ to: '/' })
}
