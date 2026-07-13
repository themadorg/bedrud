import { describe, expect, it } from 'vitest'
import { WEBXDC_REALTIME_TOPIC, WEBXDC_STATUS_TOPIC } from './webxdcConstants'

describe('webxdc topics', () => {
  it('uses distinct status and realtime topics', () => {
    expect(WEBXDC_STATUS_TOPIC).toBe('webxdc')
    expect(WEBXDC_REALTIME_TOPIC).toBe('webxdc-rt')
    expect(WEBXDC_STATUS_TOPIC).not.toBe(WEBXDC_REALTIME_TOPIC)
  })
})
