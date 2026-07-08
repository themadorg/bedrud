export const STAGE_DATA_TOPIC = 'stage'

export type StageKind = 'youtube' | 'whiteboard' | 'screenshare'

export type MeetingStage =
  | {
      kind: 'youtube'
      ownerIdentity: string
      ownerName: string
      videoId: string
      playing: boolean
      currentTime: number
      updatedAt: number
    }
  | {
      kind: 'whiteboard'
      ownerIdentity: string
      ownerName: string
      updatedAt: number
    }
  | {
      kind: 'screenshare'
      ownerIdentity: string
      ownerName: string
      updatedAt: number
    }

export type StageWire =
  | { type: 'stage_set'; stage: MeetingStage }
  | { type: 'stage_clear'; ownerIdentity: string; ts: number }
  | { type: 'stage_request'; ts: number }
  | { type: 'stage_state'; stage: MeetingStage | null; ts: number }
  | {
      type: 'stage_youtube_sync'
      ownerIdentity: string
      playing: boolean
      currentTime: number
      ts: number
    }

export function encodeStageWire(payload: StageWire): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(payload))
}

export function parseStageWire(raw: unknown): StageWire | null {
  if (!raw || typeof raw !== 'object') return null
  const msg = raw as Record<string, unknown>
  if (typeof msg.type !== 'string') return null

  switch (msg.type) {
    case 'stage_set': {
      const stage = parseMeetingStage(msg.stage)
      if (!stage) return null
      return { type: 'stage_set', stage }
    }
    case 'stage_clear':
      if (typeof msg.ownerIdentity !== 'string' || typeof msg.ts !== 'number') return null
      return msg as StageWire
    case 'stage_request':
      if (typeof msg.ts !== 'number') return null
      return msg as StageWire
    case 'stage_state':
      if (typeof msg.ts !== 'number') return null
      if (msg.stage !== null && !parseMeetingStage(msg.stage)) return null
      return msg as StageWire
    case 'stage_youtube_sync':
      if (typeof msg.ownerIdentity !== 'string' || typeof msg.playing !== 'boolean') return null
      if (typeof msg.currentTime !== 'number' || typeof msg.ts !== 'number') return null
      return msg as StageWire
    default:
      return null
  }
}

export function parseMeetingStage(raw: unknown): MeetingStage | null {
  if (!raw || typeof raw !== 'object') return null
  const stage = raw as Record<string, unknown>
  if (typeof stage.kind !== 'string' || typeof stage.ownerIdentity !== 'string') return null
  if (typeof stage.ownerName !== 'string' || typeof stage.updatedAt !== 'number') return null

  switch (stage.kind) {
    case 'youtube':
      if (typeof stage.videoId !== 'string' || typeof stage.playing !== 'boolean') return null
      if (typeof stage.currentTime !== 'number') return null
      return stage as MeetingStage
    case 'whiteboard':
    case 'screenshare':
      return stage as MeetingStage
    default:
      return null
  }
}

export function stageOwnerLabel(stage: MeetingStage): string {
  return stage.ownerName || stage.ownerIdentity
}

export function stageSessionKey(stage: MeetingStage): string {
  if (stage.kind === 'youtube') {
    return `youtube:${stage.ownerIdentity}:${stage.videoId}:${stage.updatedAt}`
  }
  return `${stage.kind}:${stage.ownerIdentity}:${stage.updatedAt}`
}

export function stageDescription(stage: MeetingStage): string {
  const who = stageOwnerLabel(stage)
  switch (stage.kind) {
    case 'youtube':
      return `${who} is sharing a YouTube video on stage`
    case 'whiteboard':
      return `${who} opened the shared whiteboard`
    case 'screenshare':
      return `${who} is presenting their screen on stage`
  }
}
