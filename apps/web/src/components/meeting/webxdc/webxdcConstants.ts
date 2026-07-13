/** LiveKit / host constants from docs/plan/webxdc. */

export const WEBXDC_STATUS_TOPIC = 'webxdc' as const
export const WEBXDC_REALTIME_TOPIC = 'webxdc-rt' as const

/** Official defaults (webxdc.org sendUpdate). */
export const WEBXDC_SEND_UPDATE_INTERVAL_MS = 10_000
export const WEBXDC_SEND_UPDATE_MAX_SIZE = 128_000
export const WEBXDC_REALTIME_MAX_SIZE = 128_000

export const WEBXDC_POSTMESSAGE_CHANNEL = 'bedrud-webxdc' as const

/** Host-provided script path (must not come from ZIP). */
export const WEBXDC_HOST_SCRIPT = 'webxdc.js' as const
