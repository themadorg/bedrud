import { beforeEach, describe, expect, it } from 'vitest'
import { useAuthStore } from './auth.store'

const REMEMBER_KEY = 'auth_remember'
const ACCESS_TOKEN_KEY = 'auth_at'

const tokens = { accessToken: 'at-1', refreshToken: 'rt-1' }

beforeEach(() => {
  useAuthStore.setState({ tokens: null, initialized: false })
  localStorage.clear()
  sessionStorage.clear()
})

describe('setTokens', () => {
  it('persists to localStorage by default, so the session survives a browser restart', () => {
    useAuthStore.getState().setTokens(tokens)

    expect(useAuthStore.getState().tokens).toEqual(tokens)
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-1')
    expect(localStorage.getItem(REMEMBER_KEY)).toBe('1')
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('keeps the token out of localStorage when the user did not ask to be remembered', () => {
    useAuthStore.getState().setTokens(tokens, false)

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-1')
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REMEMBER_KEY)).toBeNull()
  })

  it('treats an ephemeral session the same as not being remembered', () => {
    useAuthStore.getState().setTokens(tokens, 'ephemeral')

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-1')
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('does not leave a stale token in localStorage when a remembered session is replaced by a session-only one', () => {
    // Deliberately no clear() in between: one storage owns the session, so
    // switching modes has to empty the other by itself. Callers do not
    // reliably clear first — the guest join path does not.
    useAuthStore.getState().setTokens(tokens)
    useAuthStore.getState().setTokens({ accessToken: 'at-2', refreshToken: null }, false)

    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REMEMBER_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-2')
  })

  it('empties sessionStorage when a session-only login is replaced by a remembered one', () => {
    useAuthStore.getState().setTokens(tokens, 'ephemeral')
    useAuthStore.getState().setTokens({ accessToken: 'at-2', refreshToken: null })

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(REMEMBER_KEY)).toBeNull()
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-2')
  })
})

describe('updateAccessToken', () => {
  it('writes the refreshed token back to whichever storage holds the session', () => {
    useAuthStore.getState().setTokens(tokens)

    useAuthStore.getState().updateAccessToken('at-2')

    expect(useAuthStore.getState().tokens).toEqual({ accessToken: 'at-2', refreshToken: 'rt-1' })
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-2')
  })

  it('stays in sessionStorage for a session-only login', () => {
    useAuthStore.getState().setTokens(tokens, false)

    useAuthStore.getState().updateAccessToken('at-2')

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBe('at-2')
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('keeps the refresh token', () => {
    useAuthStore.getState().setTokens(tokens)

    useAuthStore.getState().updateAccessToken('at-2')

    expect(useAuthStore.getState().tokens?.refreshToken).toBe('rt-1')
  })

  it('is a no-op when nobody is signed in, rather than creating a half-built session', () => {
    useAuthStore.getState().updateAccessToken('at-2')

    expect(useAuthStore.getState().tokens).toBeNull()
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })
})

describe('clear', () => {
  it('leaves no trace of the session in either storage', () => {
    useAuthStore.getState().setTokens(tokens)

    useAuthStore.getState().clear()

    expect(useAuthStore.getState().tokens).toBeNull()
    expect(localStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REMEMBER_KEY)).toBeNull()
    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(REMEMBER_KEY)).toBeNull()
  })

  it('clears a session-only login too', () => {
    useAuthStore.getState().setTokens(tokens, false)

    useAuthStore.getState().clear()

    expect(sessionStorage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    expect(sessionStorage.getItem(REMEMBER_KEY)).toBeNull()
  })

  it('resets initialized so the next initialize() re-runs instead of trusting a dead session', () => {
    useAuthStore.setState({ initialized: true })

    useAuthStore.getState().clear()

    expect(useAuthStore.getState().initialized).toBe(false)
  })
})
