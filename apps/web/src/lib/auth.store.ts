import { create } from 'zustand'

export interface AuthTokens {
  accessToken: string
  refreshToken: string | null
}

const REMEMBER_KEY = 'auth_remember'
const ACCESS_TOKEN_KEY = 'auth_at'

interface AuthStore {
  tokens: AuthTokens | null
  initialized: boolean
  setTokens: (tokens: AuthTokens, remember?: boolean | 'ephemeral') => void
  updateAccessToken: (accessToken: string) => void
  clear: () => void
  initialize: () => Promise<void>
}

const BASE_URL = (import.meta.env['VITE_API_URL'] as string | undefined) ?? ''

/**
 * The storage mode the current session already uses.
 *
 * Token refreshes must pass this to setTokens. Letting `remember` fall back to
 * its default would move a session-only login into localStorage, so a session
 * the user deliberately kept ephemeral — a guest joining a meeting on a shared
 * machine — would start surviving browser restarts.
 *
 * A session created with `false` reads back as `'ephemeral'`. No information is
 * lost: setTokens stores both in sessionStorage and treats them identically.
 *
 * With no session on record there is nothing to preserve, so a first login is
 * remembered, as it always has been.
 */
export function currentRememberMode(): boolean | 'ephemeral' {
  if (localStorage.getItem(REMEMBER_KEY)) return true
  if (sessionStorage.getItem(REMEMBER_KEY)) return 'ephemeral'
  return true
}

const _init = { promise: null as Promise<void> | null }

export const useAuthStore = create<AuthStore>()((set, get) => ({
  tokens: null,
  initialized: false,

  setTokens: (tokens, remember = true) => {
    set({ tokens })

    // Exactly one storage owns the session. Writing one without clearing the
    // other leaves keys from a previous login behind, and currentRememberMode
    // reads localStorage first — so a leftover remembered session would make a
    // later ephemeral one read back as remembered and get promoted on its first
    // refresh. Callers do not reliably clear() in between; the guest join path
    // does not.
    const [owner, other] = remember === true ? [localStorage, sessionStorage] : [sessionStorage, localStorage]

    other.removeItem(REMEMBER_KEY)
    other.removeItem(ACCESS_TOKEN_KEY)
    owner.setItem(REMEMBER_KEY, '1')
    owner.setItem(ACCESS_TOKEN_KEY, tokens.accessToken)
  },

  updateAccessToken: (accessToken) => {
    const current = get().tokens
    if (!current) return
    set({ tokens: { ...current, accessToken } })
    const storage = localStorage.getItem(REMEMBER_KEY) ? localStorage : sessionStorage
    storage.setItem(ACCESS_TOKEN_KEY, accessToken)
  },

  clear: () => {
    set({ tokens: null, initialized: false })
    _init.promise = null
    localStorage.removeItem(REMEMBER_KEY)
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    sessionStorage.removeItem(REMEMBER_KEY)
    sessionStorage.removeItem(ACCESS_TOKEN_KEY)
  },

  initialize: async () => {
    if (get().initialized) return

    // Deduplicate: if an initialize() call is already in-flight, reuse it.
    if (_init.promise) return _init.promise

    _init.promise = (async () => {
      const wasRemembered = Boolean(localStorage.getItem(REMEMBER_KEY)) || Boolean(sessionStorage.getItem(REMEMBER_KEY))

      // Try cookie-based refresh first (primary path).
      // Always attempt this — email verification sets cookies but not REMEMBER_KEY.
      try {
        const res = await fetch(`${BASE_URL}/api/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
        })

        if (res.ok) {
          const data = (await res.json()) as { access_token: string; refresh_token: string }
          get().setTokens({ accessToken: data.access_token, refreshToken: data.refresh_token }, currentRememberMode())
          set({ initialized: true })
          return
        }
      } catch {
        // Network error — fall through to persisted token
      }

      // Fallback: use the persisted access token (only if user was previously logged in).
      if (wasRemembered) {
        const storage = localStorage.getItem(REMEMBER_KEY) ? localStorage : sessionStorage
        const persistedAT = storage.getItem(ACCESS_TOKEN_KEY)
        if (persistedAT) {
          try {
            const meRes = await fetch(`${BASE_URL}/api/auth/me`, {
              headers: { Authorization: `Bearer ${persistedAT}` },
              credentials: 'include',
            })
            if (meRes.ok) {
              get().setTokens({ accessToken: persistedAT, refreshToken: null }, currentRememberMode())
              set({ initialized: true })
              return
            }
          } catch {
            // Token expired — fall through to clear
          }
        }
      }

      // Both paths failed — clear session
      get().clear()
      set({ initialized: true })
    })().finally(() => {
      // Released on every outcome, not just failure. The `initialized` guard
      // above already stops a second run, so holding a settled promise here
      // only means that resetting `initialized` without clear() would make
      // initialize() a silent no-op.
      _init.promise = null
    })

    return _init.promise
  },
}))
