import { useState } from 'react'
import type { ChatPoll } from '../MeetingContext'
import { ChatPollResultsModal } from './ChatPollResultsModal'

interface Props {
  poll: ChatPoll
  messageId: string
  isLocal: boolean
  currentIdentity: string
  onVote: (messageId: string, optionId: string) => void
}

export function ChatPollBubble({ poll, messageId, isLocal, currentIdentity, onVote }: Props) {
  const [resultsOpen, setResultsOpen] = useState(false)
  const voteEntries = Object.values(poll.votes)
  const totalVotes = voteEntries.length
  const myVote = poll.votes[currentIdentity]

  return (
    <>
      <div className="min-w-[200px]">
        <p className="m-0 mb-2 text-[13px] font-semibold leading-snug text-white/95">{poll.question}</p>
        <div className="flex flex-col gap-1.5">
          {poll.options.map((option) => {
            const count = voteEntries.filter((id) => id === option.id).length
            const pct = totalVotes > 0 ? Math.round((count / totalVotes) * 100) : 0
            const selected = myVote === option.id

            return (
              <button
                key={option.id}
                type="button"
                onClick={() => onVote(messageId, option.id)}
                className="relative overflow-hidden rounded-lg border px-2.5 py-2 text-start transition-colors"
                style={{
                  borderColor: selected
                    ? 'color-mix(in oklab, var(--accent-400) 45%, transparent)'
                    : 'rgba(255,255,255,0.1)',
                  background: selected
                    ? 'color-mix(in oklab, var(--primary) 20%, transparent)'
                    : 'rgba(255,255,255,0.04)',
                }}
              >
                {totalVotes > 0 && (
                  <span
                    className="absolute inset-y-0 left-0 rounded-lg"
                    style={{
                      width: `${pct}%`,
                      background: isLocal
                        ? 'color-mix(in oklab, var(--primary) 35%, transparent)'
                        : 'rgba(255,255,255,0.08)',
                    }}
                  />
                )}
                <span className="relative z-[1] flex min-h-5 items-center justify-between gap-2">
                  <span className="flex items-center text-[12px] leading-snug text-white/90">{option.text}</span>
                  <span className="flex shrink-0 items-center text-[11px] tabular-nums leading-none text-white/55">
                    {totalVotes > 0 ? `${pct}%` : ''}
                  </span>
                </span>
              </button>
            )
          })}
        </div>
        <div className="mt-2 flex items-center justify-between gap-2">
          <p className="m-0 text-[10px] text-white/45">
            {totalVotes} vote{totalVotes === 1 ? '' : 's'}
          </p>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => setResultsOpen(true)}
            className="border-none bg-transparent p-0 text-[10px] font-medium text-accent-400/90 hover:text-accent-400"
          >
            View results
          </button>
        </div>
      </div>

      <ChatPollResultsModal
        poll={poll}
        open={resultsOpen}
        onOpenChange={setResultsOpen}
        currentIdentity={currentIdentity}
      />
    </>
  )
}
