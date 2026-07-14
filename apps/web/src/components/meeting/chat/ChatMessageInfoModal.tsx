import { useParticipants } from '@livekit/components-react'
import { useCallback, useMemo } from 'react'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { ChatMessage } from '../MeetingContext'
import { absoluteTime } from './chatGrouping'
import { groupReactionsByEmoji } from './chatReactions'

interface Props {
  message: ChatMessage
  senderName: string
  open: boolean
  onOpenChange: (open: boolean) => void
  currentIdentity: string
  aboveElevatedDock?: boolean
}

export function messagePreview(message: ChatMessage): string {
  const parts: string[] = []
  const text = message.message.trim()
  if (text) parts.push(text)
  if (message.poll) parts.push(`Poll: ${message.poll.question}`)
  let hasImage = false
  const fileNames: string[] = []
  for (const att of message.attachments) {
    if (att.kind === 'file') fileNames.push(att.name)
    else hasImage = true
  }
  if (hasImage) parts.push('Image')
  if (fileNames.length === 1) parts.push(`File: ${fileNames[0]}`)
  else if (fileNames.length > 1) parts.push(`${fileNames.length} files`)
  return parts.join(' · ') || 'Empty message'
}

export function ChatMessageInfoModal({
  message,
  senderName,
  open,
  onOpenChange,
  currentIdentity,
  aboveElevatedDock = false,
}: Props) {
  const participants = useParticipants()

  const resolveName = useCallback(
    (identity: string) => {
      const match = participants.find((p) => p.identity === identity)
      return match?.name || match?.identity || identity
    },
    [participants],
  )

  const reactions = useMemo(
    () => groupReactionsByEmoji(message.reactions, currentIdentity, resolveName),
    [message.reactions, currentIdentity, resolveName],
  )

  const reactionCount = Object.keys(message.reactions).length

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        aboveElevatedDock={aboveElevatedDock}
        className="meet-dialog max-h-[min(calc(var(--app-height,100svh)*0.85),520px)] max-w-[min(380px,calc(var(--app-width,100svw)-2rem))] gap-0 overflow-hidden p-0 shadow-2xl"
      >
        <DialogHeader className="border-b border-white/[0.08] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-white/90">Message info</DialogTitle>
        </DialogHeader>

        <div className="meet-scroll flex max-h-[min(calc(var(--app-height,100svh)*0.6),400px)] flex-col gap-4 overflow-y-auto px-4 py-4">
          <div className="flex flex-col gap-1">
            <span className="text-[11px] font-medium uppercase tracking-wide text-white/40">Sent by</span>
            <p className="m-0 text-[14px] font-semibold text-white/95">{senderName}</p>
            <p className="m-0 text-[11px] text-white/45">{absoluteTime(message.timestamp)}</p>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-[11px] font-medium uppercase tracking-wide text-white/40">Message</span>
            <p className="m-0 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-white/80">
              {messagePreview(message)}
            </p>
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] font-medium uppercase tracking-wide text-white/40">Reactions</span>
              <span className="text-[11px] text-white/45">
                {reactionCount} reaction{reactionCount === 1 ? '' : 's'}
              </span>
            </div>

            {reactions.length === 0 ? (
              <p className="m-0 text-[13px] text-white/45">No reactions yet.</p>
            ) : (
              <div className="flex flex-col gap-2.5">
                {reactions.map(({ emoji, voters }) => (
                  <div key={emoji} className="rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-2.5">
                    <div className="mb-1.5 flex items-center gap-2">
                      <span className="text-[18px] leading-none">{emoji}</span>
                      <span className="text-[12px] tabular-nums text-white/55">
                        {voters.length} {voters.length === 1 ? 'person' : 'people'}
                      </span>
                    </div>
                    <ul className="m-0 flex list-none flex-col gap-1 p-0">
                      {voters.map((voter) => (
                        <li key={voter.identity} className="flex items-center justify-between gap-2 text-[13px]">
                          <span className="text-white/85">{voter.name}</span>
                          {voter.mine && <span className="text-[11px] text-accent-400/90">You</span>}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            )}
          </div>
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
