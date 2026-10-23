import { API_URL } from '#/lib/api'

/** Returns the origin the account's tokens belong to: the API host, or this page's when they share one. */
function serverOrigin(): string {
  return new URL(API_URL || window.location.origin, window.location.origin).origin
}

/**
 * Reports whether a chat image lives on the account's own server.
 *
 * A chat image's address is whatever the sender wrote, so only an image whose scheme, host and
 * port all match the server may be fetched with the viewer's token and cookies.
 */
export function isOnServer(url: string): boolean {
  try {
    return new URL(url, window.location.origin).origin === serverOrigin()
  } catch {
    return false
  }
}
