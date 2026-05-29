// Shared admin domain types + pure helpers.
// Extracted from duplicated definitions across admin user/room pages (3x AdminUser shapes/logic, 2x AdminRoom).
// These mirror backend DTOs for client-side use (casts, table state, role badges, etc.).
// Keep in sync with server responses; update here + regenerate swagger types when backend changes.

export interface AdminUser {
  id: string
  email: string
  name: string
  provider: string
  isActive: boolean
  accesses: string[] | null
  createdAt: string
}

export interface AdminRoom {
  id: string
  name: string
  createdBy: string
  isPublic: boolean
  isActive: boolean
  maxParticipants: number
  createdAt: string
  updatedAt: string
  expiresAt: string
  adminId: string
  mode: string
  settings?: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee: boolean
    isPersistent?: boolean
  }
  // Extended fields from AdminRoomDetail DTO (populated in some detail views)
  participantsCount?: number
  lastActivityAt?: string | null
  ownerName?: string
  ownerEmail?: string
  deletedAt?: string | null
}

// ── Role model (5 roles, matches backend) ─────────────────────────────────────

export const ROLE_OPTS = [
  { label: 'Superadmin', value: 'superadmin' },
  { label: 'Admin', value: 'admin' },
  { label: 'Moderator', value: 'moderator' },
  { label: 'User', value: 'user' },
  { label: 'Guest', value: 'guest' },
] as const

export type RoleValue = (typeof ROLE_OPTS)[number]['value']

export const ROLE_ACCESS_MAP: Record<RoleValue, string[]> = {
  superadmin: ['superadmin', 'user'],
  admin: ['admin', 'user'],
  moderator: ['moderator', 'user'],
  user: ['user'],
  guest: ['guest'],
}

export function detectRole(accesses: string[] | null): RoleValue {
  if (!accesses || accesses.length === 0) return 'user'
  if (accesses.includes('superadmin')) return 'superadmin'
  if (accesses.includes('admin')) return 'admin'
  if (accesses.includes('moderator')) return 'moderator'
  if (accesses.includes('guest')) return 'guest'
  return 'user'
}

export function getRoleLabel(accesses: string[] | null): string {
  const role = detectRole(accesses)
  const found = ROLE_OPTS.find((r) => r.value === role)
  return found ? found.label : 'User'
}
