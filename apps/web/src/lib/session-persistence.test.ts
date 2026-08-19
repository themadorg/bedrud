import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { useAuthStore } from './auth.store'

/**
 * Where a session is stored is a security decision, and it has to survive the
 * things that rewrite tokens behind the user's back: the 401 refresh in api.ts
 * and initialize() on every page load. These tests cross both modules, which
 * is the only way the promotion bug was visible — the store honours the flag
 * perfectly well when called directly.
 */

const REMEMBER_KEY = 'auth_remember'
const ACCESS_TOKEN_KEY = 'auth_at'

function reply(status: number, body: string) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => body,
    json: async () => JSON.parse(body),
  } as unknown as Response
}

/** Answers the refresh endpoint, and any other path once the token is fresh. */
function refreshingFetch(newToken: string) {
  return vi.fn(async (url: string, init: RequestInit): Promise<Response> => {
    if (url.endsWith('/api/auth/refresh')) {
      return reply(200, JSON.stringify({ access_token: newToken, refresh_token: 'rt-new' }))
    }
    const auth = (init.headers as Record<string, string>)['Authorization']
    return auth === `Bearer ${newToken}` ? reply(200, '{}') : reply(401, '')
  })
}

beforeEach(() => {
  // clear() rather than setState: initialize() dedupes through a module-level
  // promise that it only nulls on the failure path, so a preceding successful
  // initialize would otherwise make the next one return that stale promise
  // without calling fetch at all. clear() resets it.
  useAuthStore.getState().clear()
  localStorage.clear()
  sessionStorage.clear()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, replace: vi.fn() },
  })
})

describe('a 401 refresh keeps the session where it was', () => {
  it('leaves an ephemeral session out of localStorage', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'stale', refreshToken: 'rt-1' }, 'ephemeral')
    vi.stubGlobal('fetch', refreshingFetch('fresh'))

    await api.get('/api/rooms')

    // Otherwise a guest who joined a meeting on a shared machine stays signed
    // in across a browser restart.
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REMEMBER_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('fresh')
  })

  it('leaves a session-only login out of localStorage', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'stale', refreshToken: 'rt-1' }, false)
    vi.stubGlobal('fetch', refreshingFetch('fresh'))

    await api.get('/api/rooms')

    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('fresh')
  })

  it('keeps a remembered session in localStorage', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'stale', refreshToken: 'rt-1' })
    vi.stubGlobal('fetch', refreshingFetch('fresh'))

    await api.get('/api/rooms')

    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('fresh')
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })
})

describe('initialize', () => {
  it('restores a session from the refresh cookie', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(200, JSON.stringify({ access_token: 'at-1', refresh_token: 'rt-1' }))),
    )

    await useAuthStore.getState().initialize()

    expect(useAuthStore.getState().tokens?.accessToken).toBe('at-1')
    expect(useAuthStore.getState().initialized).toBe(true)
  })

  it('does not promote an ephemeral session when the cookie refresh succeeds', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'old', refreshToken: 'rt-0' }, 'ephemeral')
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(200, JSON.stringify({ access_token: 'at-1', refresh_token: 'rt-1' }))),
    )

    await useAuthStore.getState().initialize()

    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-1')
  })

  it('falls back to the persisted token when the cookie refresh fails', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'persisted', refreshToken: null })
    useAuthStore.setState({ tokens: null }) // simulate a fresh page load

    const fetchMock = vi.fn(async (url: string) =>
      url.endsWith('/api/auth/refresh') ? reply(401, '') : reply(200, '{}'),
    )
    vi.stubGlobal('fetch', fetchMock)

    await useAuthStore.getState().initialize()

    // /api/auth/me validated the persisted token, so the session stands.
    expect(useAuthStore.getState().tokens?.accessToken).toBe('persisted')
    expect(fetchMock.mock.calls.map((c) => c[0] as string)).toEqual(['/api/auth/refresh', '/api/auth/me'])
  })

  it('clears the session when neither path works', async () => {
    useAuthStore.getState().setTokens({ accessToken: 'persisted', refreshToken: null })
    useAuthStore.setState({ tokens: null }) // simulate a fresh page load
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(401, '')),
    )

    await useAuthStore.getState().initialize()

    expect(useAuthStore.getState().tokens).toBeNull()
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(useAuthStore.getState().initialized).toBe(true)
  })

  it('does not try the persisted-token path for someone who was never signed in', async () => {
    // Typed so mock.calls carries the url, which the assertion below reads.
    const fetchMock = vi.fn<(url: string) => Promise<Response>>(async () => reply(401, ''))
    vi.stubGlobal('fetch', fetchMock)

    await useAuthStore.getState().initialize()

    // No REMEMBER_KEY in either storage, so there is nothing to validate.
    expect(fetchMock.mock.calls.map((c) => c[0] as string)).toEqual(['/api/auth/refresh'])
  })

  it('runs once for concurrent callers', async () => {
    const fetchMock = vi.fn(async () => {
      await Promise.resolve()
      return reply(200, JSON.stringify({ access_token: 'at-1', refresh_token: 'rt-1' }))
    })
    vi.stubGlobal('fetch', fetchMock)

    await Promise.all([useAuthStore.getState().initialize(), useAuthStore.getState().initialize()])

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
