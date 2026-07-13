/**
 * Helpers for per-app selfAddr (spec: unique per user per app instance,
 * not linkable across apps; do not show in UI).
 *
 * Production should use server-side HMAC. These pure helpers define the
 * contract for tests and client-side placeholders.
 */

/** Derive a stable opaque id from parts (not crypto-grade; tests + client cache key). */
export function deriveWebxdcSelfAddrKey(parts: {
  roomId: string
  appId: string
  userId: string
}): string {
  // Deliberately include appId so two apps for same user differ.
  return `wx:${parts.roomId}:${parts.appId}:${parts.userId}`
}

/** Two instances must not share selfAddr for the same user. */
export function selfAddrsAreUnlinkableAcrossApps(
  a: { roomId: string; appId: string; userId: string },
  b: { roomId: string; appId: string; userId: string },
): boolean {
  if (a.userId !== b.userId) return true
  if (a.appId === b.appId && a.roomId === b.roomId) return false
  return deriveWebxdcSelfAddrKey(a) !== deriveWebxdcSelfAddrKey(b)
}
