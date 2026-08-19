import { describe, expect, it } from 'vitest'
import { getErrorMessage } from './errors'

const fallback = 'Something went wrong'

describe('getErrorMessage', () => {
  it('reads message, error and detail from a JSON body', () => {
    expect(getErrorMessage(new Error('{"message":"name is required"}'), fallback)).toBe('name is required')
    expect(getErrorMessage(new Error('{"error":"room name taken"}'), fallback)).toBe('room name taken')
    expect(getErrorMessage(new Error('{"detail":"quota exceeded"}'), fallback)).toBe('quota exceeded')
  })

  it('prefers message over the other two', () => {
    expect(getErrorMessage(new Error('{"message":"first","error":"second","detail":"third"}'), fallback)).toBe('first')
  })

  it('strips a leading status code, which is how the server prefixes some errors', () => {
    expect(getErrorMessage(new Error('404: {"error":"room not found"}'), fallback)).toBe('room not found')
    expect(getErrorMessage(new Error('500: upstream unavailable'), fallback)).toBe('upstream unavailable')
  })

  it('unwraps a bare JSON string', () => {
    expect(getErrorMessage(new Error('"just a string"'), fallback)).toBe('just a string')
  })

  it('uses a plain-text message as-is', () => {
    expect(getErrorMessage(new Error('upstream unavailable'), fallback)).toBe('upstream unavailable')
  })

  it('accepts a raw string as well as an Error', () => {
    expect(getErrorMessage('network request failed', fallback)).toBe('network request failed')
  })

  it('falls back when there is nothing usable to show', () => {
    expect(getErrorMessage(new Error(''), fallback)).toBe(fallback)
    expect(getErrorMessage(new Error('   '), fallback)).toBe(fallback)
    expect(getErrorMessage(null, fallback)).toBe(fallback)
    expect(getErrorMessage(undefined, fallback)).toBe(fallback)
    expect(getErrorMessage({ unexpected: true }, fallback)).toBe(fallback)
  })

  it('returns the raw body when the JSON has no human-readable field', () => {
    // {"code":42} would otherwise surface to the user as raw JSON.
    expect(getErrorMessage(new Error('{"code":42}'), fallback)).toBe('{"code":42}')
  })

  it('lets a blank message field mask a usable error field', () => {
    // Documents current behaviour rather than endorsing it. The `??` chain only
    // skips null and undefined, so a present-but-blank `message` wins over
    // `error`, and the blank fails the trim check — leaving the raw JSON as the
    // user-facing string. Harmless unless the server starts sending both.
    expect(getErrorMessage(new Error('{"message":"   ","error":"real problem"}'), fallback)).toBe(
      '{"message":"   ","error":"real problem"}',
    )
  })
})
