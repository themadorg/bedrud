import { getErrorMessage } from './errors'

/**
 * Reports whether a passkey ceremony ended because the reader closed the picker. Browsers report
 * that, and a picker left to time out, as NotAllowedError; it was the reader's own choice, so it
 * is not shown as a failure.
 */
export function isPasskeyCancelled(error: unknown): boolean {
  return error instanceof DOMException && (error.name === 'NotAllowedError' || error.name === 'AbortError')
}

const PASSKEY_ALREADY_REGISTERED = 'This device already has a passkey for this account'

/** Returns the reader-facing message for a failed passkey ceremony, naming the failures a passkey has of its own. */
export function passkeyErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof DOMException && error.name === 'InvalidStateError') return PASSKEY_ALREADY_REGISTERED
  return getErrorMessage(error, fallback)
}

export function base64ToBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64.replace(/-/g, '+').replace(/_/g, '/'))
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

export function bufferToBase64(buffer: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

/** Subset of the WebAuthn options returned by the server's begin endpoints. */
export interface PublicKeyCredentialRequestOptionsJSON {
  challenge: string
  timeout?: number
  rpId?: string
  userVerification?: UserVerificationRequirement
  allowCredentials?: Array<{ id: string; type: 'public-key' }>
}

export interface PublicKeyCredentialCreationOptionsJSON {
  rp: PublicKeyCredentialRpEntity
  user: { id: string; name: string; displayName: string }
  challenge: string
  pubKeyCredParams: PublicKeyCredentialParameters[]
  timeout?: number
  attestation?: AttestationConveyancePreference
  authenticatorSelection?: AuthenticatorSelectionCriteria
}
