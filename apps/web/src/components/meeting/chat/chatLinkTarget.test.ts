// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { resolveChatLink } from './chatLinkTarget'

const ORIGIN = 'https://meet.example.com'

describe('resolveChatLink', () => {
  it('should treat a link to another site as a page', () => {
    expect(resolveChatLink('https://other.example.com/m/standup', ORIGIN, 'weekly')).toEqual({ kind: 'page' })
  })

  it('should treat a non-room path on this server as a page', () => {
    expect(resolveChatLink(`${ORIGIN}/dashboard`, ORIGIN, 'weekly')).toEqual({ kind: 'page' })
  })

  it('should resolve another room on this server', () => {
    expect(resolveChatLink(`${ORIGIN}/m/standup`, ORIGIN, 'weekly')).toEqual({ kind: 'room', roomName: 'standup' })
  })

  it('should resolve a relative room link against this server', () => {
    expect(resolveChatLink('/m/standup/', ORIGIN, 'weekly')).toEqual({ kind: 'room', roomName: 'standup' })
  })

  it('should recognise the current room whatever the case of its name or host', () => {
    expect(resolveChatLink('https://MEET.example.com/m/Weekly', ORIGIN, 'weekly')).toEqual({ kind: 'currentRoom' })
  })

  it('should treat the current room name on another server as a page', () => {
    expect(resolveChatLink('https://other.example.com/m/weekly', ORIGIN, 'weekly')).toEqual({ kind: 'page' })
  })

  it('should treat a path below a room as a page', () => {
    expect(resolveChatLink(`${ORIGIN}/m/standup/extra`, ORIGIN, 'weekly')).toEqual({ kind: 'page' })
  })
})
