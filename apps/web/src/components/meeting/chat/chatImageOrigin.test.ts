import { describe, expect, it } from 'vitest'
import { isOnServer } from './chatImageOrigin'

// The tests run without VITE_API_URL, so the account's server is this page's own origin.
const server = new URL(window.location.origin)

describe('isOnServer', () => {
  it('accepts an upload path on the account server', () => {
    expect(isOnServer('/api/room/r1/chat/uploads/a.png')).toBe(true)
  })

  it('accepts a full address on the account server', () => {
    expect(isOnServer(`${server.origin}/api/room/r1/chat/uploads/a.png`)).toBe(true)
  })

  it('turns away another host', () => {
    expect(isOnServer('https://images.example.test/a.png')).toBe(false)
  })

  it('turns away a host that only starts with the server name', () => {
    expect(isOnServer(`${server.protocol}//${server.hostname}.example.test/a.png`)).toBe(false)
  })

  it('turns away a user-info prefix naming the server', () => {
    expect(isOnServer(`${server.protocol}//${server.host}@example.test/a.png`)).toBe(false)
  })

  it('turns away another port on the same host', () => {
    expect(isOnServer(`${server.protocol}//${server.hostname}:1/a.png`)).toBe(false)
  })

  it('turns away the same host over another scheme', () => {
    const otherScheme = server.protocol === 'https:' ? 'http:' : 'https:'
    expect(isOnServer(`${otherScheme}//${server.host}/a.png`)).toBe(false)
  })

  it('turns away an address that does not parse', () => {
    expect(isOnServer('http://')).toBe(false)
  })
})
