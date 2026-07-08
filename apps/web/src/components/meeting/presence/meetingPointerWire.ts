import {
  type ContentInsets,
  clientToContentNorm,
  contentNormToClient,
  readElementContentInsets,
  type ViewportTransform,
  ZERO_CONTENT_INSETS,
} from '@/components/meeting/meetingViewportTransform'

export { contentNormToClient, readElementContentInsets }

export const MEETING_POINTER_TOPIC = 'meeting-pointer'

export type MeetingPointerPacket = {
  identity: string
  username: string
  /** Normalized X within the shared meeting grid content (0–1). */
  x: number
  /** Normalized Y within the shared meeting grid content (0–1). */
  y: number
  visible: boolean
}

export function clientToSurfaceNorm(
  clientX: number,
  clientY: number,
  rect: DOMRect,
  transform: ViewportTransform = { panX: 0, panY: 0, zoom: 1 },
  insets: ContentInsets = ZERO_CONTENT_INSETS,
): { x: number; y: number } | null {
  return clientToContentNorm(clientX, clientY, rect, transform, insets)
}

export function getMeetGridSurface(): { rect: DOMRect; insets: ContentInsets } | null {
  const grid = document.getElementById('meet-grid')
  if (!grid) return null
  return {
    rect: grid.getBoundingClientRect(),
    insets: readElementContentInsets(grid),
  }
}

export function encodeMeetingPointerPacket(packet: MeetingPointerPacket): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(packet))
}

export function decodeMeetingPointerPacket(payload: Uint8Array): MeetingPointerPacket | null {
  try {
    const parsed = JSON.parse(new TextDecoder().decode(payload)) as MeetingPointerPacket
    if (
      typeof parsed.identity !== 'string' ||
      typeof parsed.username !== 'string' ||
      typeof parsed.x !== 'number' ||
      typeof parsed.y !== 'number' ||
      typeof parsed.visible !== 'boolean' ||
      !Number.isFinite(parsed.x) ||
      !Number.isFinite(parsed.y)
    ) {
      return null
    }
    return {
      identity: parsed.identity,
      username: parsed.username,
      x: Math.min(1, Math.max(0, parsed.x)),
      y: Math.min(1, Math.max(0, parsed.y)),
      visible: parsed.visible,
    }
  } catch {
    return null
  }
}
