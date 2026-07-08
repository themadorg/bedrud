export const QUICK_REACTIONS = ['👍', '❤️', '😂', '😮', '😢', '🎉', '🔥', '👀'] as const

export type QuickReaction = (typeof QUICK_REACTIONS)[number]

const EMOJI_REACTION_RE = /\p{Extended_Pictographic}/u

export function isValidReactionEmoji(emoji: string): boolean {
  if (!emoji || emoji.length > 32) return false
  return EMOJI_REACTION_RE.test(emoji)
}

export type ChatReactions = Record<string, string>

export function normalizeReactions(raw: unknown): ChatReactions {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
  const out: ChatReactions = {}
  for (const [identity, emoji] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof identity === 'string' && typeof emoji === 'string' && isValidReactionEmoji(emoji)) {
      out[identity] = emoji
    }
  }
  return out
}

export function applyReactionToggle(reactions: ChatReactions, voterIdentity: string, emoji: string): ChatReactions {
  if (!isValidReactionEmoji(emoji)) return reactions
  const next = { ...reactions }
  if (next[voterIdentity] === emoji) {
    delete next[voterIdentity]
  } else {
    next[voterIdentity] = emoji
  }
  return next
}

export interface GroupedReaction {
  emoji: string
  count: number
  mine: boolean
}

export function groupReactions(reactions: ChatReactions, currentIdentity: string): GroupedReaction[] {
  const map = new Map<string, GroupedReaction>()
  for (const [identity, emoji] of Object.entries(reactions)) {
    if (!isValidReactionEmoji(emoji)) continue
    const entry = map.get(emoji) ?? { emoji, count: 0, mine: false }
    entry.count += 1
    if (identity === currentIdentity) entry.mine = true
    map.set(emoji, entry)
  }
  return Array.from(map.values())
}

export interface ReactionVoter {
  identity: string
  name: string
  mine: boolean
}

export interface ReactionsByEmoji {
  emoji: string
  voters: ReactionVoter[]
}

export function groupReactionsByEmoji(
  reactions: ChatReactions,
  currentIdentity: string,
  resolveName: (identity: string) => string,
): ReactionsByEmoji[] {
  const map = new Map<string, ReactionVoter[]>()

  for (const [identity, emoji] of Object.entries(reactions)) {
    if (!isValidReactionEmoji(emoji)) continue
    const voters = map.get(emoji) ?? []
    voters.push({
      identity,
      name: resolveName(identity),
      mine: identity === currentIdentity,
    })
    map.set(emoji, voters)
  }

  return Array.from(map.entries())
    .map(([emoji, voters]) => ({ emoji, voters }))
    .sort((a, b) => b.voters.length - a.voters.length || a.emoji.localeCompare(b.emoji))
}
