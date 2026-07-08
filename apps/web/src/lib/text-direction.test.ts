import { describe, expect, it } from 'vitest'
import { textDirectionFor, textStartsRtl } from './text-direction'

describe('text-direction', () => {
  it('detects Persian/Farsi at the start', () => {
    expect(textStartsRtl('سلام')).toBe(true)
    expect(textDirectionFor('سلام')).toBe('rtl')
  })

  it('ignores leading neutral characters before RTL text', () => {
    expect(textStartsRtl('  123 سلام')).toBe(true)
  })

  it('keeps Latin text left-to-right', () => {
    expect(textStartsRtl('Hello')).toBe(false)
    expect(textDirectionFor('Hello')).toBe('ltr')
  })

  it('uses the first strong character for mixed text', () => {
    expect(textStartsRtl('Hello سلام')).toBe(false)
    expect(textStartsRtl('سلام hello')).toBe(true)
  })
})
