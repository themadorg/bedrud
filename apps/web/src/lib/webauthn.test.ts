import { describe, expect, it } from 'vitest'
import { NetworkError } from './api'
import { base64ToBuffer, bufferToBase64, isPasskeyCancelled, passkeyErrorMessage } from './webauthn'

describe('passkeyErrorMessage', () => {
  it('names a passkey this device already holds', () => {
    expect(
      passkeyErrorMessage(
        new DOMException('The authenticator was previously registered.', 'InvalidStateError'),
        'failed',
      ),
    ).toBe('This device already has a passkey for this account')
  })

  it('names any other failure the way every other request does', () => {
    expect(passkeyErrorMessage(new NetworkError(), 'failed')).toBe("Couldn't reach the server")
    expect(passkeyErrorMessage(new DOMException('The operation is insecure.', 'SecurityError'), 'failed')).toBe(
      'failed',
    )
  })
})

describe('isPasskeyCancelled', () => {
  it('treats a closed passkey picker as cancelled', () => {
    expect(
      isPasskeyCancelled(new DOMException('The operation either timed out or was not allowed.', 'NotAllowedError')),
    ).toBe(true)
    expect(isPasskeyCancelled(new DOMException('The operation was aborted.', 'AbortError'))).toBe(true)
  })

  it('reports every other failure as a failure', () => {
    expect(isPasskeyCancelled(new DOMException('Already registered.', 'InvalidStateError'))).toBe(false)
    expect(isPasskeyCancelled(new Error('NotAllowedError'))).toBe(false)
    expect(isPasskeyCancelled(null)).toBe(false)
  })
})

function bytes(...values: number[]): ArrayBuffer {
  return new Uint8Array(values).buffer
}

function toArray(buffer: ArrayBuffer): number[] {
  return Array.from(new Uint8Array(buffer))
}

describe('webauthn base64url', () => {
  it('round-trips arbitrary bytes', () => {
    // 0xFB 0xFF forces both of the characters that differ between the
    // standard and URL-safe alphabets.
    const original = bytes(0, 1, 127, 128, 251, 255)

    expect(toArray(base64ToBuffer(bufferToBase64(original)))).toEqual(toArray(original))
  })

  it('round-trips every byte value', () => {
    const original = new Uint8Array(256).map((_, i) => i).buffer

    expect(toArray(base64ToBuffer(bufferToBase64(original)))).toEqual(toArray(original))
  })

  it('emits the URL-safe alphabet with no padding', () => {
    const encoded = bufferToBase64(bytes(251, 255, 190))

    // '+' and '/' would be mangled in a URL; '=' is not part of base64url.
    expect(encoded).not.toMatch(/[+/=]/)
    expect(encoded).toMatch(/^[A-Za-z0-9\-_]+$/)
  })

  it('produces the URL-safe substitutions where standard base64 would use + and /', () => {
    // 255,239,190 splits into the 6-bit groups 63,62,62,62 — standard base64
    // "/+++", so every character is one of the two that need substituting.
    expect(bufferToBase64(bytes(255, 239, 190))).toBe('_---')
  })

  it('decodes padded standard base64, which is what some servers send', () => {
    // The server side is not guaranteed to strip padding or use the URL-safe
    // alphabet, so decoding has to accept both forms.
    expect(toArray(base64ToBuffer('/+8='))).toEqual([255, 239])
    expect(toArray(base64ToBuffer('_-8='))).toEqual([255, 239])
  })

  it('handles the empty buffer in both directions', () => {
    expect(bufferToBase64(bytes())).toBe('')
    expect(toArray(base64ToBuffer(''))).toEqual([])
  })

  it('round-trips a realistic 32-byte challenge', () => {
    const challenge = new Uint8Array(32).map((_, i) => (i * 37) % 256).buffer

    const encoded = bufferToBase64(challenge)
    expect(encoded).not.toMatch(/[+/=]/)
    expect(toArray(base64ToBuffer(encoded))).toEqual(toArray(challenge))
  })
})
