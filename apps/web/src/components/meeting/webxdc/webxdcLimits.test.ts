import { describe, expect, it } from 'vitest'
import {
  WEBXDC_REALTIME_MAX_SIZE,
  WEBXDC_SEND_UPDATE_INTERVAL_MS,
  WEBXDC_SEND_UPDATE_MAX_SIZE,
} from './webxdcConstants'

describe('official WebXDC limits', () => {
  it('matches sendUpdate defaults from the spec', () => {
    expect(WEBXDC_SEND_UPDATE_INTERVAL_MS).toBe(10_000)
    expect(WEBXDC_SEND_UPDATE_MAX_SIZE).toBe(128_000)
    expect(WEBXDC_REALTIME_MAX_SIZE).toBe(128_000)
  })
})
