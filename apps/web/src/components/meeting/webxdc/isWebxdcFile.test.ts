import { describe, expect, it } from 'vitest'
import { isWebxdcFile } from './webxdcFile'

describe('isWebxdcFile', () => {
  it('accepts .xdc extension', () => {
    expect(isWebxdcFile(new File([], 'app.xdc'))).toBe(true)
    expect(isWebxdcFile(new File([], 'APP.XDC'))).toBe(true)
  })

  it('rejects zip without .xdc extension', () => {
    expect(isWebxdcFile(new File([], 'pack.zip', { type: 'application/zip' }))).toBe(false)
  })

  it('rejects images', () => {
    expect(isWebxdcFile(new File([], 'pic.png', { type: 'image/png' }))).toBe(false)
  })
})
