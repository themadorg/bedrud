import { redirect } from '@tanstack/react-router'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { isGuestToken } from '#/lib/jwt-user'
import { isGuestUser, type ServerUser, toUser, useUserStore } from '#/lib/user.store'

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
  const mapped = toUser(await api.get<ServerUser>('/api/auth/me'))
  useUserStore.getState().setUser(mapped)
  if (isGuestUser(mapped)) throw redirect({ to: '/' })
}
