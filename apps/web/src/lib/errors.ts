import { ApiError } from '#/lib/api'

function parseErrorPayload(payload: string): string | null {
  const trimmed = payload.trim()
  if (!trimmed) return null

  try {
    const parsed = JSON.parse(trimmed) as unknown

    if (typeof parsed === 'string') {
      return parsed
    }

    if (parsed && typeof parsed === 'object') {
      const record = parsed as Record<string, unknown>
      const message = record.message ?? record.error ?? record.detail

      if (typeof message === 'string' && message.trim()) {
        return message.trim()
      }
    }
  } catch {
    return trimmed
  }

  return trimmed
}

export function getErrorMessage(error: unknown, fallback: string): string {
  const raw = error instanceof Error ? error.message : typeof error === 'string' ? error : ''

  const normalized = raw.replace(/^\d{3}:\s*/s, '').trim()

  return parseErrorPayload(normalized) ?? fallback
}

export function formatJoinRoomError(error: unknown): string {
  if (error instanceof ApiError) {
    const msg = error.message.toLowerCase()
    switch (error.status) {
      case 404:
        return "This room doesn't exist"
      case 410:
        return 'This room is no longer active'
      case 403:
        if (msg.includes('full')) return 'This room is full'
        if (msg.includes('private')) return 'This room is private'
        if (msg.includes('banned')) return 'You are banned from this room'
        if (msg.includes('approval')) return 'This room requires approval to join'
        return error.message || "You can't join this room"
      default:
        return getErrorMessage(error, 'Failed to join room')
    }
  }
  return getErrorMessage(error, 'Failed to join room')
}
