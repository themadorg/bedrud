import { useParticipants } from '@livekit/components-react'
import { useCallback, useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { ChatPoll } from '../MeetingContext'
import { computePollResults } from './chatPollUtils'

interface Props {
  poll: ChatPoll
  open: boolean
  onOpenChange: (open: boolean) => void
  currentIdentity: string
}

export function ChatPollResultsModal({ poll, open, onOpenChange, currentIdentity }: Props) {
  const participants = useParticipants()

  const resolveName = useCallback(
    (identity: string) => {
      const match = participants.find((p) => p.identity === identity)
      return match?.name || match?.identity || identity
    },
    [participants],
  )

  const results = useMemo(() => computePollResults(poll, resolveName), [poll, resolveName])
  const totalVotes = Object.keys(poll.votes).length
  const myVote = poll.votes[currentIdentity]

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="meet-dialog max-h-[min(calc(var(--app-height,100svh)*0.85),520px)] max-w-[min(380px,calc(var(--app-width,100svw)-2rem))] gap-0 overflow-hidden p-0 shadow-2xl">
        <DialogHeader className="border-b border-white/[0.08] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-white/90">Poll results</DialogTitle>
        </DialogHeader>

        <div className="meet-scroll flex max-h-[min(calc(var(--app-height,100svh)*0.6),400px)] flex-col gap-3 overflow-y-auto px-4 py-4">
          <p className="m-0 text-[14px] font-semibold leading-snug text-white/95">{poll.question}</p>

          <div className="flex flex-col gap-2.5">
            {results.map((row) => {
              const selected = myVote === row.optionId
              return (
                <div key={row.optionId} className="flex flex-col gap-1">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-[13px] leading-snug text-white/90">{row.text}</span>
                    <span className="shrink-0 text-[12px] tabular-nums text-white/55">
                      {totalVotes > 0 ? `${row.pct}%` : '0%'}
                    </span>
                  </div>
                  <div
                    className="relative h-2 overflow-hidden rounded-sm"
                    style={{ background: 'rgba(255,255,255,0.08)' }}
                  >
                    <span
                      className="absolute inset-y-0 left-0 rounded-sm"
                      style={{
                        width: `${row.pct}%`,
                        background: selected
                          ? 'color-mix(in oklab, var(--primary) 75%, transparent)'
                          : 'color-mix(in oklab, var(--accent-400) 65%, transparent)',
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between gap-2 text-[11px] text-white/45">
                    <span>
                      {row.count} vote{row.count === 1 ? '' : 's'}
                    </span>
                    {selected && <span className="text-accent-400/90">Your vote</span>}
                  </div>
                  {row.voterNames.length > 0 && (
                    <p className="m-0 text-[11px] leading-relaxed text-white/40">{row.voterNames.join(', ')}</p>
                  )}
                </div>
              )
            })}
          </div>

          <p className="m-0 border-t border-white/[0.06] pt-2 text-[11px] text-white/45">
            {totalVotes} total vote{totalVotes === 1 ? '' : 's'}
          </p>
        </div>

        <DialogFooter className="border-t border-white/[0.08] px-4 py-3 sm:justify-end">
          <Button
            type="button"
            size="sm"
            onClick={() => onOpenChange(false)}
            className="h-8 bg-primary/90 text-primary-foreground hover:bg-primary"
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
