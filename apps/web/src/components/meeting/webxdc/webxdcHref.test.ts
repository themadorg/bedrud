import { describe, expect, it } from 'vitest'
import { isSafeRelativeWebxdcHref, resolveWebxdcHref } from './webxdcHref'

describe('resolveWebxdcHref', () => {
  it('accepts relative paths and hashes', () => {
    expect(resolveWebxdcHref('index.html#about')).toBe('/index.html#about')
    expect(resolveWebxdcHref('#section')).toBe('/#section')
    expect(resolveWebxdcHref('pages/a.html?x=1')).toBe('/pages/a.html?x=1')
  })

  it('rejects absolute and protocol-relative URLs', () => {
    expect(resolveWebxdcHref('https://evil.example/')).toBeNull()
    expect(resolveWebxdcHref('http://evil.example/')).toBeNull()
    expect(resolveWebxdcHref('//evil.example/x')).toBeNull()
    expect(resolveWebxdcHref('javascript:alert(1)')).toBeNull()
    expect(resolveWebxdcHref('data:text/html,hi')).toBeNull()
  })

  it('isSafeRelativeWebxdcHref mirrors resolve', () => {
    expect(isSafeRelativeWebxdcHref('foo.html')).toBe(true)
    expect(isSafeRelativeWebxdcHref('https://x')).toBe(false)
  })
})
