/**
 * Resolve update.href as a relative URL only (Delta Chat Desktop pattern).
 * Absolute http(s) and other schemes are rejected.
 */
export function resolveWebxdcHref(href: string | undefined | null): string | null {
  if (href == null) return null
  const raw = String(href).trim()
  if (!raw) return null

  // Explicit absolute schemes
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(raw)) {
    return null
  }
  // Protocol-relative
  if (raw.startsWith('//')) {
    return null
  }

  try {
    const url = new URL(raw, 'http://webxdc.invalid/')
    // Only allow path/query/hash under dummy host
    if (url.hostname !== 'webxdc.invalid') {
      return null
    }
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

export function isSafeRelativeWebxdcHref(href: string | undefined | null): boolean {
  return resolveWebxdcHref(href) !== null
}
