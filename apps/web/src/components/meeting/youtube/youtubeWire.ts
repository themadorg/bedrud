export const YOUTUBE_DATA_TOPIC = 'youtube'

export interface YoutubeSession {
  videoId: string
  hostIdentity: string
  hostName: string
  playing: boolean
  currentTime: number
  updatedAt: number
}

export type YoutubeWire =
  | {
      type: 'youtube_load'
      videoId: string
      hostIdentity: string
      hostName: string
      playing: boolean
      currentTime: number
      ts: number
    }
  | {
      type: 'youtube_sync'
      hostIdentity: string
      playing: boolean
      currentTime: number
      ts: number
    }
  | { type: 'youtube_stop'; hostIdentity: string; ts: number }
  | { type: 'youtube_state_request'; ts: number }
  | {
      type: 'youtube_state'
      videoId: string
      hostIdentity: string
      hostName: string
      playing: boolean
      currentTime: number
      ts: number
    }

export function encodeYoutubeWire(payload: YoutubeWire): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(payload))
}

export function parseYoutubeWire(raw: unknown): YoutubeWire | null {
  if (!raw || typeof raw !== 'object') return null
  const msg = raw as Record<string, unknown>
  if (typeof msg.type !== 'string') return null

  switch (msg.type) {
    case 'youtube_load':
    case 'youtube_state':
      if (typeof msg.videoId !== 'string' || typeof msg.hostIdentity !== 'string') return null
      if (typeof msg.hostName !== 'string' || typeof msg.playing !== 'boolean') return null
      if (typeof msg.currentTime !== 'number' || typeof msg.ts !== 'number') return null
      return msg as YoutubeWire
    case 'youtube_sync':
      if (typeof msg.hostIdentity !== 'string' || typeof msg.playing !== 'boolean') return null
      if (typeof msg.currentTime !== 'number' || typeof msg.ts !== 'number') return null
      return msg as YoutubeWire
    case 'youtube_stop':
      if (typeof msg.hostIdentity !== 'string' || typeof msg.ts !== 'number') return null
      return msg as YoutubeWire
    case 'youtube_state_request':
      if (typeof msg.ts !== 'number') return null
      return msg as YoutubeWire
    default:
      return null
  }
}

export function sessionFromWire(
  wire: Extract<YoutubeWire, { type: 'youtube_load' | 'youtube_state' }>,
): YoutubeSession {
  return {
    videoId: wire.videoId,
    hostIdentity: wire.hostIdentity,
    hostName: wire.hostName,
    playing: wire.playing,
    currentTime: wire.currentTime,
    updatedAt: wire.ts,
  }
}
