import { describe, expect, it } from 'vitest'
import { validateSendUpdate } from './webxdcUpdate'

describe('validateSendUpdate', () => {
  it('accepts minimal payload', () => {
    const r = validateSendUpdate({ payload: { votes: 1 } })
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.update.payload).toEqual({ votes: 1 })
  })

  it('rejects missing or undefined payload', () => {
    expect(validateSendUpdate({}).ok).toBe(false)
    expect(validateSendUpdate({ payload: undefined }).ok).toBe(false)
  })

  it('allows null payload', () => {
    expect(validateSendUpdate({ payload: null }).ok).toBe(true)
  })

  it('rejects absolute href', () => {
    const r = validateSendUpdate({ payload: 1, href: 'https://evil.example' })
    expect(r.ok).toBe(false)
  })

  it('accepts relative href and truncates info', () => {
    const r = validateSendUpdate({
      payload: 'x',
      href: 'index.html#a',
      info: `${'a'.repeat(100)}\nline`,
    })
    expect(r.ok).toBe(true)
    if (r.ok) {
      expect(r.update.href).toBe('index.html#a')
      expect(r.update.info?.length).toBeLessThanOrEqual(50)
      expect(r.update.info).not.toContain('\n')
    }
  })

  it('enforces max serialized size', () => {
    const r = validateSendUpdate({ payload: 'x'.repeat(1000) }, 50)
    expect(r.ok).toBe(false)
  })

  it('validates notify map', () => {
    expect(validateSendUpdate({ payload: 1, notify: { '*': 'hi' } }).ok).toBe(true)
    expect(validateSendUpdate({ payload: 1, notify: { a: 1 } as unknown as Record<string, string> }).ok).toBe(
      false,
    )
  })
})
