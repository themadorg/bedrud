import { WEBXDC_SEND_UPDATE_INTERVAL_MS } from './webxdcConstants'

/**
 * Enforces sendUpdateInterval: returns true if a send is allowed now and
 * records the timestamp. Faster calls return false (host may queue/coalesce).
 */
export class WebxdcSendUpdateRateLimiter {
  private lastByKey = new Map<string, number>()

  constructor(private readonly intervalMs: number = WEBXDC_SEND_UPDATE_INTERVAL_MS) {}

  tryTake(key: string, nowMs: number = Date.now()): boolean {
    const last = this.lastByKey.get(key)
    if (last !== undefined && nowMs - last < this.intervalMs) {
      return false
    }
    this.lastByKey.set(key, nowMs)
    return true
  }

  reset(key?: string): void {
    if (key === undefined) this.lastByKey.clear()
    else this.lastByKey.delete(key)
  }
}
