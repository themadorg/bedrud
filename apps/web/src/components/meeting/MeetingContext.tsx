// TODO oncoming feature
import { useRoomContext } from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { getPublicSettings } from '#/lib/use-public-settings'
import { useUserStore } from '#/lib/user.store'
import { useChatPersistence } from './chat/useChatPersistence'

/** Generate a unique ID with fallback for non-secure contexts (HTTP). */
function generateID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 10)
}

export type SystemEventName =
  | 'kick'
  | 'ban'
  | 'ask_unmute'
  | 'ask_camera'
  | 'spotlight'
  | 'deafen'
  | 'undeafen'
  | 'room_deleted'
  | 'room_ended'
  | 'room_closed'

export interface SystemMessage {
  type: 'system'
  event: SystemEventName
  actor?: string
  target?: string
  message?: string
  deletedIdentity?: string
  ts: number
}

export type RoomDeletionEvent = Extract<SystemEventName, 'room_deleted' | 'room_ended' | 'room_closed'>

export interface ChatAttachment {
  kind: 'image'
  url: string
  mime: string
  w: number
  h: number
  size: number
}

export type ChatMessageStatus = 'sending' | 'sent' | 'failed'

export interface ChatMessage {
  id: string
  timestamp: number
  senderName: string
  senderIdentity: string
  message: string
  attachments: ChatAttachment[]
  isLocal: boolean
  status?: ChatMessageStatus
}

const KNOWN_SYSTEM_EVENTS = new Set([
  'kick',
  'ban',
  'ask_unmute',
  'ask_camera',
  'spotlight',
  'deafen',
  'undeafen',
  'room_deleted',
  'room_ended',
  'room_closed',
])

const ROOM_DELETION_EVENTS: Set<string> = new Set(['room_deleted', 'room_ended', 'room_closed'])

// ── Room context (static / slow-changing metadata) ──────────────────────────

interface MeetingRoomContextValue {
  roomId: string
  roomName: string
  adminId: string
  currentUserId: string
  isCreator: boolean
  isAdmin: boolean
  isModerator: boolean
  // Server-deafened: admin/mod sent a deafen system message targeting this user
  isServerDeafened: boolean
  // Self-deafened: user toggled deafen from controls bar
  isSelfDeafened: boolean
  toggleSelfDeafen: () => void
  // Recording state
  // TODO oncoming feature
  isRecording: boolean
  // TODO oncoming feature
  isRecordingStarting: boolean
  // TODO oncoming feature
  isRecordingStopping: boolean
  // TODO oncoming feature
  toggleRecording: () => void
  // TODO oncoming feature
  recordingsAllowed: boolean
  // TODO oncoming feature
  recordingsEnabled: boolean
}

const MeetingRoomContext = createContext<MeetingRoomContextValue | null>(null)

export function useMeetingRoomContext(): MeetingRoomContextValue {
  const ctx = useContext(MeetingRoomContext)
  if (!ctx) throw new Error('useMeetingRoomContext must be used inside MeetingProvider')
  return ctx
}

// ── Chat context (fast-changing chat state) ─────────────────────────────────

interface MeetingChatContextValue {
  chatMessages: ChatMessage[]
  systemMessages: SystemMessage[]
  sendChat: (text: string, attachments?: ChatAttachment[]) => void
  unreadCount: number
  markRead: () => void
}

const MeetingChatContext = createContext<MeetingChatContextValue | null>(null)

export function useMeetingChatContext(): MeetingChatContextValue {
  const ctx = useContext(MeetingChatContext)
  if (!ctx) throw new Error('useMeetingChatContext must be used inside MeetingProvider')
  return ctx
}

// ── Legacy combined hook (for backward compatibility) ───────────────────────

interface MeetingContextValue extends MeetingRoomContextValue, MeetingChatContextValue {}

export function useMeetingContext(): MeetingContextValue {
  const room = useMeetingRoomContext()
  const chat = useMeetingChatContext()
  return useMemo(() => ({ ...room, ...chat }), [room, chat])
}

// ── Chat memory & persistence limits ─────────────────────────────────────

const MEMORY_CHAT_CAP = 400
const MEMORY_SYSTEM_CAP = 100
const PERSIST_CHAT_CAP = 200

function capMessages<T>(arr: T[], max: number): T[] {
  if (arr.length <= max) return arr
  return arr.slice(arr.length - max)
}

// ── Chat retention helpers ──────────────────────────────────────────────────

function applyChatRetention(messages: ChatMessage[], ttlHours: number): ChatMessage[] {
  if (ttlHours > 0) {
    const cutoff = Date.now() - ttlHours * 60 * 60 * 1000
    return messages.filter((m) => m.timestamp >= cutoff)
  }
  return messages
}

// ── Provider ────────────────────────────────────────────────────────────────

interface MeetingProviderProps {
  roomId: string
  roomName: string
  adminId: string
  recordingsAllowed?: boolean
  activeRecordingId?: string
  onRoomDeletionMessage?: (event: RoomDeletionEvent, message: string, isCurrentUserDeleted: boolean) => void
  children: ReactNode
}

export function MeetingProvider({
  roomId,
  roomName,
  adminId,
  recordingsAllowed = true,
  activeRecordingId,
  onRoomDeletionMessage,
  children,
}: MeetingProviderProps) {
  const user = useUserStore((s) => s.user)
  const currentUserId = user?.id ?? ''
  const accesses = user?.accesses ?? []
  const room = useRoomContext()

  const [ttlHours, setTtlHours] = useState(2160)

  const [recordingsEnabled, setRecordingsEnabled] = useState(true)

  useEffect(() => {
    getPublicSettings()
      .then((s) => {
        setTtlHours(s.chatMessageTTLHours ?? 2160)
        setRecordingsEnabled(false) // TODO oncoming feature
      })
      .catch(() => {})
  }, [])

  const [initialMessages, persistMessages] = useChatPersistence(roomId, PERSIST_CHAT_CAP, ttlHours)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>(initialMessages)
  useEffect(() => {
    persistMessages(chatMessages.map(({ status: _, ...m }) => m as ChatMessage))
  }, [chatMessages, persistMessages])
  const [systemMessages, setSystemMessages] = useState<SystemMessage[]>([])
  const [isServerDeafened, setIsServerDeafened] = useState(false)
  const [isSelfDeafened, setIsSelfDeafened] = useState(false)
  const [isRecording, _setIsRecording] = useState(!!activeRecordingId)
  const [isRecordingStarting, _setIsRecordingStarting] = useState(false)
  const [isRecordingStopping, _setIsRecordingStopping] = useState(false)
  const micBeforeDeafenRef = useRef(true)
  const toggleSelfDeafen = useCallback(() => {
    if (isSelfDeafened) {
      setIsSelfDeafened(false)
      // Restore mic only if it was on before we deafened
      if (micBeforeDeafenRef.current) room.localParticipant.setMicrophoneEnabled(true)
      return
    }
    micBeforeDeafenRef.current = room.localParticipant.isMicrophoneEnabled
    room.localParticipant.setMicrophoneEnabled(false)
    setIsSelfDeafened(true)
  }, [isSelfDeafened, room.localParticipant])

  const toggleRecording = useCallback(async () => {
    // TODO oncoming feature
  }, [])

  const [unreadCount, setUnreadCount] = useState(0)

  // Track how many messages existed at the last markRead() so we only count new arrivals
  const chatSeenRef = useRef(0)
  const systemSeenRef = useRef(0)
  const onRoomDeletionMessageRef = useRef(onRoomDeletionMessage)
  onRoomDeletionMessageRef.current = onRoomDeletionMessage

  useEffect(() => {
    const handler = (payload: Uint8Array, participant: unknown, _kind: unknown, topic?: string) => {
      try {
        const raw = JSON.parse(new TextDecoder().decode(payload))

        if (topic === 'system') {
          if (raw.type === 'system' && typeof raw.event === 'string' && KNOWN_SYSTEM_EVENTS.has(raw.event)) {
            if (ROOM_DELETION_EVENTS.has(raw.event)) {
              const msg = { ...(raw as SystemMessage), ts: Date.now() }
              // If virtual scroll: index-based keys (sys-${i}) need stable IDs; scroll anchoring required when cap evicts old system events.
              setSystemMessages((prev) => capMessages([...prev, msg], MEMORY_SYSTEM_CAP))
              const isCurrentUserDeleted = raw.deletedIdentity === room.localParticipant.identity
              onRoomDeletionMessageRef.current?.(
                raw.event as RoomDeletionEvent,
                raw.message ?? '',
                isCurrentUserDeleted,
              )
              return
            }
            if (
              typeof raw.actor === 'string' &&
              raw.actor.length > 0 &&
              typeof raw.target === 'string' &&
              raw.target.length > 0
            ) {
              const msg = { ...(raw as SystemMessage), ts: Date.now() }
              // If virtual scroll: index-based keys (sys-${i}) need stable IDs; scroll anchoring required when cap evicts old system events.
              setSystemMessages((prev) => capMessages([...prev, msg], MEMORY_SYSTEM_CAP))
              if (msg.target === currentUserId) {
                if (msg.event === 'deafen') setIsServerDeafened(true)
                else if (msg.event === 'undeafen') setIsServerDeafened(false)
              }
            }
          }
          return
        }

        if (topic === 'chat' && raw.type === 'chat') {
          // Resolve sender identity from the participant object (RemoteParticipant)
          const p = participant as { identity?: string; name?: string } | null
          const senderIdentity = (raw.senderIdentity as string) || p?.identity || ''
          const senderName = (raw.senderName as string) || p?.name || p?.identity || 'Unknown'

          const msg: ChatMessage = {
            id: (raw.id as string) || generateID(),
            timestamp: (raw.timestamp as number) || Date.now(),
            senderName,
            senderIdentity,
            message: (raw.message as string) || '',
            attachments: Array.isArray(raw.attachments) ? (raw.attachments as ChatAttachment[]) : [],
            isLocal: false,
          }
          // If virtual scroll: index keys (cluster-${i}) need stable IDs; groupMessages output shifts on cap — virtualizer needs scroll anchoring.
          setChatMessages((prev) => {
            if (prev.some((m) => m.id === msg.id)) return prev
            const updated = [...prev, msg]
            return capMessages(applyChatRetention(updated, ttlHours), MEMORY_CHAT_CAP)
          })
        }
      } catch {
        // Silently discard malformed data messages — a malicious participant
        // could flood the channel with garbage, so we avoid polluting the console.
      }
    }
    room.on(RoomEvent.DataReceived, handler)
    return () => {
      room.off(RoomEvent.DataReceived, handler)
    }
  }, [room, currentUserId, ttlHours])

  // Increment unread counter only for messages that arrive after the last markRead()
  useEffect(() => {
    const chatDelta = chatMessages.length - chatSeenRef.current
    const systemDelta = systemMessages.length - systemSeenRef.current
    chatSeenRef.current = chatMessages.length
    systemSeenRef.current = systemMessages.length
    if (chatDelta > 0 || systemDelta > 0) {
      setUnreadCount((n) => n + chatDelta + systemDelta)
    }
  }, [chatMessages.length, systemMessages.length])

  const markRead = useCallback(() => {
    chatSeenRef.current = chatMessages.length
    systemSeenRef.current = systemMessages.length
    setUnreadCount(0)
  }, [chatMessages.length, systemMessages.length])

  // sendChat publishes a reliable data packet on the "chat" topic.
  // The message is also echoed locally immediately for zero-latency feedback.
  const sendChat = useCallback(
    (text: string, attachments?: ChatAttachment[]) => {
      const lp = room.localParticipant
      const id = generateID()
      const timestamp = Date.now()
      const senderName = lp.name || lp.identity || 'You'
      const senderIdentity = lp.identity || ''

      const payload = {
        type: 'chat',
        id,
        timestamp,
        senderName,
        senderIdentity,
        message: text,
        attachments: attachments ?? [],
      }

      const data = new TextEncoder().encode(JSON.stringify(payload))
      lp.publishData(data, { reliable: true, topic: 'chat' })
        .then(() => {
          setChatMessages((prev) => prev.map((m): ChatMessage => (m.id === id ? { ...m, status: 'sent' } : m)))
        })
        .catch((err) => {
          setChatMessages((prev) => prev.map((m): ChatMessage => (m.id === id ? { ...m, status: 'failed' } : m)))
          if (import.meta.env.DEV) console.error('[MeetingContext] failed to publish chat message:', err)
        })

      // Local echo so the sender sees the message immediately
      // If virtual scroll: index keys (cluster-${i}) need stable IDs; groupMessages output shifts on cap — virtualizer needs scroll anchoring.
      setChatMessages((prev) => {
        const updated: ChatMessage[] = [
          ...prev,
          {
            id,
            timestamp,
            senderName,
            senderIdentity,
            message: text,
            attachments: attachments ?? [],
            isLocal: true,
            status: 'sending',
          },
        ]
        return capMessages(applyChatRetention(updated, ttlHours), MEMORY_CHAT_CAP)
      })
    },
    [room, ttlHours],
  )

  // Room context value — stable unless room metadata actually changes
  const roomValue = useMemo<MeetingRoomContextValue>(
    () => ({
      roomId,
      roomName,
      adminId,
      currentUserId,
      isCreator: !!adminId && (currentUserId === adminId || room.localParticipant.identity === adminId),
      isAdmin: accesses.includes('admin') || accesses.includes('superadmin'),
      isModerator:
        accesses.includes('moderator') ||
        accesses.includes('admin') ||
        accesses.includes('superadmin') ||
        (!!adminId && currentUserId === adminId),
      isServerDeafened,
      isSelfDeafened,
      toggleSelfDeafen,
      isRecording,
      isRecordingStarting,
      isRecordingStopping,
      toggleRecording,
      recordingsAllowed,
      recordingsEnabled,
    }),
    [
      roomId,
      roomName,
      adminId,
      currentUserId,
      accesses,
      isServerDeafened,
      isSelfDeafened,
      toggleSelfDeafen,
      isRecording,
      isRecordingStarting,
      isRecordingStopping,
      toggleRecording,
      recordingsAllowed,
      recordingsEnabled,
      room.localParticipant.identity,
    ],
  )

  // Chat context value — changes every time a message arrives (isolated from room context)
  const chatValue = useMemo<MeetingChatContextValue>(
    () => ({
      chatMessages,
      systemMessages,
      sendChat,
      unreadCount,
      markRead,
    }),
    [chatMessages, systemMessages, sendChat, unreadCount, markRead],
  )

  return (
    <MeetingRoomContext.Provider value={roomValue}>
      <MeetingChatContext.Provider value={chatValue}>{children}</MeetingChatContext.Provider>
    </MeetingRoomContext.Provider>
  )
}
