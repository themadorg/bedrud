export const WHITEBOARD_POINTER_TOPIC = 'whiteboard-pointer'

export type WhiteboardPointerTool = 'pointer' | 'laser'

export type WhiteboardPointerPacket = {
  identity: string
  username: string
  color?: string
  pointer: {
    x: number
    y: number
    tool: WhiteboardPointerTool
    renderCursor?: boolean
  }
  button: 'up' | 'down'
  selectedElementIds?: string[]
}

export function encodeWhiteboardPointerPacket(packet: WhiteboardPointerPacket): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(packet))
}

export function decodeWhiteboardPointerPacket(payload: Uint8Array): WhiteboardPointerPacket | null {
  try {
    const parsed = JSON.parse(new TextDecoder().decode(payload)) as WhiteboardPointerPacket
    const tool = parsed.pointer?.tool
    if (
      typeof parsed.identity !== 'string' ||
      typeof parsed.username !== 'string' ||
      typeof parsed.pointer?.x !== 'number' ||
      typeof parsed.pointer?.y !== 'number' ||
      (tool !== 'laser' && tool !== 'pointer') ||
      (parsed.button !== 'up' && parsed.button !== 'down')
    ) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}