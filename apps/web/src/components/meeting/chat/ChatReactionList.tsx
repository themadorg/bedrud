import { type ChatReactions, groupReactions } from './chatReactions'

interface Props {
  reactions: ChatReactions
  currentIdentity: string
  isLocal: boolean
  onReact: (emoji: string) => void
}

export function ChatReactionList({ reactions, currentIdentity, isLocal, onReact }: Props) {
  const grouped = groupReactions(reactions, currentIdentity)
  if (grouped.length === 0) return null

  return (
    <div className="mt-1 flex flex-wrap gap-1" style={{ justifyContent: isLocal ? 'flex-end' : 'flex-start' }}>
      {grouped.map(({ emoji, count, mine }) => (
        <button
          key={emoji}
          type="button"
          onClick={() => onReact(emoji)}
          className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] leading-none transition-colors"
          style={{
            borderColor: mine ? 'color-mix(in oklab, var(--accent-400) 45%, transparent)' : 'rgba(255,255,255,0.12)',
            background: mine ? 'color-mix(in oklab, var(--primary) 25%, transparent)' : 'rgba(255,255,255,0.06)',
          }}
          aria-label={`${emoji} ${count} reaction${count === 1 ? '' : 's'}`}
        >
          <span className="text-[13px]">{emoji}</span>
          <span className="tabular-nums text-white/70">{count}</span>
        </button>
      ))}
    </div>
  )
}
