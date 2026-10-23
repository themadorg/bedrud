import { describe, expect, it } from 'vitest'
import { applyProfileUpdate, canOpenAdmin, hasPassword, type ServerUser, toUser, type User } from '#/lib/user.store'

const serverUser: ServerUser = {
  id: 'user-1',
  email: 'someone@example.test',
  name: 'Someone',
  provider: 'local',
  accesses: ['user'],
  avatarUrl: '/avatar.png',
  passwordChangedAt: null,
}

function userWith(overrides: Partial<User>): User {
  return { ...toUser(serverUser), ...overrides }
}

describe('toUser', () => {
  it('should carry the password stamp the server sends', () => {
    const stamp = '2026-10-20T10:00:00Z'

    expect(toUser({ ...serverUser, passwordChangedAt: stamp }).passwordChangedAt).toBe(stamp)
  })

  it('should read superadmin from accesses and count it as admin', () => {
    const user = toUser({ ...serverUser, accesses: ['superadmin'] })

    expect(user.isSuperAdmin).toBe(true)
    expect(user.isAdmin).toBe(true)
  })

  it('should read admin from accesses without making it superadmin', () => {
    const user = toUser({ ...serverUser, accesses: ['admin'] })

    expect(user.isSuperAdmin).toBe(false)
    expect(user.isAdmin).toBe(true)
  })

  it('should replace missing accesses with an empty list', () => {
    expect(toUser({ ...serverUser, accesses: null }).accesses).toEqual([])
  })
})

describe('applyProfileUpdate', () => {
  const stamp = '2026-10-20T10:00:00Z'

  it('should keep the stored password stamp when the update omits it', () => {
    const { passwordChangedAt: _omitted, ...profileResponse } = serverUser
    const current = toUser({ ...serverUser, passwordChangedAt: stamp })

    expect(applyProfileUpdate(current, { ...profileResponse, name: 'Renamed' })).toMatchObject({
      name: 'Renamed',
      passwordChangedAt: stamp,
    })
  })

  it('should take the stamp the update carries', () => {
    const current = toUser(serverUser)

    expect(applyProfileUpdate(current, { ...serverUser, passwordChangedAt: stamp }).passwordChangedAt).toBe(stamp)
  })

  it('should map the update alone when no user is stored', () => {
    expect(applyProfileUpdate(null, serverUser)).toEqual(toUser(serverUser))
  })
})

describe('hasPassword', () => {
  it('should return false for a passkey account that never set a password', () => {
    expect(hasPassword(userWith({ provider: 'passkey', passwordChangedAt: null }))).toBe(false)
  })

  it('should return true for a passkey account once a password is stamped', () => {
    expect(hasPassword(userWith({ provider: 'passkey', passwordChangedAt: '2026-10-20T10:00:00Z' }))).toBe(true)
  })

  it('should return true for an email account that never changed its password', () => {
    expect(hasPassword(userWith({ provider: 'local', passwordChangedAt: null }))).toBe(true)
  })
})

describe('canOpenAdmin', () => {
  it('should return true for a superadmin', () => {
    expect(canOpenAdmin(toUser({ ...serverUser, accesses: ['superadmin'] }))).toBe(true)
  })

  it('should return false for an admin who is not a superadmin', () => {
    expect(canOpenAdmin(toUser({ ...serverUser, accesses: ['admin'] }))).toBe(false)
  })

  it('should return false for a moderator', () => {
    expect(canOpenAdmin(toUser({ ...serverUser, accesses: ['moderator'] }))).toBe(false)
  })

  it('should return false when no user is loaded', () => {
    expect(canOpenAdmin(null)).toBe(false)
  })
})
