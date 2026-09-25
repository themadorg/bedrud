import { api } from '#/lib/api'
import type { AuthResponse } from '#/lib/handle-auth-success'
import {
  base64ToBuffer,
  bufferToBase64,
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
} from '#/lib/webauthn'

export interface PasskeyVerificationRequired {
  requiresVerification: true
  email: string
  message: string
}

export type PasskeySignupResponse = AuthResponse | PasskeyVerificationRequired

export interface PasskeySummary {
  id: string
  name: string
  createdAt: string
}

/** True when this browser can create and use passkeys at all. */
export function passkeysSupported(): boolean {
  return typeof window !== 'undefined' && typeof window.PublicKeyCredential === 'function'
}

type PublicKeyCredentialStatics = {
  isConditionalMediationAvailable?: () => Promise<boolean>
  isUserVerifyingPlatformAuthenticatorAvailable?: () => Promise<boolean>
}

async function askBrowser(check: keyof PublicKeyCredentialStatics): Promise<boolean> {
  if (!passkeysSupported()) return false
  const statics = window.PublicKeyCredential as unknown as PublicKeyCredentialStatics
  const fn = statics[check]
  if (typeof fn !== 'function') return false
  try {
    return await fn.call(statics)
  } catch {
    return false
  }
}

/** True when the browser can list saved passkeys in a field's autofill (conditional mediation). */
export function passkeyAutofillSupported(): Promise<boolean> {
  return askBrowser('isConditionalMediationAvailable')
}

function toRequestOptions(opts: PublicKeyCredentialRequestOptionsJSON): PublicKeyCredentialRequestOptions {
  const publicKey: PublicKeyCredentialRequestOptions = {
    challenge: base64ToBuffer(opts.challenge),
    timeout: opts.timeout,
    rpId: opts.rpId,
    userVerification: opts.userVerification ?? 'preferred',
  }
  if (opts.allowCredentials?.length) {
    publicKey.allowCredentials = opts.allowCredentials.map((c) => ({
      id: base64ToBuffer(c.id),
      type: 'public-key' as const,
    }))
  }
  return publicKey
}

function finishLogin(cred: PublicKeyCredential): Promise<AuthResponse> {
  const assertion = cred.response as AuthenticatorAssertionResponse
  return api.post<AuthResponse>('/api/auth/passkey/login/finish', {
    credentialId: bufferToBase64(cred.rawId),
    clientDataJSON: bufferToBase64(assertion.clientDataJSON),
    authenticatorData: bufferToBase64(assertion.authenticatorData),
    signature: bufferToBase64(assertion.signature),
  })
}

/**
 * Signs in through the browser's passkey dialog. No email is needed: passkeys are discoverable,
 * so the browser lists the ones it holds for this site.
 *
 * `emailHint` is whatever the email field already holds. It is passed along so a passkey saved
 * before discoverable credentials were required can still be offered. An address with no passkeys
 * falls back to the plain dialog, so the page never reveals whether an address has one.
 */
export async function loginWithPasskey(emailHint?: string): Promise<AuthResponse> {
  const email = emailHint?.trim()
  const opts = await api.post<PublicKeyCredentialRequestOptionsJSON>(
    '/api/auth/passkey/login/begin',
    email ? { email } : {},
  )
  const cred = (await navigator.credentials.get({ publicKey: toRequestOptions(opts) })) as PublicKeyCredential | null
  if (!cred) throw new Error('Passkey sign-in was cancelled')
  return finishLogin(cred)
}

/**
 * Offers saved passkeys in the autofill of the field marked `autocomplete="username webauthn"`.
 * Resolves with the new session once one is picked, or null when the request is aborted or the
 * browser cannot run it. Autofill is a convenience, so failures before a passkey is picked stay
 * silent; a failed sign-in after one is picked rejects so the page can report it.
 *
 * A browser serves one WebAuthn request at a time: abort `signal` before starting another.
 */
export async function autofillPasskeyLogin(signal: AbortSignal): Promise<AuthResponse | null> {
  let publicKey: PublicKeyCredentialRequestOptions
  try {
    publicKey = toRequestOptions(
      await api.post<PublicKeyCredentialRequestOptionsJSON>('/api/auth/passkey/login/begin', {}),
    )
  } catch {
    return null
  }
  if (signal.aborted) return null

  let cred: PublicKeyCredential | null
  try {
    cred = (await navigator.credentials.get({
      mediation: 'conditional',
      publicKey,
      signal,
    })) as PublicKeyCredential | null
  } catch {
    return null
  }
  if (!cred) return null
  return finishLogin(cred)
}

function createCredential(opts: PublicKeyCredentialCreationOptionsJSON): Promise<Credential | null> {
  const publicKey: PublicKeyCredentialCreationOptions = {
    rp: opts.rp,
    user: { id: base64ToBuffer(opts.user.id), name: opts.user.name, displayName: opts.user.displayName },
    challenge: base64ToBuffer(opts.challenge),
    pubKeyCredParams: opts.pubKeyCredParams,
    timeout: opts.timeout,
    attestation: opts.attestation,
    authenticatorSelection: opts.authenticatorSelection,
  }
  if (opts.excludeCredentials?.length) {
    publicKey.excludeCredentials = opts.excludeCredentials.map((c) => ({
      id: base64ToBuffer(c.id),
      type: 'public-key' as const,
    }))
  }
  return navigator.credentials.create({ publicKey })
}

function attestationBody(cred: PublicKeyCredential) {
  const att = cred.response as AuthenticatorAttestationResponse
  return {
    clientDataJSON: bufferToBase64(att.clientDataJSON),
    attestationObject: bufferToBase64(att.attestationObject),
  }
}

/** Creates an account whose only credential is a new passkey. */
export async function signupWithPasskey(
  name: string,
  email: string,
  inviteToken?: string,
): Promise<PasskeySignupResponse> {
  const body: Record<string, string> = { name, email }
  if (inviteToken) body.inviteToken = inviteToken
  const opts = await api.post<PublicKeyCredentialCreationOptionsJSON>('/api/auth/passkey/signup/begin', body)
  const cred = (await createCredential(opts)) as PublicKeyCredential | null
  if (!cred) throw new Error('Passkey setup was cancelled')
  return api.post<PasskeySignupResponse>('/api/auth/passkey/signup/finish', attestationBody(cred))
}

/** Adds a passkey to the signed-in account. */
export async function registerPasskey(): Promise<void> {
  const opts = await api.post<PublicKeyCredentialCreationOptionsJSON>('/api/auth/passkey/register/begin', {})
  const cred = (await createCredential(opts)) as PublicKeyCredential | null
  if (!cred) throw new Error('Passkey setup was cancelled')
  await api.post('/api/auth/passkey/register/finish', attestationBody(cred))
}

/** The signed-in account's passkeys. */
export async function listPasskeys(): Promise<PasskeySummary[]> {
  const res = await api.get<{ passkeys: PasskeySummary[] | null }>('/api/auth/passkeys')
  return res.passkeys ?? []
}

/** Turns a WebAuthn or API failure into a sentence a person can act on. */
export function passkeyErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof DOMException) {
    if (err.name === 'NotAllowedError' || err.name === 'AbortError') {
      return 'The passkey request was cancelled or timed out.'
    }
    if (err.name === 'InvalidStateError') {
      return 'This device already holds a passkey for your account.'
    }
  }
  return err instanceof Error && err.message ? err.message : fallback
}

// ── Offer to add a passkey after a password sign-in ─────────────────────────

const OFFER_DISMISSED_KEY = 'passkey_offer_dismissed'

function dismissedUserIds(): string[] {
  try {
    const parsed = JSON.parse(localStorage.getItem(OFFER_DISMISSED_KEY) ?? '[]')
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : []
  } catch {
    return []
  }
}

/** Remembers, on this browser, that this user turned the passkey offer down. */
export function dismissPasskeyOffer(userId: string) {
  const ids = dismissedUserIds()
  if (ids.includes(userId)) return
  try {
    localStorage.setItem(OFFER_DISMISSED_KEY, JSON.stringify([...ids, userId]))
  } catch {
    // Storage unavailable (private mode, blocked site data): the offer simply comes back next time.
  }
}

/**
 * Whether to offer adding a passkey right after a password sign-in: this device has a built-in
 * authenticator (fingerprint, face, screen lock), the account has no passkey yet, and the user
 * has not turned the offer down on this browser. Any failure answers no, so the offer never
 * stands between a user and the app.
 */
export async function shouldOfferPasskey(userId: string): Promise<boolean> {
  if (dismissedUserIds().includes(userId)) return false
  if (!(await askBrowser('isUserVerifyingPlatformAuthenticatorAvailable'))) return false
  try {
    return (await listPasskeys()).length === 0
  } catch {
    return false
  }
}
