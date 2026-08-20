import { describe, expect, it } from 'vitest'
import { decodeBedrudJwt, isGuestToken } from './jwt-user'

/** Builds an unsigned JWT with the given payload — decoding never verifies. */
function jwt(payload: Record<string, unknown>): string {
  const encode = (obj: unknown) => btoa(JSON.stringify(obj)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  return `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode(payload)}.signature`
}

describe('decodeBedrudJwt', () => {
  it('reads the user id, provider, and access list', () => {
    expect(decodeBedrudJwt(jwt({ userId: 'u-1', provider: 'local', accesses: ['admin', 'superadmin'] }))).toEqual({
      userId: 'u-1',
      provider: 'local',
      accesses: ['admin', 'superadmin'],
    })
  })

  it('defaults missing claims rather than returning undefined', () => {
    expect(decodeBedrudJwt(jwt({ sub: 'someone-elses-claim' }))).toEqual({
      userId: '',
      provider: '',
      accesses: [],
    })
  })

  it('grants nothing when the token is absent', () => {
    expect(decodeBedrudJwt(undefined)).toEqual({ userId: '', provider: '', accesses: [] })
    expect(decodeBedrudJwt(null)).toEqual({ userId: '', provider: '', accesses: [] })
    expect(decodeBedrudJwt('')).toEqual({ userId: '', provider: '', accesses: [] })
  })

  it('grants nothing when the token is malformed, instead of throwing', () => {
    expect(decodeBedrudJwt('not-a-jwt')).toEqual({ userId: '', provider: '', accesses: [] })
    expect(decodeBedrudJwt('a.b')).toEqual({ userId: '', provider: '', accesses: [] })
    expect(decodeBedrudJwt('a.!!!not-base64!!!.c')).toEqual({ userId: '', provider: '', accesses: [] })
  })

  it("ignores expiry — freshness is the server's call, not the decoder's", () => {
    const expired = jwt({ userId: 'u-1', accesses: ['admin'], exp: 1 })

    expect(decodeBedrudJwt(expired)).toEqual({ userId: 'u-1', provider: '', accesses: ['admin'] })
  })
})

describe('isGuestToken', () => {
  it('is true when provider is guest', () => {
    expect(isGuestToken(jwt({ userId: 'g-1', provider: 'guest', accesses: [] }))).toBe(true)
  })

  it('is true when accesses includes guest', () => {
    expect(isGuestToken(jwt({ userId: 'g-1', provider: 'local', accesses: ['guest'] }))).toBe(true)
  })

  it('is false for normal users', () => {
    expect(isGuestToken(jwt({ userId: 'u-1', provider: 'local', accesses: ['admin'] }))).toBe(false)
    expect(isGuestToken(undefined)).toBe(false)
  })
})
