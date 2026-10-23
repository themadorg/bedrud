/** Where a chat link leads: an ordinary page, the room the reader is already in, or another room on this server. */
export type ChatLinkTarget = { kind: 'page' } | { kind: 'currentRoom' } | { kind: 'room'; roomName: string }

const ROOM_PATH = /^\/m\/([^/]+)\/?$/

/**
 * Resolves a chat link against this server. Room names are only unique per server, so a room link
 * counts only when its origin is this one; anything else opens as a page.
 */
export function resolveChatLink(href: string, origin: string, currentRoomName: string): ChatLinkTarget {
  let url: URL
  try {
    url = new URL(href, origin)
  } catch {
    return { kind: 'page' }
  }
  if (url.origin !== new URL(origin).origin) return { kind: 'page' }
  const match = ROOM_PATH.exec(url.pathname)
  if (!match) return { kind: 'page' }
  const roomName = decodeURIComponent(match[1]).toLowerCase()
  if (roomName === currentRoomName.toLowerCase()) return { kind: 'currentRoom' }
  return { kind: 'room', roomName }
}
