import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api } from './api'
import { useAuthStore } from './auth.store'

/**
 * Responses are hand-built rather than constructed with `new Response(...)` so
 * the tests do not depend on which fetch implementation the environment ships.
 */
function reply(status: number, body: string) {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => body,
    json: async () => JSON.parse(body),
  } as unknown as Response
}

function ok(body: unknown = {}) {
  return reply(200, JSON.stringify(body))
}

/** The RequestInit the mock recorded for the nth fetch call. */
function initOf(fetchMock: ReturnType<typeof vi.fn>, call = 0): RequestInit {
  return fetchMock.mock.calls[call]?.[1] as RequestInit
}

function headersOf(fetchMock: ReturnType<typeof vi.fn>, call = 0): Record<string, string> {
  return initOf(fetchMock, call).headers as Record<string, string>
}

const replace = vi.fn()

beforeEach(() => {
  useAuthStore.setState({ tokens: null, initialized: false })
  localStorage.clear()
  sessionStorage.clear()
  document.head.innerHTML = ''
  // Expire anything a previous test set.
  document.cookie = 'csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT'

  replace.mockClear()
  // Only `replace` survives the spread — jsdom exposes the rest of Location as
  // prototype accessors. That is enough while api.ts calls nothing else on it,
  // but a future guard such as `if (location.pathname !== '/auth')` would read
  // undefined here and pass without meaning anything.
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, replace },
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('api request construction', () => {
  it('sends the access token as a bearer header', async () => {
    useAuthStore.setState({ tokens: { accessToken: 'at-1', refreshToken: 'rt-1' } })
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.get('/api/rooms')

    expect(headersOf(fetchMock)['Authorization']).toBe('Bearer at-1')
    expect(headersOf(fetchMock)['X-CSRF-Token']).toBeUndefined()
  })

  it('falls back to the CSRF meta tag when there is no access token', async () => {
    document.head.innerHTML = '<meta name="csrf-token" content="from-meta">'
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.post('/api/rooms', { name: 'standup' })

    expect(headersOf(fetchMock)['X-CSRF-Token']).toBe('from-meta')
    expect(headersOf(fetchMock)['Authorization']).toBeUndefined()
  })

  it('falls back to the csrf_token cookie when there is no meta tag', async () => {
    document.cookie = 'csrf_token=from%20cookie'
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.post('/api/rooms')

    // The cookie value is URL-encoded on the wire and must arrive decoded.
    expect(headersOf(fetchMock)['X-CSRF-Token']).toBe('from cookie')
  })

  it('prefers the meta tag over the cookie', async () => {
    document.head.innerHTML = '<meta name="csrf-token" content="from-meta">'
    document.cookie = 'csrf_token=from-cookie'
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.post('/api/rooms')

    expect(headersOf(fetchMock)['X-CSRF-Token']).toBe('from-meta')
  })

  it('sends neither auth header when there is no token and no CSRF source', async () => {
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.post('/api/auth/login', { email: 'a@b.test' })

    expect(headersOf(fetchMock)['Authorization']).toBeUndefined()
    expect(headersOf(fetchMock)['X-CSRF-Token']).toBeUndefined()
  })

  it('serialises a JSON body and declares its content type', async () => {
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.post('/api/rooms', { name: 'standup' })

    expect(headersOf(fetchMock)['Content-Type']).toBe('application/json')
    expect(initOf(fetchMock).body).toBe('{"name":"standup"}')
  })

  it('passes FormData through untouched and leaves the content type to the browser', async () => {
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)
    const form = new FormData()
    form.append('file', 'x')

    await api.post('/api/room/r1/chat/upload', form)

    // Setting Content-Type here would break the multipart boundary.
    expect(headersOf(fetchMock)['Content-Type']).toBeUndefined()
    expect(initOf(fetchMock).body).toBe(form)
  })

  it('omits the body entirely when none is given', async () => {
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.get('/api/rooms')

    expect(initOf(fetchMock).body).toBeUndefined()
  })

  it('always sends cookies', async () => {
    const fetchMock = vi.fn(async () => ok())
    vi.stubGlobal('fetch', fetchMock)

    await api.get('/api/rooms')

    expect(initOf(fetchMock).credentials).toBe('include')
  })
})

describe('401 handling', () => {
  it('refreshes once and retries the original request with the new token', async () => {
    useAuthStore.setState({ tokens: { accessToken: 'stale', refreshToken: 'rt-1' } })

    const fetchMock = vi.fn(async (url: string, init: RequestInit): Promise<Response> => {
      if (url.endsWith('/api/auth/refresh')) {
        return ok({ access_token: 'fresh', refresh_token: 'rt-2' })
      }
      // Read the header off this very call, so the assertion is that the retry
      // carried the refreshed token rather than the stale one.
      const authorization = (init.headers as Record<string, string>)['Authorization']
      return authorization === 'Bearer fresh' ? ok({ rooms: [] }) : reply(401, '{"error":"expired"}')
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.get('/api/rooms')).resolves.toEqual({ rooms: [] })

    const paths = fetchMock.mock.calls.map((c) => c[0] as string)
    expect(paths).toEqual(['/api/rooms', '/api/auth/refresh', '/api/rooms'])
    expect(useAuthStore.getState().tokens?.accessToken).toBe('fresh')
    expect(replace).not.toHaveBeenCalled()
  })

  it('does not refresh when the request carried no token', async () => {
    const fetchMock = vi.fn(async () => reply(401, '{"error":"invalid credentials"}'))
    vi.stubGlobal('fetch', fetchMock)

    // A 401 on an unauthenticated request means the credentials were wrong,
    // not that a session expired — refreshing would be nonsense.
    await expect(api.post('/api/auth/login', { email: 'a@b.test' })).rejects.toThrow('invalid credentials')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(replace).not.toHaveBeenCalled()
  })

  it('does not try to refresh the refresh endpoint itself', async () => {
    useAuthStore.setState({ tokens: { accessToken: 'at-1', refreshToken: 'rt-1' } })
    const fetchMock = vi.fn(async () => reply(401, '{"error":"no session"}'))
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.post('/api/auth/refresh')).rejects.toThrow('no session')

    // One call, not an infinite refresh loop.
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('clears the session and redirects when the refresh fails', async () => {
    useAuthStore.setState({ tokens: { accessToken: 'stale', refreshToken: 'rt-1' } })
    localStorage.setItem('auth_at', 'stale')

    const fetchMock = vi.fn(async (url: string) =>
      url.endsWith('/api/auth/refresh') ? reply(401, '') : reply(401, '{"error":"expired"}'),
    )
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.get('/api/rooms')).rejects.toThrow('Session expired')

    expect(useAuthStore.getState().tokens).toBeNull()
    expect(localStorage.getItem('auth_at')).toBeNull()
    expect(replace).toHaveBeenCalledWith('/auth')
  })

  it('shares a single refresh across concurrent 401s', async () => {
    useAuthStore.setState({ tokens: { accessToken: 'stale', refreshToken: 'rt-1' } })

    let refreshes = 0
    const fetchMock = vi.fn(async (url: string) => {
      if (url.endsWith('/api/auth/refresh')) {
        refreshes++
        // Yield so both callers are waiting on the same in-flight promise.
        await Promise.resolve()
        return ok({ access_token: 'fresh', refresh_token: 'rt-2' })
      }
      return useAuthStore.getState().tokens?.accessToken === 'fresh' ? ok({}) : reply(401, '')
    })
    vi.stubGlobal('fetch', fetchMock)

    await Promise.all([api.get('/api/rooms'), api.get('/api/users')])

    expect(refreshes).toBe(1)
  })
})

describe('error responses', () => {
  it('prefers the error field for the user-facing message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(409, '{"error":"room name taken","message":"ignored"}')),
    )

    await expect(api.post('/api/rooms')).rejects.toThrow('room name taken')
  })

  it('uses the message field when there is no error field', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(400, '{"message":"name is required"}')),
    )

    await expect(api.post('/api/rooms')).rejects.toThrow('name is required')
  })

  it('uses a short non-JSON body verbatim', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(502, 'upstream unavailable')),
    )

    await expect(api.get('/api/rooms')).rejects.toThrow('upstream unavailable')
  })

  it('falls back to the status when a non-JSON body is too long to show', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(500, 'x'.repeat(200))),
    )

    // A 200-character HTML error page is not a user-facing message.
    await expect(api.get('/api/rooms')).rejects.toThrow('Request failed with status 500')
  })

  it('carries the status and both raw and parsed bodies on ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(403, '{"error":"forbidden","code":7}')),
    )

    const error = await api.get('/api/admin/users').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    const apiError = error as ApiError
    expect(apiError.status).toBe(403)
    expect(apiError.body).toBe('{"error":"forbidden","code":7}')
    expect(apiError.parsedBody).toEqual({ error: 'forbidden', code: 7 })
    expect(apiError.name).toBe('ApiError')
  })

  it('leaves parsedBody null when the body is not JSON', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reply(502, 'bad gateway')),
    )

    const error = (await api.get('/api/rooms').catch((e: unknown) => e)) as ApiError

    expect(error.parsedBody).toBeNull()
    expect(error.body).toBe('bad gateway')
  })
})
