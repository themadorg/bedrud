import { WEBXDC_SEND_UPDATE_INTERVAL_MS } from './webxdcConstants'

/**
 * Enforces sendUpdateInterval: returns true if a send is allowed now and
 * records the timestamp. Faster calls return false (host SHOULD queue, not drop).
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

  /** Ms until tryTake would succeed (0 if ready now). */
  msUntilReady(key: string, nowMs: number = Date.now()): number {
    const last = this.lastByKey.get(key)
    if (last === undefined) return 0
    const wait = this.intervalMs - (nowMs - last)
    return wait > 0 ? wait : 0
  }

  reset(key?: string): void {
    if (key === undefined) this.lastByKey.clear()
    else this.lastByKey.delete(key)
  }
}
