// TODO oncoming feature
import { useRoomContext } from '@livekit/components-react'
import { ConnectionState, type Participant, type Room, RoomEvent } from 'livekit-client'
import {
  isPublishUnavailableError,
  isRoomPublishReady,
  isRoomSignalingReady,
  MEETING_CHAT_TOPIC,
  waitForRoomPublishReady,
} from '#/lib/livekit-publish'
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '#/lib/api'
import { getPublicSettings } from '#/lib/use-public-settings'
import { useAuthStore } from '#/lib/auth.store'
import { useProfileSyncStore } from '#/lib/profile-sync.store'
import { decodeBedrudJwt } from '#/lib/jwt-user'
import { type User, useUserStore } from '#/lib/user.store'
import {
  applyChatChunkPart,
  assembledChatFromChunks,
  buildChatWirePackets,
  type ChatChunkMetaWire,
  type ChatChunkWire,
  type ChatWirePayload,
  createChunkBuffer,
  encodeChatWire,
  ingestChatChunk,
  pruneChunkBuffers,
} from './chat/chatDataChannel'
import { applyReactionToggle } from './chat/chatReactions'
import { isMeetingChatDataTopic } from './chat/chatTopic'
import { fetchMeetingParticipantProfile } from './meetingParticipantProfile'
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

/** Normalize upload/API payloads and legacy messages missing `kind`. */
export function normalizeChatAttachment(raw: unknown): ChatAttachment | null {
  if (!raw || typeof raw !== 'object') return null
  const att = raw as Record<string, unknown>
  const url = typeof att.url === 'string' ? att.url : null
  const mime = typeof att.mime === 'string' ? att.mime : null
  if (!url || !mime) return null
  if (att.kind !== 'image' && !mime.startsWith('image/')) return null
  return {
    kind: 'image',
    url,
    mime,
    w: typeof att.w === 'number' ? att.w : 0,
    h: typeof att.h === 'number' ? att.h : 0,
    size: typeof att.size === 'number' ? att.size : 0,
  }
}

function normalizeAttachments(raw: unknown): ChatAttachment[] {
  if (!Array.isArray(raw)) return []
  return raw.map(normalizeChatAttachment).filter((a): a is ChatAttachment => a !== null)
}

export interface ChatPollOption {
  id: string
  text: string
}

export interface ChatPoll {
  id: string
  question: string
  options: ChatPollOption[]
  votes: Record<string, string>
}

export function normalizeChatPoll(raw: unknown): ChatPoll | null {
  if (!raw || typeof raw !== 'object') return null
  const poll = raw as Record<string, unknown>
  const id = typeof poll.id === 'string' ? poll.id : null
  const question = typeof poll.question === 'string' ? poll.question : null
  if (!id || !question) return null

  const options: ChatPollOption[] = []
  if (Array.isArray(poll.options)) {
    for (const opt of poll.options) {
      if (!opt || typeof opt !== 'object') continue
      const row = opt as Record<string, unknown>
      const optId = typeof row.id === 'string' ? row.id : null
      const text = typeof row.text === 'string' ? row.text : null
      if (optId && text) options.push({ id: optId, text })
    }
  }
  if (options.length < 2) return null

  const votes: Record<string, string> = {}
  if (poll.votes && typeof poll.votes === 'object' && !Array.isArray(poll.votes)) {
    for (const [voter, optionId] of Object.entries(poll.votes as Record<string, unknown>)) {
      if (typeof optionId === 'string' && options.some((o) => o.id === optionId)) {
        votes[voter] = optionId
      }
    }
  }

  return { id, question, options, votes }
}

export type ChatMessageStatus = 'sending' | 'sent' | 'failed'

export type ChatReactions = Record<string, string>

export interface ChatMessage {
  id: string
  timestamp: number
  senderName: string
  senderIdentity: string
  message: string
  attachments: ChatAttachment[]
  poll?: ChatPoll
  reactions: ChatReactions
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
  isPublic: boolean
  setRoomIsPublic: (isPublic: boolean) => void
  currentUserId: string
  isCreator: boolean
  canManageRoomAccess: boolean
  isAdmin: boolean
  isModerator: boolean
  // Server-deafened: admin/mod sent a deafen system message targeting this user
  isServerDeafened: boolean
  // Self-deafened: user toggled deafen from controls bar
  isSelfDeafened: boolean
  toggleSelfDeafen: () => void
  isParticipantDeafened: (participant: Participant) => boolean
  getParticipantDisplayName: (participant: Participant) => string
  getParticipantAvatarUrl: (participant: Participant) => string | undefined
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
  sendChat: (text: string, attachments?: ChatAttachment[], poll?: ChatPoll) => void
  votePoll: (messageId: string, optionId: string) => void
  reactToMessage: (messageId: string, emoji: string) => void
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

function readDeafenedFromMetadata(metadata?: string): boolean {
  try {
    return JSON.parse(metadata ?? '{}').deafened === true
  } catch {
    return false
  }
}

function readDisplayNameFromMetadata(metadata?: string): string | undefined {
  try {
    const name = JSON.parse(metadata ?? '{}').displayName
    if (typeof name !== 'string') return undefined
    const trimmed = name.trim()
    return trimmed || undefined
  } catch {
    return undefined
  }
}

function readAvatarFromMetadata(metadata?: string): string | undefined {
  try {
    const parsed = JSON.parse(metadata ?? '{}') as Record<string, unknown>
    if (!('avatarUrl' in parsed)) return undefined
    if (typeof parsed.avatarUrl !== 'string') return undefined
    const trimmed = parsed.avatarUrl.trim()
    return trimmed || undefined
  } catch {
    return undefined
  }
}

function metadataHasAvatarKey(metadata?: string): boolean {
  try {
    return 'avatarUrl' in (JSON.parse(metadata ?? '{}') as Record<string, unknown>)
  } catch {
    return false
  }
}

interface MeResponse {
  id: string
  name: string
  email: string
  provider: string
  accesses: string[] | null
  avatarUrl?: string
}

function mapMeToUser(me: MeResponse): User {
  return {
    id: me.id,
    email: me.email,
    name: me.name,
    provider: me.provider,
    isSuperAdmin: me.accesses?.includes('superadmin') ?? false,
    isAdmin: (me.accesses?.includes('admin') || me.accesses?.includes('superadmin')) ?? false,
    accesses: me.accesses ?? [],
    avatarUrl: me.avatarUrl,
  }
}

function patchLocalParticipantMetadata(
  room: Room,
  participant: { metadata?: string; setMetadata: (data: string) => Promise<void> },
  patch: Record<string, unknown>,
) {
  if (!isRoomSignalingReady(room)) return
  let base: Record<string, unknown> = {}
  try {
    base = JSON.parse(participant.metadata ?? '{}') as Record<string, unknown>
  } catch {
    base = {}
  }
  const next = JSON.stringify({ ...base, ...patch })
  if (participant.metadata === next) return
  void participant.setMetadata(next).catch(() => {})
}

interface MeetingProviderProps {
  roomId: string
  roomName: string
  adminId: string
  createdBy?: string
  isPublic?: boolean
  recordingsAllowed?: boolean
  activeRecordingId?: string
  onRoomDeletionMessage?: (event: RoomDeletionEvent, message: string, isCurrentUserDeleted: boolean) => void
  children: ReactNode
}

export function MeetingProvider({
  roomId,
  roomName,
  adminId,
  createdBy = '',
  isPublic = false,
  recordingsAllowed = true,
  activeRecordingId,
  onRoomDeletionMessage,
  children,
}: MeetingProviderProps) {
  const user = useUserStore((s) => s.user)
  const setUser = useUserStore((s) => s.setUser)
  const accessToken = useAuthStore((s) => s.tokens?.accessToken)
  const profileSyncVersion = useProfileSyncStore((s) => s.version)
  const jwtUser = useMemo(() => decodeBedrudJwt(accessToken), [accessToken])
  const currentUserId = user?.id ?? jwtUser.userId
  const accesses = user?.accesses ?? jwtUser.accesses
  const room = useRoomContext()
  const hostUserId = adminId || createdBy
  const localIdentity = room.localParticipant.identity
  const isGuestParticipant = localIdentity.startsWith('guest-')
  const isCreator =
    !isGuestParticipant &&
    !!hostUserId &&
    (currentUserId === hostUserId || localIdentity === hostUserId)
  const isSuperAdmin = accesses.includes('superadmin')
  const canManageRoomAccess = isCreator || isSuperAdmin

  const [ttlHours, setTtlHours] = useState(2160)
  const [roomIsPublic, setRoomIsPublic] = useState(isPublic)

  const [recordingsEnabled, setRecordingsEnabled] = useState(true)

  useEffect(() => {
    setRoomIsPublic(isPublic)
  }, [isPublic])

  useEffect(() => {
    getPublicSettings()
      .then((s) => {
        setTtlHours(s.chatMessageTTLHours ?? 2160)
        setRecordingsEnabled(false) // TODO oncoming feature
      })
      .catch(() => {})
  }, [])

  const [initialMessages, persistMessages] = useChatPersistence(roomId, PERSIST_CHAT_CAP, ttlHours)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>(() =>
    initialMessages.map((m) => ({ ...m, reactions: m.reactions ?? {} })),
  )
  useEffect(() => {
    persistMessages(chatMessages.map(({ status: _, ...m }) => m as ChatMessage))
  }, [chatMessages, persistMessages])
  const [systemMessages, setSystemMessages] = useState<SystemMessage[]>([])
  const [isServerDeafened, setIsServerDeafened] = useState(false)
  const [isSelfDeafened, setIsSelfDeafened] = useState(false)
  const [peerDeafened, setPeerDeafened] = useState<Record<string, boolean>>({})
  const [peerDisplayNames, setPeerDisplayNames] = useState<Record<string, string>>({})
  const [peerAvatarUrls, setPeerAvatarUrls] = useState<Record<string, string>>({})
  const [isRecording, _setIsRecording] = useState(!!activeRecordingId)
  const [isRecordingStarting, _setIsRecordingStarting] = useState(false)
  const [isRecordingStopping, _setIsRecordingStopping] = useState(false)
  const micBeforeDeafenRef = useRef(true)
  const deafenedRef = useRef(false)
  deafenedRef.current = isSelfDeafened || isServerDeafened
  const userNameRef = useRef(user?.name?.trim() || undefined)
  userNameRef.current = user?.name?.trim() || undefined
  const userAvatarRef = useRef(user?.avatarUrl?.trim() || undefined)
  userAvatarRef.current = user?.avatarUrl?.trim() || undefined

  const applyRemoteProfile = useCallback((identity: string, name: string, avatarUrl?: string) => {
    const trimmedName = name.trim()
    if (!trimmedName) return
    setPeerDisplayNames((prev) => {
      if (prev[identity] === trimmedName) return prev
      return { ...prev, [identity]: trimmedName }
    })
    setPeerAvatarUrls((prev) => {
      const trimmedAvatar = avatarUrl?.trim()
      if (trimmedAvatar) {
        if (prev[identity] === trimmedAvatar) return prev
        return { ...prev, [identity]: trimmedAvatar }
      }
      if (!(identity in prev)) return prev
      const next = { ...prev }
      delete next[identity]
      return next
    })
  }, [])

  const applyLocalProfile = useCallback(
    (name: string, avatarUrl?: string) => {
      const trimmedName = name.trim()
      if (!trimmedName) return
      const trimmedAvatar = avatarUrl?.trim() || undefined
      void room.localParticipant.setName(trimmedName).catch(() => {})
      patchLocalParticipantMetadata(room, room.localParticipant, {
        displayName: trimmedName,
        avatarUrl: trimmedAvatar ?? '',
      })
      applyRemoteProfile(localIdentity, trimmedName, trimmedAvatar)
    },
    [applyRemoteProfile, localIdentity, room],
  )

  const notifyProfileChanged = useCallback(() => {
    const name = userNameRef.current || user?.name?.trim() || room.localParticipant.name?.trim()
    if (!name) return
    const payload = new TextEncoder().encode(
      JSON.stringify({
        type: 'profile_changed',
        identity: localIdentity,
        name,
        avatarUrl: userAvatarRef.current ?? '',
      }),
    )
    void room.localParticipant.publishData(payload, { reliable: true, topic: 'presence' }).catch(() => {})
  }, [localIdentity, room.localParticipant, user?.name])

  const fetchRemoteParticipantProfile = useCallback(
    async (identity: string) => {
      if (!identity) return
      const profile = await fetchMeetingParticipantProfile(roomId, identity)
      if (!profile?.name?.trim()) return
      if (identity === localIdentity) {
        userNameRef.current = profile.name.trim()
        userAvatarRef.current = profile.avatarUrl?.trim() || undefined
        applyLocalProfile(profile.name, profile.avatarUrl)
        return
      }
      applyRemoteProfile(identity, profile.name, profile.avatarUrl)
    },
    [applyLocalProfile, applyRemoteProfile, localIdentity, roomId],
  )

  const fetchRemoteParticipantProfileRef = useRef(fetchRemoteParticipantProfile)
  fetchRemoteParticipantProfileRef.current = fetchRemoteParticipantProfile
  const applyRemoteProfileRef = useRef(applyRemoteProfile)
  applyRemoteProfileRef.current = applyRemoteProfile
  const localIdentityRef = useRef(localIdentity)
  localIdentityRef.current = localIdentity

  const advertiseDeafenedState = useCallback(
    (deafened: boolean) => {
      patchLocalParticipantMetadata(room, room.localParticipant, { deafened })
      const payload = new TextEncoder().encode(
        JSON.stringify({
          type: 'deafen_state',
          identity: room.localParticipant.identity,
          deafened,
        }),
      )
      void room.localParticipant.publishData(payload, { reliable: true, topic: 'presence' }).catch(() => {})
    },
    [room],
  )

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

  useEffect(() => {
    advertiseDeafenedState(isSelfDeafened || isServerDeafened)
  }, [isSelfDeafened, isServerDeafened, advertiseDeafenedState])

  const refreshLocalProfile = useCallback(async () => {
    if (room.state !== ConnectionState.Connected || !isRoomSignalingReady(room)) return

    if (accessToken && !localIdentity.startsWith('guest-')) {
      try {
        const me = await api.get<MeResponse>('/api/auth/me')
        setUser(mapMeToUser(me))
        const serverName = me.name?.trim()
        const serverAvatar = me.avatarUrl?.trim() || undefined
        if (serverName) {
          userNameRef.current = serverName
          userAvatarRef.current = serverAvatar
          applyLocalProfile(serverName, serverAvatar)
          notifyProfileChanged()
        }
        return
      } catch {}
    }

    const fallbackName = userNameRef.current || room.localParticipant.name?.trim()
    if (fallbackName) {
      applyLocalProfile(fallbackName, userAvatarRef.current)
      notifyProfileChanged()
    }
  }, [
    accessToken,
    localIdentity,
    room.state,
    room.localParticipant,
    setUser,
    applyLocalProfile,
    notifyProfileChanged,
  ])

  const fetchAllRemoteProfiles = useCallback(() => {
    for (const participant of room.remoteParticipants.values()) {
      void fetchRemoteParticipantProfile(participant.identity)
    }
  }, [fetchRemoteParticipantProfile, room.remoteParticipants])

  useEffect(() => {
    const trimmedName = user?.name?.trim()
    if (!trimmedName || room.state !== ConnectionState.Connected || !isRoomSignalingReady(room)) return
    userNameRef.current = trimmedName
    applyLocalProfile(trimmedName, userAvatarRef.current)
    notifyProfileChanged()
  }, [user?.name, room.state, applyLocalProfile, notifyProfileChanged])

  useEffect(() => {
    if (room.state !== ConnectionState.Connected || !isRoomSignalingReady(room)) return
    userAvatarRef.current = user?.avatarUrl?.trim() || undefined
    const trimmedName = userNameRef.current || user?.name?.trim() || room.localParticipant.name?.trim()
    if (!trimmedName) return
    applyLocalProfile(trimmedName, userAvatarRef.current)
    notifyProfileChanged()
  }, [
    user?.avatarUrl,
    user?.name,
    room.state,
    room.localParticipant.name,
    profileSyncVersion,
    applyLocalProfile,
    notifyProfileChanged,
  ])

  useEffect(() => {
    const syncParticipant = (participant: Participant) => {
      setPeerDeafened((prev) => {
        const deafened = readDeafenedFromMetadata(participant.metadata)
        if (prev[participant.identity] === deafened) return prev
        return { ...prev, [participant.identity]: deafened }
      })

      const displayName = readDisplayNameFromMetadata(participant.metadata)
      if (displayName) {
        setPeerDisplayNames((prev) => {
          if (prev[participant.identity] === displayName) return prev
          return { ...prev, [participant.identity]: displayName }
        })
      }

      if (metadataHasAvatarKey(participant.metadata)) {
        const avatarUrl = readAvatarFromMetadata(participant.metadata)
        setPeerAvatarUrls((prev) => {
          if (avatarUrl) {
            if (prev[participant.identity] === avatarUrl) return prev
            return { ...prev, [participant.identity]: avatarUrl }
          }
          if (!(participant.identity in prev)) return prev
          const next = { ...prev }
          delete next[participant.identity]
          return next
        })
      }
    }

    for (const participant of room.remoteParticipants.values()) {
      syncParticipant(participant)
    }
    syncParticipant(room.localParticipant)

    const onMetadataChanged = (_metadata: string | undefined, participant: Participant) => {
      syncParticipant(participant)
    }
    const onParticipantConnected = (participant: Participant) => {
      if (deafenedRef.current) advertiseDeafenedState(true)
      void fetchRemoteParticipantProfile(participant.identity)
      void refreshLocalProfile()
    }
    const onConnected = () => {
      if (deafenedRef.current) advertiseDeafenedState(true)
      void refreshLocalProfile()
      fetchAllRemoteProfiles()
    }

    room.on(RoomEvent.ParticipantMetadataChanged, onMetadataChanged)
    room.on(RoomEvent.ParticipantConnected, onParticipantConnected)
    room.on(RoomEvent.Connected, onConnected)
    if (room.state === ConnectionState.Connected) {
      void refreshLocalProfile()
      fetchAllRemoteProfiles()
    }
    return () => {
      room.off(RoomEvent.ParticipantMetadataChanged, onMetadataChanged)
      room.off(RoomEvent.ParticipantConnected, onParticipantConnected)
      room.off(RoomEvent.Connected, onConnected)
    }
  }, [
    room,
    advertiseDeafenedState,
    fetchRemoteParticipantProfile,
    refreshLocalProfile,
    fetchAllRemoteProfiles,
  ])

  const isParticipantDeafened = useCallback(
    (participant: Participant) => {
      if (participant.isLocal) return isSelfDeafened || isServerDeafened
      if (peerDeafened[participant.identity]) return true
      return readDeafenedFromMetadata(participant.metadata)
    },
    [isSelfDeafened, isServerDeafened, peerDeafened],
  )

  const getParticipantDisplayName = useCallback(
    (participant: Participant) => {
      const fromPeer = peerDisplayNames[participant.identity]
      if (fromPeer) return fromPeer
      const fromMeta = readDisplayNameFromMetadata(participant.metadata)
      if (fromMeta) return fromMeta
      const liveName = participant.name?.trim()
      if (liveName && liveName !== participant.identity) return liveName
      return liveName || participant.identity
    },
    [peerDisplayNames],
  )

  const getParticipantAvatarUrl = useCallback(
    (participant: Participant) => {
      const fromPeers = peerAvatarUrls[participant.identity]
      if (fromPeers) return fromPeers
      const fromMeta = readAvatarFromMetadata(participant.metadata)
      if (fromMeta) return fromMeta
      if (participant.isLocal) return user?.avatarUrl?.trim() || undefined
      return undefined
    },
    [peerAvatarUrls, user?.avatarUrl],
  )

  const toggleRecording = useCallback(async () => {
    // TODO oncoming feature
  }, [])

  const [unreadCount, setUnreadCount] = useState(0)

  // Track how many messages existed at the last markRead() so we only count new arrivals
  const chatSeenRef = useRef(0)
  const systemSeenRef = useRef(0)

  // Refs to always read latest lengths inside stable callbacks (avoid recreating markRead on every length change)
  const chatMessagesRef = useRef(chatMessages)
  const systemMessagesRef = useRef(systemMessages)
  useEffect(() => {
    chatMessagesRef.current = chatMessages
  }, [chatMessages])
  useEffect(() => {
    systemMessagesRef.current = systemMessages
  }, [systemMessages])

  const onRoomDeletionMessageRef = useRef(onRoomDeletionMessage)
  onRoomDeletionMessageRef.current = onRoomDeletionMessage

  const chatChunkBuffersRef = useRef(createChunkBuffer())
  const chatPublishInFlightRef = useRef(false)

  const publishChatPackets = useCallback(
    async (id: string, packets: ReturnType<typeof buildChatWirePackets>): Promise<boolean> => {
      const lp = room.localParticipant
      const retryDelays = [0, 500, 1500, 3000, 5000, 8000, 12000, 18000, 24000]
      for (let attempt = 0; attempt < retryDelays.length; attempt++) {
        if (attempt > 0) {
          await new Promise((resolve) => window.setTimeout(resolve, retryDelays[attempt]!))
        }
        if (!isRoomPublishReady(room)) {
          const ready = await waitForRoomPublishReady(room, attempt === 0 ? 8_000 : 3_000)
          if (!ready) continue
        }
        try {
          for (const packet of packets) {
            await lp.publishData(encodeChatWire(packet), { reliable: true, topic: MEETING_CHAT_TOPIC })
          }
          if (import.meta.env.DEV) {
            console.log('[chat] published', { id, packets: packets.length, topic: MEETING_CHAT_TOPIC })
          }
          setChatMessages((prev) => prev.map((m): ChatMessage => (m.id === id ? { ...m, status: 'sent' } : m)))
          return true
        } catch (err) {
          const transient = isPublishUnavailableError(err) || (err instanceof Error && err.message === 'not connected')
          if (transient && attempt < retryDelays.length - 1) continue
          setChatMessages((prev) => prev.map((m): ChatMessage => (m.id === id ? { ...m, status: 'failed' } : m)))
          if (import.meta.env.DEV) console.error('[MeetingContext] failed to publish chat message:', err)
          return false
        }
      }
      return false
    },
    [room],
  )

  const retryPendingChatPublishes = useCallback(async () => {
    if (chatPublishInFlightRef.current || !isRoomPublishReady(room)) return
    chatPublishInFlightRef.current = true
    try {
      const pending = chatMessagesRef.current.filter((m) => m.isLocal && (m.status === 'sending' || m.status === 'failed'))
      for (const msg of pending) {
        const wirePayload: ChatWirePayload = {
          type: 'chat',
          id: msg.id,
          timestamp: msg.timestamp,
          senderName: msg.senderName,
          senderIdentity: msg.senderIdentity,
          message: msg.message,
          attachments: msg.attachments ?? [],
          ...(msg.poll ? { poll: msg.poll } : {}),
        }
        let packets: ReturnType<typeof buildChatWirePackets>
        try {
          packets = buildChatWirePackets(wirePayload)
        } catch {
          continue
        }
        setChatMessages((prev) =>
          prev.map((m): ChatMessage => (m.id === msg.id ? { ...m, status: 'sending' } : m)),
        )
        await publishChatPackets(msg.id, packets)
      }
    } finally {
      chatPublishInFlightRef.current = false
    }
  }, [room, publishChatPackets])

  useEffect(() => {
    const onRoomReady = () => {
      void waitForRoomPublishReady(room).then((ready) => {
        if (ready) void retryPendingChatPublishes()
      })
    }
    const onConnectionStateChanged = (state: ConnectionState) => {
      if (state === ConnectionState.Connected) onRoomReady()
    }
    room.on(RoomEvent.Connected, onRoomReady)
    room.on(RoomEvent.Reconnected, onRoomReady)
    room.on(RoomEvent.ConnectionStateChanged, onConnectionStateChanged)
    return () => {
      room.off(RoomEvent.Connected, onRoomReady)
      room.off(RoomEvent.Reconnected, onRoomReady)
      room.off(RoomEvent.ConnectionStateChanged, onConnectionStateChanged)
    }
  }, [room, retryPendingChatPublishes])

  useEffect(() => {
    const handler = (payload: Uint8Array, participant: unknown, _kind: unknown, topic?: string) => {
      try {
        const raw = JSON.parse(new TextDecoder().decode(payload))
        const isChatTopic = isMeetingChatDataTopic(topic, raw.type)

        if (import.meta.env.DEV && isChatTopic && typeof raw.type === 'string') {
          const from = (participant as { identity?: string } | null)?.identity ?? 'unknown'
          console.log('[chat] received', { topic: topic ?? '(none)', type: raw.type, from })
        }

        if (topic === 'presence' && raw.type === 'deafen_state' && typeof raw.identity === 'string') {
          setPeerDeafened((prev) => ({
            ...prev,
            [raw.identity]: raw.deafened === true,
          }))
          return
        }

        if (
          topic === 'presence' &&
          raw.type === 'profile_changed' &&
          typeof raw.identity === 'string'
        ) {
          const changedIdentity = raw.identity
          if (changedIdentity !== localIdentityRef.current && typeof raw.name === 'string') {
            const messageName = raw.name.trim()
            const messageAvatar =
              typeof raw.avatarUrl === 'string' && raw.avatarUrl.trim() ? raw.avatarUrl.trim() : undefined
            if (messageName) {
              applyRemoteProfileRef.current(changedIdentity, messageName, messageAvatar)
            }
          }
          void fetchRemoteParticipantProfileRef.current(changedIdentity)
          return
        }

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
              if (msg.target === currentUserId || msg.target === room.localParticipant.identity) {
                if (msg.event === 'deafen') setIsServerDeafened(true)
                else if (msg.event === 'undeafen') setIsServerDeafened(false)
              }
            }
          }
          return
        }

        if (isChatTopic && raw.type === 'reaction') {
          const messageId = typeof raw.messageId === 'string' ? raw.messageId : null
          const emoji = typeof raw.emoji === 'string' ? raw.emoji : null
          const voterIdentity = typeof raw.voterIdentity === 'string' ? raw.voterIdentity : null
          if (!messageId || !emoji || !voterIdentity) return

          setChatMessages((prev) =>
            prev.map((m) => {
              if (m.id !== messageId) return m
              return { ...m, reactions: applyReactionToggle(m.reactions, voterIdentity, emoji) }
            }),
          )
          return
        }

        if (isChatTopic && raw.type === 'poll_vote') {
          const messageId = typeof raw.messageId === 'string' ? raw.messageId : null
          const optionId = typeof raw.optionId === 'string' ? raw.optionId : null
          const voterIdentity = typeof raw.voterIdentity === 'string' ? raw.voterIdentity : null
          if (!messageId || !optionId || !voterIdentity) return

          setChatMessages((prev) =>
            prev.map((m) => {
              if (m.id !== messageId || !m.poll) return m
              if (!m.poll.options.some((o) => o.id === optionId)) return m
              return {
                ...m,
                poll: { ...m.poll, votes: { ...m.poll.votes, [voterIdentity]: optionId } },
              }
            }),
          )
          return
        }

        const appendRemoteChat = (wire: ChatWirePayload) => {
          const p = participant as { identity?: string; name?: string } | null
          const senderIdentity = wire.senderIdentity || p?.identity || ''
          const senderName = wire.senderName || p?.name || p?.identity || 'Unknown'
          const poll = normalizeChatPoll(wire.poll) ?? undefined
          const localIdentity = room.localParticipant.identity
          const isSelfMessage = !!senderIdentity && senderIdentity === localIdentity

          const msg: ChatMessage = {
            id: wire.id || generateID(),
            timestamp: wire.timestamp || Date.now(),
            senderName,
            senderIdentity,
            message: wire.message || '',
            attachments: normalizeAttachments(wire.attachments),
            poll,
            reactions: {},
            isLocal: isSelfMessage,
          }

          setChatMessages((prev) => {
            const existingIdx = prev.findIndex((m) => m.id === msg.id)
            if (existingIdx >= 0) {
              const existing = prev[existingIdx]
              if (!isSelfMessage && !existing.isLocal) return prev
              const merged: ChatMessage = {
                ...existing,
                ...msg,
                isLocal: true,
                reactions: existing.reactions,
                status: existing.status,
              }
              return prev.map((m, i) => (i === existingIdx ? merged : m))
            }

            const updated = [...prev, isSelfMessage ? { ...msg, isLocal: true } : msg]
            return capMessages(applyChatRetention(updated, ttlHours), MEMORY_CHAT_CAP)
          })
        }

        if (isChatTopic && raw.type === 'chat_chunk_meta') {
          const id = typeof raw.id === 'string' ? raw.id : null
          const messageChunks = typeof raw.messageChunks === 'number' ? raw.messageChunks : null
          const attachmentChunks = typeof raw.attachmentChunks === 'number' ? raw.attachmentChunks : null
          const pollChunks = typeof raw.pollChunks === 'number' ? raw.pollChunks : null
          if (
            !id ||
            messageChunks === null ||
            attachmentChunks === null ||
            pollChunks === null ||
            messageChunks < 0 ||
            attachmentChunks < 0 ||
            pollChunks < 0 ||
            messageChunks + attachmentChunks + pollChunks < 1
          ) {
            return
          }

          const meta: ChatChunkMetaWire = {
            type: 'chat_chunk_meta',
            id,
            timestamp: typeof raw.timestamp === 'number' ? raw.timestamp : Date.now(),
            senderName: typeof raw.senderName === 'string' ? raw.senderName : 'Unknown',
            senderIdentity: typeof raw.senderIdentity === 'string' ? raw.senderIdentity : '',
            messageChunks,
            attachmentChunks,
            pollChunks,
          }

          ingestChatChunk(chatChunkBuffersRef.current, meta)
          pruneChunkBuffers(chatChunkBuffersRef.current)
          return
        }

        if (isChatTopic && raw.type === 'chat_chunk') {
          const id = typeof raw.id === 'string' ? raw.id : null
          const kind = raw.kind === 'message' || raw.kind === 'attachments' || raw.kind === 'poll' ? raw.kind : null
          const index = typeof raw.index === 'number' ? raw.index : null
          const part = typeof raw.part === 'string' ? raw.part : null
          if (!id || !kind || index === null || part === null) return

          const pending = chatChunkBuffersRef.current.get(id)
          if (!pending) return

          const chunk: ChatChunkWire = { type: 'chat_chunk', id, kind, index, part }
          const done = applyChatChunkPart(pending, chunk)
          pruneChunkBuffers(chatChunkBuffersRef.current)
          if (!done) return

          chatChunkBuffersRef.current.delete(id)
          appendRemoteChat(assembledChatFromChunks(done))
          return
        }

        if (isChatTopic && raw.type === 'chat') {
          appendRemoteChat({
            type: 'chat',
            id: (raw.id as string) || generateID(),
            timestamp: (raw.timestamp as number) || Date.now(),
            senderName: (raw.senderName as string) || '',
            senderIdentity: (raw.senderIdentity as string) || '',
            message: (raw.message as string) || '',
            attachments: Array.isArray(raw.attachments) ? raw.attachments : [],
            ...(raw.poll !== undefined ? { poll: raw.poll } : {}),
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

  // Re-tag persisted or echoed messages once LiveKit identity is known.
  useEffect(() => {
    const localIdentity = room.localParticipant.identity
    if (!localIdentity) return
    setChatMessages((prev) => {
      let changed = false
      const next = prev.map((m) => {
        if (!m.isLocal && m.senderIdentity === localIdentity) {
          changed = true
          return { ...m, isLocal: true }
        }
        return m
      })
      return changed ? next : prev
    })
  }, [room.localParticipant.identity])

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
    chatSeenRef.current = chatMessagesRef.current.length
    systemSeenRef.current = systemMessagesRef.current.length
    setUnreadCount(0)
  }, []) // stable reference – does not cause chatValue to change on message arrival

  // sendChat publishes a reliable data packet on the "chat" topic.
  // The message is also echoed locally immediately for zero-latency feedback.
  const sendChat = useCallback(
    (text: string, attachments?: ChatAttachment[], poll?: ChatPoll) => {
      const lp = room.localParticipant
      const id = generateID()
      const timestamp = Date.now()
      const senderName = userNameRef.current || lp.name || lp.identity || 'You'
      const senderIdentity = lp.identity || ''
      const normalizedAttachments = (attachments ?? [])
        .map(normalizeChatAttachment)
        .filter((a): a is ChatAttachment => a !== null)
      const normalizedPoll = poll ? normalizeChatPoll(poll) : null
      if (!text.trim() && normalizedAttachments.length === 0 && !normalizedPoll) return

      const wirePayload: ChatWirePayload = {
        type: 'chat',
        id,
        timestamp,
        senderName,
        senderIdentity,
        message: text,
        attachments: normalizedAttachments,
        ...(normalizedPoll ? { poll: normalizedPoll } : {}),
      }

      let packets: ReturnType<typeof buildChatWirePackets>
      try {
        packets = buildChatWirePackets(wirePayload)
      } catch (err) {
        setChatMessages((prev) => prev.map((m): ChatMessage => (m.id === id ? { ...m, status: 'failed' } : m)))
        if (import.meta.env.DEV) console.error('[MeetingContext] failed to prepare chat message:', err)
        return
      }

      // Local echo first so data-channel echo cannot win the race as a remote message.
      setChatMessages((prev) => {
        const localMsg: ChatMessage = {
          id,
          timestamp,
          senderName,
          senderIdentity,
          message: text,
          attachments: normalizedAttachments,
          poll: normalizedPoll ?? undefined,
          reactions: {},
          isLocal: true,
          status: 'sending',
        }
        const existingIdx = prev.findIndex((m) => m.id === id)
        const updated =
          existingIdx >= 0
            ? prev.map((m, i) =>
                i === existingIdx ? { ...localMsg, reactions: m.reactions, status: 'sending' as const } : m,
              )
            : [...prev, localMsg]
        return capMessages(applyChatRetention(updated, ttlHours), MEMORY_CHAT_CAP)
      })

      void publishChatPackets(id, packets)
    },
    [publishChatPackets, ttlHours],
  )

  const reactToMessage = useCallback(
    (messageId: string, emoji: string) => {
      const lp = room.localParticipant
      const voterIdentity = lp.identity || ''
      if (!voterIdentity) return

      setChatMessages((prev) =>
        prev.map((m) => {
          if (m.id !== messageId) return m
          return { ...m, reactions: applyReactionToggle(m.reactions, voterIdentity, emoji) }
        }),
      )

      const payload = { type: 'reaction', messageId, emoji, voterIdentity }
      const data = new TextEncoder().encode(JSON.stringify(payload))
      void lp.publishData(data, { reliable: true, topic: MEETING_CHAT_TOPIC }).catch((err) => {
        if (import.meta.env.DEV) console.error('[MeetingContext] failed to publish reaction:', err)
      })
    },
    [room],
  )

  const votePoll = useCallback(
    (messageId: string, optionId: string) => {
      const lp = room.localParticipant
      const voterIdentity = lp.identity || ''
      if (!voterIdentity) return

      setChatMessages((prev) =>
        prev.map((m) => {
          if (m.id !== messageId || !m.poll) return m
          if (!m.poll.options.some((o) => o.id === optionId)) return m
          return {
            ...m,
            poll: { ...m.poll, votes: { ...m.poll.votes, [voterIdentity]: optionId } },
          }
        }),
      )

      const payload = { type: 'poll_vote', messageId, optionId, voterIdentity }
      const data = new TextEncoder().encode(JSON.stringify(payload))
      void lp.publishData(data, { reliable: true, topic: MEETING_CHAT_TOPIC }).catch((err) => {
        if (import.meta.env.DEV) console.error('[MeetingContext] failed to publish poll vote:', err)
      })
    },
    [room],
  )

  // Room context value — stable unless room metadata actually changes
  const roomValue = useMemo<MeetingRoomContextValue>(
    () => ({
      roomId,
      roomName,
      adminId,
      isPublic: roomIsPublic,
      setRoomIsPublic,
      currentUserId,
      isCreator,
      canManageRoomAccess,
      isAdmin: accesses.includes('admin') || isSuperAdmin,
      isModerator:
        accesses.includes('moderator') ||
        accesses.includes('admin') ||
        accesses.includes('superadmin') ||
        (!!hostUserId && (currentUserId === hostUserId || localIdentity === hostUserId)),
      isServerDeafened,
      isSelfDeafened,
      toggleSelfDeafen,
      isParticipantDeafened,
      getParticipantDisplayName,
      getParticipantAvatarUrl,
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
      localIdentity,
      roomIsPublic,
      currentUserId,
      isCreator,
      canManageRoomAccess,
      accesses,
      isSuperAdmin,
      isServerDeafened,
      isSelfDeafened,
      toggleSelfDeafen,
      isParticipantDeafened,
      getParticipantDisplayName,
      getParticipantAvatarUrl,
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
      votePoll,
      reactToMessage,
      unreadCount,
      markRead,
    }),
    [chatMessages, systemMessages, sendChat, votePoll, reactToMessage, unreadCount, markRead],
  )

  return (
    <MeetingRoomContext.Provider value={roomValue}>
      <MeetingChatContext.Provider value={chatValue}>{children}</MeetingChatContext.Provider>
    </MeetingRoomContext.Provider>
  )
}
