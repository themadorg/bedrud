import { describe, expect, it } from 'vitest'
import {
  deriveWebxdcSelfAddrKey,
  selfAddrsAreUnlinkableAcrossApps,
} from './webxdcSelfAddr'

describe('webxdcSelfAddr', () => {
  it('is stable for same user+app+room', () => {
    const a = deriveWebxdcSelfAddrKey({ roomId: 'r1', appId: 'a1', userId: 'u1' })
    const b = deriveWebxdcSelfAddrKey({ roomId: 'r1', appId: 'a1', userId: 'u1' })
    expect(a).toBe(b)
  })

  it('differs across app instances for the same user', () => {
    expect(
      selfAddrsAreUnlinkableAcrossApps(
        { roomId: 'r1', appId: 'app-a', userId: 'u1' },
        { roomId: 'r1', appId: 'app-b', userId: 'u1' },
      ),
    ).toBe(true)
  })

  it('same instance is not "unlinkable across apps"', () => {
    expect(
      selfAddrsAreUnlinkableAcrossApps(
        { roomId: 'r1', appId: 'app-a', userId: 'u1' },
        { roomId: 'r1', appId: 'app-a', userId: 'u1' },
      ),
    ).toBe(false)
  })
})
