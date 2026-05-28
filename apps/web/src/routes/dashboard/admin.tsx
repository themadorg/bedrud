import { createFileRoute, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { createContext, useContext, useEffect, useMemo } from 'react'
import { useUserStore } from '#/lib/user.store'

export const Route = createFileRoute('/dashboard/admin')({
  beforeLoad: () => {
    if (typeof window === 'undefined') return
    const user = useUserStore.getState().user
    const accesses = user?.accesses ?? []
    const canAccess = user?.isSuperAdmin || accesses.includes('admin') || accesses.includes('moderator')
    if (user !== null && !canAccess) throw redirect({ to: '/dashboard' })
  },
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
  const canAccess = user?.isSuperAdmin || accesses.includes('admin') || isModerator

  useEffect(() => {
    if (user !== null && !canAccess) {
      navigate({ to: '/dashboard' })
    }
  }, [user, canAccess, navigate])

  const ctx = useMemo<AdminContextValue>(
    () => ({ isReadOnly: isModerator, isModerator, currentUserId: user?.id ?? null }),
    [isModerator, user?.id],
  )

  return (
    <AdminContext.Provider value={ctx}>
      <Outlet />
    </AdminContext.Provider>
  )
}
