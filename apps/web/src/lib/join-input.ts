/** The canonical share path segment, `{server}/m/{room}`. */
const MEETING_PATH_SEGMENT = 'm'

/** The alternate call path segment, `{server}/c/{room}`, kept for links shared before the rename. */
const CALL_PATH_SEGMENT = 'c'

const ROOM_PATH_PATTERN = new RegExp(`(?:^|/)(?:${MEETING_PATH_SEGMENT}|${CALL_PATH_SEGMENT})/([^/?#]+)`)

/** Reports whether the input looks like a URL or a path rather than a room name typed by hand. */
function looksLikeLocation(trimmedInput: string): boolean {
  return trimmedInput.includes('/')
}

/** Turns a name a person typed into the lowercase hyphenated form room names take. */
function slugify(trimmedInput: string): string {
  return trimmedInput.toLowerCase().replace(/\s+/g, '-')
}

/**
 * Resolves whatever was typed or pasted into the quick-join field to a room name.
 *
 * Accepts a full meeting URL on any host, with or without a scheme or a port, a bare `/m/` or `/c/`
 * path, or a plain room name. Returns null when the input names no room, so the caller can say so
 * rather than navigating to a room that cannot exist — which is what slugifying a whole URL did.
 */
export function parseJoinInput(rawInput: string): string | null {
  const trimmedInput = rawInput.trim()
  if (!trimmedInput) return null

  if (looksLikeLocation(trimmedInput)) {
    const roomSegment = ROOM_PATH_PATTERN.exec(trimmedInput)?.[1]
    return roomSegment ? decodeURIComponent(roomSegment) : null
  }

  return slugify(trimmedInput)
}
