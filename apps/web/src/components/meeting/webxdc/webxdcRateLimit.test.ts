import { describe, expect, it } from 'vitest'
import { WebxdcSendUpdateRateLimiter } from './webxdcRateLimit'

describe('WebxdcSendUpdateRateLimiter', () => {
  it('allows first send and blocks until interval elapses', () => {
    const lim = new WebxdcSendUpdateRateLimiter(10_000)
    expect(lim.tryTake('app-1', 0)).toBe(true)
    expect(lim.tryTake('app-1', 5_000)).toBe(false)
    expect(lim.tryTake('app-1', 10_000)).toBe(true)
  })

  it('tracks keys independently', () => {
    const lim = new WebxdcSendUpdateRateLimiter(10_000)
    expect(lim.tryTake('a', 0)).toBe(true)
    expect(lim.tryTake('b', 0)).toBe(true)
    expect(lim.tryTake('a', 1)).toBe(false)
  })

  it('reports ms until ready', () => {
    const lim = new WebxdcSendUpdateRateLimiter(10_000)
    expect(lim.msUntilReady('x', 0)).toBe(0)
    expect(lim.tryTake('x', 0)).toBe(true)
    expect(lim.msUntilReady('x', 3_000)).toBe(7_000)
    expect(lim.msUntilReady('x', 10_000)).toBe(0)
  })
})
