import { describe, expect, it } from 'vitest'

import { inviteShareTargets } from './inviteShareTargets'

const ROOM_LINK = 'https://bedrud.test/m/abc-def-ghi'

describe('inviteShareTargets', () => {
  it('should offer the system share sheet, copy and QR where sharing is supported', () => {
    expect(inviteShareTargets(ROOM_LINK, true).map((target) => target.id)).toEqual(['share', 'copy', 'qr'])
  })

  it('should replace sharing with the named services where it is unsupported', () => {
    expect(inviteShareTargets(ROOM_LINK, false).map((target) => target.id)).toEqual([
      'copy',
      'qr',
      'email',
      'telegram',
      'whatsapp',
    ])
  })

  it('should never offer sharing alongside the services that stand in for it', () => {
    const supported = inviteShareTargets(ROOM_LINK, true).map((target) => target.id)
    expect(supported).toContain('share')
    expect(supported).not.toContain('telegram')
  })

  it('should escape the room link into every fallback href', () => {
    const targets = inviteShareTargets('https://bedrud.test/m/a b&c', false)
    const hrefs = targets.filter((target) => target.href !== undefined).map((target) => target.href)
    expect(hrefs).toEqual([
      'mailto:?body=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
      'https://t.me/share/url?url=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
      'https://wa.me/?text=https%3A%2F%2Fbedrud.test%2Fm%2Fa%20b%26c',
    ])
  })

  it('should give the targets the sheet handles itself no href', () => {
    const handled = inviteShareTargets(ROOM_LINK, true).filter((target) => target.href === undefined)
    expect(handled.map((target) => target.id)).toEqual(['share', 'copy', 'qr'])
  })
})
