import { create } from 'zustand'

export interface User {
  id: string
  email: string
  name: string
  provider: string
  isSuperAdmin: boolean
  isAdmin: boolean
  accesses: string[] | null
  avatarUrl?: string
  passwordChangedAt?: string | null
}

/** The user as `/api/auth/me` and the sign-in responses send it. */
export interface ServerUser {
  id: string
  email: string
  name: string
  provider: string
  accesses: string[] | null
  avatarUrl?: string
  passwordChangedAt?: string | null
}

export function toUser(serverUser: ServerUser): User {
  return {
    id: serverUser.id,
    email: serverUser.email,
    name: serverUser.name,
    provider: serverUser.provider,
    isSuperAdmin: serverUser.accesses?.includes('superadmin') ?? false,
    isAdmin: (serverUser.accesses?.includes('admin') || serverUser.accesses?.includes('superadmin')) ?? false,
    accesses: serverUser.accesses ?? [],
    avatarUrl: serverUser.avatarUrl,
    passwordChangedAt: serverUser.passwordChangedAt,
  }
}

/**
 * Maps a profile update onto the stored user. The profile endpoints answer without
 * `passwordChangedAt`, so the stored stamp is kept unless the update carries one.
 */
export function applyProfileUpdate(current: User | null, updated: ServerUser): User {
  const user = toUser(updated)
  if (updated.passwordChangedAt === undefined && current) user.passwordChangedAt = current.passwordChangedAt
  return user
}

/** The server's `/admin` group answers superadmins only. */
export function canOpenAdmin(user: User | null | undefined): boolean {
  return user?.isSuperAdmin ?? false
}

export function isGuestUser(user: User | null | undefined): boolean {
  if (!user) return false
  return user.provider === 'guest' || (user.accesses?.includes('guest') ?? false)
}

interface UserStore {
  user: User | null
  setUser: (user: User) => void
  clear: () => void
}

export const useUserStore = create<UserStore>()((set) => ({
  user: null,
  setUser: (user) => set({ user }),
  clear: () => set({ user: null }),
}))
