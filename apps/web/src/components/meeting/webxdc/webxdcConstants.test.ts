import { describe, expect, it } from 'vitest'
import {
  WEBXDC_HOST_SCRIPT,
  WEBXDC_POSTMESSAGE_CHANNEL,
  WEBXDC_REALTIME_MAX_SIZE,
  WEBXDC_REALTIME_TOPIC,
  WEBXDC_SEND_UPDATE_INTERVAL_MS,
  WEBXDC_SEND_UPDATE_MAX_SIZE,
  WEBXDC_STATUS_TOPIC,
} from './webxdcConstants'

describe('webxdcConstants', () => {
  it('exports stable protocol identifiers', () => {
    expect(WEBXDC_STATUS_TOPIC).toBe('webxdc')
    expect(WEBXDC_REALTIME_TOPIC).toBe('webxdc-rt')
    expect(WEBXDC_POSTMESSAGE_CHANNEL).toBe('bedrud-webxdc')
    expect(WEBXDC_HOST_SCRIPT).toBe('webxdc.js')
  })

  it('matches official size/interval defaults', () => {
    expect(WEBXDC_SEND_UPDATE_MAX_SIZE).toBe(128_000)
    expect(WEBXDC_REALTIME_MAX_SIZE).toBe(128_000)
    expect(WEBXDC_SEND_UPDATE_INTERVAL_MS).toBe(10_000)
  })
})
