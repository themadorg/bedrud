import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import {
  dismissPasskeyOffer,
  loginWithPasskey,
  passkeyErrorMessage,
  shouldOfferPasskey,
  signupWithPasskey,
} from './passkey'

const CHALLENGE = 'AAECAw' // base64url of bytes 0..3
const buf = () => new Uint8Array([1, 2, 3]).buffer

const AUTH_RESPONSE = {
  user: { id: 'u1', email: 'a@ex.com', name: 'A', provider: 'local', accesses: ['user'] },
  tokens: { accessToken: 'at', refreshToken: 'rt' },
}

const credentials = { get: vi.fn(), create: vi.fn() }
let platformAuthenticator = true

beforeEach(() => {
  Object.defineProperty(navigator, 'credentials', { value: credentials, configurable: true })
  const PublicKeyCredential = Object.assign(() => {}, {
    isUserVerifyingPlatformAuthenticatorAvailable: async () => platformAuthenticator,
  })
  Object.defineProperty(window, 'PublicKeyCredential', { value: PublicKeyCredential, configurable: true })
  platformAuthenticator = true
  localStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
  credentials.get.mockReset()
  credentials.create.mockReset()
})

describe('loginWithPasskey', () => {
  function assertion() {
    return {
      rawId: buf(),
      response: { clientDataJSON: buf(), authenticatorData: buf(), signature: buf() },
    }
  }

  it('needs no email: begins with an empty body and lets the browser list its passkeys', async () => {
    const post = vi.spyOn(api, 'post').mockImplementation(async (path: string) => {
      if (path.endsWith('/login/begin')) return { challenge: CHALLENGE }
      return AUTH_RESPONSE
    })
    credentials.get.mockResolvedValue(assertion())

    await expect(loginWithPasskey()).resolves.toEqual(AUTH_RESPONSE)

    expect(post).toHaveBeenCalledWith('/api/auth/passkey/login/begin', {})
    expect(credentials.get.mock.calls[0][0].publicKey.allowCredentials).toBeUndefined()
  })

  // Throwing here used to tell anyone typing an address whether it had a passkey.
  it('falls back to the plain dialog when the typed email has no passkeys', async () => {
    const post = vi.spyOn(api, 'post').mockImplementation(async (path: string) => {
      if (path.endsWith('/login/begin')) return { challenge: CHALLENGE, allowCredentials: [] }
      return AUTH_RESPONSE
    })
    credentials.get.mockResolvedValue(assertion())

    await expect(loginWithPasskey('a@ex.com')).resolves.toEqual(AUTH_RESPONSE)

    expect(post).toHaveBeenCalledWith('/api/auth/passkey/login/begin', { email: 'a@ex.com' })
    expect(credentials.get.mock.calls[0][0].publicKey.allowCredentials).toBeUndefined()
  })

  it('narrows the dialog to the account passkeys when the typed email has some', async () => {
    vi.spyOn(api, 'post').mockImplementation(async (path: string) => {
      if (path.endsWith('/login/begin')) {
        return { challenge: CHALLENGE, allowCredentials: [{ id: 'AQID', type: 'public-key' }] }
      }
      return AUTH_RESPONSE
    })
    credentials.get.mockResolvedValue(assertion())

    await loginWithPasskey('a@ex.com')

    const allow = credentials.get.mock.calls[0][0].publicKey.allowCredentials
    expect(allow).toHaveLength(1)
    expect(Array.from(new Uint8Array(allow[0].id))).toEqual([1, 2, 3])
  })
})

describe('signupWithPasskey', () => {
  const creationOptions = {
    rp: { id: 'localhost', name: 'localhost' },
    user: { id: 'dXNlcg', name: 'a@ex.com', displayName: 'A' },
    challenge: CHALLENGE,
    pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
  }

  function mockSignup() {
    credentials.create.mockResolvedValue({
      rawId: buf(),
      response: { clientDataJSON: buf(), attestationObject: buf() },
    })
    return vi.spyOn(api, 'post').mockImplementation(async (path: string) => {
      if (path.endsWith('/signup/begin')) return creationOptions
      return AUTH_RESPONSE
    })
  }

  // Invite-only servers refuse a passkey signup without the token.
  it('sends the invite token with the signup', async () => {
    const post = mockSignup()

    await signupWithPasskey('A', 'a@ex.com', 'invite-123')

    expect(post).toHaveBeenCalledWith('/api/auth/passkey/signup/begin', {
      name: 'A',
      email: 'a@ex.com',
      inviteToken: 'invite-123',
    })
  })

  it('leaves the invite token out when there is none', async () => {
    const post = mockSignup()

    await signupWithPasskey('A', 'a@ex.com')

    expect(post).toHaveBeenCalledWith('/api/auth/passkey/signup/begin', { name: 'A', email: 'a@ex.com' })
  })
})

describe('shouldOfferPasskey', () => {
  it('offers when the device has a built-in authenticator and the account has no passkey', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ passkeys: [] })

    await expect(shouldOfferPasskey('u1')).resolves.toBe(true)
  })

  it('does not offer to an account that already has a passkey', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ passkeys: [{ id: 'p1', name: 'Passkey', createdAt: '' }] })

    await expect(shouldOfferPasskey('u1')).resolves.toBe(false)
  })

  it('does not offer on a device without a built-in authenticator', async () => {
    platformAuthenticator = false
    const get = vi.spyOn(api, 'get').mockResolvedValue({ passkeys: [] })

    await expect(shouldOfferPasskey('u1')).resolves.toBe(false)
    expect(get).not.toHaveBeenCalled()
  })

  it('stays quiet for a user who turned it down, and only for that user', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ passkeys: [] })

    dismissPasskeyOffer('u1')

    await expect(shouldOfferPasskey('u1')).resolves.toBe(false)
    await expect(shouldOfferPasskey('u2')).resolves.toBe(true)
  })

  it('answers no when the passkey list cannot be read', async () => {
    vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))

    await expect(shouldOfferPasskey('u1')).resolves.toBe(false)
  })
})

describe('passkeyErrorMessage', () => {
  it('explains a cancelled or timed-out browser prompt', () => {
    const err = new DOMException('The operation either timed out or was not allowed.', 'NotAllowedError')

    expect(passkeyErrorMessage(err, 'fallback')).toBe('The passkey request was cancelled or timed out.')
  })

  it('explains an authenticator that already holds a passkey for the account', () => {
    const err = new DOMException('excluded', 'InvalidStateError')

    expect(passkeyErrorMessage(err, 'fallback')).toBe('This device already holds a passkey for your account.')
  })

  it('passes API messages through and falls back otherwise', () => {
    expect(passkeyErrorMessage(new Error('Invite token already used or invalid'), 'fallback')).toBe(
      'Invite token already used or invalid',
    )
    expect(passkeyErrorMessage('nope', 'fallback')).toBe('fallback')
  })
})
