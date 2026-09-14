import { describe, expect, it } from 'vitest'
import { parseJoinInput } from './join-input'

describe('parseJoinInput', () => {
  it('should take the room from a full meeting URL', () => {
    expect(parseJoinInput('https://bedrud.example/m/team-standup')).toBe('team-standup')
  })

  it('should take the room from the legacy call path', () => {
    expect(parseJoinInput('https://bedrud.example/c/team-standup')).toBe('team-standup')
  })

  it('should accept a URL with no scheme', () => {
    expect(parseJoinInput('bedrud.example/m/team-standup')).toBe('team-standup')
  })

  it('should accept a URL carrying a port', () => {
    expect(parseJoinInput('https://bedrud.example:8080/m/team-standup')).toBe('team-standup')
  })

  it('should ignore a query string and a fragment', () => {
    expect(parseJoinInput('https://bedrud.example/m/team-standup?from=email#top')).toBe('team-standup')
  })

  it('should accept a bare path', () => {
    expect(parseJoinInput('/m/team-standup')).toBe('team-standup')
  })

  // A plain name is the common case: someone reading a room name aloud over a call.
  it('should slugify a plain room name', () => {
    expect(parseJoinInput('Team Standup')).toBe('team-standup')
    expect(parseJoinInput('  Team   Standup  ')).toBe('team-standup')
  })

  // A URL that names no room must not fall through to slugification: the old code turned the whole
  // URL into a room name and navigated to a room that could never exist.
  it('should reject a URL with no room segment', () => {
    expect(parseJoinInput('https://bedrud.example/dashboard')).toBeNull()
    expect(parseJoinInput('https://bedrud.example/m/')).toBeNull()
  })

  it('should reject blank input', () => {
    expect(parseJoinInput('')).toBeNull()
    expect(parseJoinInput('   ')).toBeNull()
  })
})
