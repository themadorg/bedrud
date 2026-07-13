import { describe, expect, it } from 'vitest'
import { validateSendUpdate } from './webxdcUpdate'

describe('validateSendUpdate extra', () => {
  it('rejects non-object root', () => {
    expect(validateSendUpdate(null).ok).toBe(false)
    expect(validateSendUpdate([]).ok).toBe(false)
    expect(validateSendUpdate('x').ok).toBe(false)
  })

  it('accepts primitive payloads', () => {
    expect(validateSendUpdate({ payload: 'hi' }).ok).toBe(true)
    expect(validateSendUpdate({ payload: 0 }).ok).toBe(true)
    expect(validateSendUpdate({ payload: false }).ok).toBe(true)
    expect(validateSendUpdate({ payload: [1, 2] }).ok).toBe(true)
  })

  it('truncates document and summary', () => {
    const r = validateSendUpdate({
      payload: 1,
      document: 'd'.repeat(50),
      summary: 's'.repeat(50),
    })
    expect(r.ok).toBe(true)
    if (r.ok) {
      expect(r.update.document?.length).toBeLessThanOrEqual(20)
      expect(r.update.summary?.length).toBeLessThanOrEqual(20)
    }
  })

  it('rejects non-string info', () => {
    expect(validateSendUpdate({ payload: 1, info: 9 as unknown as string }).ok).toBe(false)
  })
})
