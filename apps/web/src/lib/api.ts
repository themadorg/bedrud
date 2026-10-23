import { currentRememberMode, refusesSession, useAuthStore } from './auth.store'

// In dev, leave BASE_URL empty so requests go to /api/... and Vite's proxy
// forwards them to localhost:7071 — no CORS. In production, set VITE_API_URL
// to the absolute server origin (e.g. https://api.bedrud.com).
const BASE_URL = (import.meta.env['VITE_API_URL'] as string | undefined) ?? ''

/**
 * CSRF protection strategy:
 *
 * 1. Primary: The `Authorization: Bearer <token>` header is sent on every
 *    request that has an access token. Server RequireBearerForMutations rejects
 *    cookie-only POST/PUT/PATCH/DELETE. Custom headers trigger CORS preflight,
 *    which cross-origin attackers cannot bypass — implicit CSRF protection.
 *
 * 2. Fallback: When no access token is available (e.g., cookie-only sessions),
 *    we attach an `X-CSRF-Token` header read from either:
 *    - a `<meta name="csrf-token">` tag set by the server-rendered page, or
 *    - a `csrf_token` cookie that the server sets alongside the session cookie.
 *
 *    The server must validate this header on state-changing requests (POST, PUT,
 *    PATCH, DELETE) when no Authorization header is present.
 */
export class ApiError extends Error {
  status: number
  body: string
  parsedBody: Record<string, unknown> | null
  constructor(status: number, body: string, parsedBody: Record<string, unknown> | null, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
    this.parsedBody = parsedBody
  }
}

/** Marks a request that never reached the server, in place of the browser's own wording, which differs per engine. */
export class NetworkError extends Error {
  constructor() {
    super("Couldn't reach the server")
    this.name = 'NetworkError'
  }
}

/** Sends a request, reporting a failure to reach the server as a NetworkError. */
async function send(url: string, init: RequestInit): Promise<Response> {
  try {
    return await fetch(url, init)
  } catch (error) {
    // An aborted request stays an abort, so a caller that cancelled it can still tell.
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new NetworkError()
  }
}

function getCsrfToken(): string | null {
  if (typeof document === 'undefined') return null
  // Try meta tag first (server-rendered pages)
  const meta = document.querySelector('meta[name="csrf-token"]')
  if (meta?.getAttribute('content')) return meta.getAttribute('content')
  // Fall back to csrf_token cookie
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
  if (match?.[1]) return decodeURIComponent(match[1])
  return null
}

type RequestOptions = Omit<RequestInit, 'body'> & { body?: unknown }

// Singleton refresh promise — multiple concurrent 401s share one refresh call
// instead of hammering the /refresh endpoint in parallel.
let refreshPromise: Promise<string | null> | null = null

/**
 * Returns the new access token, or null when the server refuses the session.
 *
 * A refresh that never reaches the server, or meets a server error, a timeout or rate limiting,
 * throws instead: those say nothing about the session, so the caller fails without signing out.
 */
async function doRefresh(): Promise<string | null> {
  const res = await send(`${BASE_URL}/api/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // HTTP-only refresh_token cookie is sent automatically
  })
  if (!res.ok) {
    if (refusesSession(res.status)) return null
    throw new ApiError(res.status, await res.text(), null, `Request failed with status ${res.status}`)
  }
  try {
    const data = (await res.json()) as { access_token?: string; refresh_token?: string }
    if (!data.access_token || !data.refresh_token) return null
    // Preserve the storage the session already uses. Omitting this promotes a
    // session-only login into localStorage on the first refresh.
    useAuthStore.getState().setTokens(
      {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
      },
      currentRememberMode(),
    )
    return data.access_token
  } catch {
    // A success without a readable token pair cannot continue the session.
    return null
  }
}

function redirectToAuth() {
  useAuthStore.getState().clear()
  if (typeof window !== 'undefined') {
    window.location.replace('/auth')
  }
}

async function request<T>(path: string, options: RequestOptions = {}, isRetry = false): Promise<T> {
  const tokens = useAuthStore.getState().tokens
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string> | undefined),
  }
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }

  if (tokens?.accessToken) {
    // Bearer token in a custom header triggers CORS preflight, providing
    // implicit CSRF protection — cross-origin attackers cannot set this header.
    headers['Authorization'] = `Bearer ${tokens.accessToken}`
  } else {
    // No Bearer token — attach CSRF token as defense-in-depth for cookie-only auth.
    const csrf = getCsrfToken()
    if (csrf) {
      headers['X-CSRF-Token'] = csrf
    }
  }

  const res = await send(`${BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: 'include', // always send HTTP-only cookies
    body:
      options.body instanceof FormData
        ? options.body
        : options.body !== undefined
          ? JSON.stringify(options.body)
          : undefined,
  })

  // On 401, try to silently refresh once, then retry the original request.
  // Skip the interceptor on the refresh endpoint itself to avoid infinite loops,
  // and skip on retries (already refreshed once).
  // Also skip if no access token was sent — 401 on an unauthenticated request
  // (e.g. login with wrong credentials) means auth failed, not session expired.
  if (res.status === 401 && !isRetry && path !== '/api/auth/refresh' && tokens?.accessToken) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => {
        refreshPromise = null
      })
    }

    const newToken = await refreshPromise

    if (!newToken) {
      // Refresh failed — session is truly expired
      redirectToAuth()
      throw new Error('Session expired')
    }

    // Retry with the new access token
    return request<T>(path, options, true)
  }

  if (!res.ok) {
    const text = await res.text()
    let parsed: Record<string, unknown> | null = null
    let userMsg = `Request failed with status ${res.status}`
    try {
      const json = JSON.parse(text) as Record<string, unknown>
      parsed = json
      if (typeof json.error === 'string') userMsg = json.error
      else if (typeof json.message === 'string') userMsg = json.message
    } catch {
      // Non-JSON response — use raw text if short
      if (text.length < 200) userMsg = text
    }
    throw new ApiError(res.status, text, parsed, userMsg)
  }

  return res.json() as Promise<T>
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>(path, { ...options, method: 'GET' }),

  post: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'POST', body }),

  put: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'PUT', body }),

  delete: <T>(path: string, options?: RequestOptions) => request<T>(path, { ...options, method: 'DELETE' }),
}

export const API_URL = BASE_URL
