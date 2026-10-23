import { createFileRoute, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { createContext, useContext, useEffect, useMemo } from 'react'
import { api } from '#/lib/api'
import { loadRegisteredUser } from '#/lib/require-registered-user'
import { canOpenAdmin, type ServerUser, toUser, useUserStore } from '#/lib/user.store'

export const Route = createFileRoute('/dashboard/admin')({
  // Loader confirms admin status (after ensuring user is loaded).
  // This prevents non-admin flash of admin UI that the old beforeLoad allowed
  // when user was still null during initial hydration.
  loader: async () => {
    if (typeof window === 'undefined') return

    // Ensure we have a fresh user (similar to parent dashboard loader)
    let user = useUserStore.getState().user
    if (!user) {
      try {
        useUserStore.getState().setUser(toUser(await api.get<ServerUser>('/api/auth/me')))
        user = useUserStore.getState().user
      } catch {
        throw redirect({ to: '/auth' })
      }
    }

    if (!canOpenAdmin(user)) {
      throw redirect({ to: '/dashboard' })
    }
  },
  staleTime: Infinity,
  component: AdminGuard,
})

interface AdminContextValue {
  isReadOnly: boolean
  isModerator: boolean
  currentUserId: string | null
}

const AdminContext = createContext<AdminContextValue>({ isReadOnly: false, isModerator: false, currentUserId: null })

export function useAdminContext() {
  return useContext(AdminContext)
}

function AdminGuard() {
  const user = useUserStore((s) => s.user)
  const navigate = useNavigate()
  const accesses = user?.accesses ?? []
  const isModerator = accesses.includes('moderator')
  const canAccess = canOpenAdmin(user)

  useEffect(() => {
    // A hard load runs the loader during SSR, where it returns early, so a deep link can
    // reach the guard before any user is loaded.
    if (user === null) {
      loadRegisteredUser().catch(() => navigate({ to: '/auth' }))
      return
    }
    if (!canAccess) {
      navigate({ to: '/dashboard' })
    }
  }, [user, canAccess, navigate])

  const ctx = useMemo<AdminContextValue>(
    () => ({ isReadOnly: isModerator, isModerator, currentUserId: user?.id ?? null }),
    [isModerator, user?.id],
  )

  if (!canAccess) return null

  return (
    <AdminContext.Provider value={ctx}>
      <Outlet />
    </AdminContext.Provider>
  )
}
