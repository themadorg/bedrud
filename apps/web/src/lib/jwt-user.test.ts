import { describe, expect, it } from 'vitest'
import { decodeBedrudJwt } from './jwt-user'

/** Builds an unsigned JWT with the given payload — decoding never verifies. */
function jwt(payload: Record<string, unknown>): string {
  const encode = (obj: unknown) => btoa(JSON.stringify(obj)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  return `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode(payload)}.signature`
}

describe('decodeBedrudJwt', () => {
  it('reads the user id and access list', () => {
    expect(decodeBedrudJwt(jwt({ userId: 'u-1', accesses: ['admin', 'superadmin'] }))).toEqual({
      userId: 'u-1',
      accesses: ['admin', 'superadmin'],
    })
  })

  it('defaults missing claims rather than returning undefined', () => {
    // Callers index straight into accesses, so it has to be an array.
    expect(decodeBedrudJwt(jwt({ sub: 'someone-elses-claim' }))).toEqual({ userId: '', accesses: [] })
  })

  it('grants nothing when the token is absent', () => {
    expect(decodeBedrudJwt(undefined)).toEqual({ userId: '', accesses: [] })
    expect(decodeBedrudJwt(null)).toEqual({ userId: '', accesses: [] })
    expect(decodeBedrudJwt('')).toEqual({ userId: '', accesses: [] })
  })

  it('grants nothing when the token is malformed, instead of throwing', () => {
    // A decode that threw here would take down whatever rendered the user menu.
    expect(decodeBedrudJwt('not-a-jwt')).toEqual({ userId: '', accesses: [] })
    expect(decodeBedrudJwt('a.b')).toEqual({ userId: '', accesses: [] })
    expect(decodeBedrudJwt('a.!!!not-base64!!!.c')).toEqual({ userId: '', accesses: [] })
  })

  it("ignores expiry — freshness is the server's call, not the decoder's", () => {
    const expired = jwt({ userId: 'u-1', accesses: ['admin'], exp: 1 })

    expect(decodeBedrudJwt(expired)).toEqual({ userId: 'u-1', accesses: ['admin'] })
  })
})
