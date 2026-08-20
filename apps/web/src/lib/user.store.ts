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
