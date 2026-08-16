import { describe, expect, it } from 'vitest'
import { wrapMarkdownBold } from './ChatInput'

describe('wrapMarkdownBold', () => {
  it('wraps a selection in **', () => {
    const r = wrapMarkdownBold('hello world', 6, 11)
    expect(r.text).toBe('hello **world**')
    expect(r.selectionStart).toBe(6)
    expect(r.selectionEnd).toBe(15)
  })

  it('unwraps when selection is already inside **', () => {
    const r = wrapMarkdownBold('hello **world**', 6, 15)
    expect(r.text).toBe('hello world')
    expect(r.selectionStart).toBe(6)
    expect(r.selectionEnd).toBe(11)
  })

  it('unwraps when markers sit just outside the selection', () => {
    const r = wrapMarkdownBold('hello **world**', 8, 13)
    expect(r.text).toBe('hello world')
    expect(r.selectionStart).toBe(6)
    expect(r.selectionEnd).toBe(11)
  })

  it('inserts markers for an empty selection', () => {
    const r = wrapMarkdownBold('ab', 1, 1)
    expect(r.text).toBe('a****b')
    expect(r.selectionStart).toBe(3)
    expect(r.selectionEnd).toBe(3)
  })
})
