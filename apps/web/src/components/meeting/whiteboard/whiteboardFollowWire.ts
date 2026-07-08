/** Visible viewport in scene coords: [minX, minY, maxX, maxY] */
export type SceneBounds = readonly [number, number, number, number]

export const WHITEBOARD_FOLLOW_TOPIC = 'whiteboard-follow'

export type WhiteboardFollowChangePacket = {
  type: 'follow-change'
  followerId: string
  followerName?: string
  targetId: string
  action: 'FOLLOW' | 'UNFOLLOW'
}

export type WhiteboardViewportPacket = {
  type: 'viewport'
  identity: string
  sceneBounds: SceneBounds
}

export type WhiteboardFollowPacket = WhiteboardFollowChangePacket | WhiteboardViewportPacket

function isSceneBounds(value: unknown): value is SceneBounds {
  return (
    Array.isArray(value) &&
    value.length === 4 &&
    typeof value[0] === 'number' &&
    typeof value[1] === 'number' &&
    typeof value[2] === 'number' &&
    typeof value[3] === 'number'
  )
}

export function encodeWhiteboardFollowPacket(packet: WhiteboardFollowPacket): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(packet))
}

export function decodeWhiteboardFollowPacket(payload: Uint8Array): WhiteboardFollowPacket | null {
  try {
    const parsed = JSON.parse(new TextDecoder().decode(payload)) as WhiteboardFollowPacket
    if (parsed.type === 'follow-change') {
      if (
        typeof parsed.followerId !== 'string' ||
        typeof parsed.targetId !== 'string' ||
        (parsed.action !== 'FOLLOW' && parsed.action !== 'UNFOLLOW')
      ) {
        return null
      }
      return parsed
    }
    if (parsed.type === 'viewport') {
      if (typeof parsed.identity !== 'string' || !isSceneBounds(parsed.sceneBounds)) return null
      return parsed
    }
    return null
  } catch {
    return null
  }
}