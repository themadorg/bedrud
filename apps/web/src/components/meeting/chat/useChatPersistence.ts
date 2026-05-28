import { useCallback, useMemo } from 'react'
import type { ChatMessage } from '../MeetingContext'

export const MAX_INITIAL_LOAD = 400

export function useChatPersistence(
  roomId: string,
  maxMessageCount: number,
  messageTTLHours: number,
): [ChatMessage[], (msgs: ChatMessage[]) => void] {
  const key = `chat:${roomId}`

  const initialMessages = useMemo<ChatMessage[]>(() => {
    try {
      const raw = sessionStorage.getItem(key)
      const msgs: ChatMessage[] = raw ? (JSON.parse(raw) as ChatMessage[]) : []
      const filtered =
        messageTTLHours > 0 ? msgs.filter((m) => m.timestamp >= Date.now() - messageTTLHours * 3600000) : msgs
      return filtered.slice(-MAX_INITIAL_LOAD)
    } catch {
      return []
    }
  }, [key, messageTTLHours])

  const persist = useCallback(
    (msgs: ChatMessage[]) => {
      try {
        let toStore = msgs
        if (messageTTLHours > 0) {
          const cutoff = Date.now() - messageTTLHours * 60 * 60 * 1000
          toStore = toStore.filter((m) => m.timestamp >= cutoff)
        }
        if (maxMessageCount > 0 && toStore.length > maxMessageCount) {
          toStore = toStore.slice(toStore.length - maxMessageCount)
        }
        sessionStorage.setItem(key, JSON.stringify(toStore))
      } catch {
        // sessionStorage unavailable (private browsing quota exceeded)
      }
    },
    [key, maxMessageCount, messageTTLHours],
  )

  return [initialMessages, persist]
}
