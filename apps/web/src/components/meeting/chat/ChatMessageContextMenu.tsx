import { useParticipants } from '@livekit/components-react'
import { BarChart3, Copy, Info } from 'lucide-react'
import { type ReactNode, useCallback, useMemo, useState } from 'react'
import { meetingElevatedOverlayClass } from '@/components/meeting/MeetingElevatedLeftDock'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { cn } from '@/lib/utils'
import type { ChatMessage } from '../MeetingContext'
import { ChatMessageInfoModal } from './ChatMessageInfoModal'
import { ChatPollResultsModal } from './ChatPollResultsModal'
import { absoluteTime } from './chatGrouping'
import { groupReactionsByEmoji, QUICK_REACTIONS } from './chatReactions'

interface Props {
  message: ChatMessage
  senderName: string
  currentIdentity: string
  onReact: (emoji: string) => void
  children: ReactNode
  elevated?: boolean
}

export function ChatMessageContextMenu({
  message,
  senderName,
  currentIdentity,
  onReact,
  children,
  elevated = false,
}: Props) {
  const [infoOpen, setInfoOpen] = useState(false)
  const [resultsOpen, setResultsOpen] = useState(false)
  const participants = useParticipants()
  const poll = message.poll

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

  const hasCopyableText = message.message.trim().length > 0

  const copyText = useCallback(() => {
    const text = message.message.trim()
    if (!text) return
    void navigator.clipboard.writeText(text)
  }, [message.message])

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent
        className={cn(
          'meet-dialog min-w-[200px] max-w-[min(260px,85vw)] border-white/10 bg-[#0f0f1c]/98 p-1 text-white/90 shadow-lg backdrop-blur-xl',
          elevated ? meetingElevatedOverlayClass : 'z-50',
        )}
      >
        <ContextMenuGroup className="grid grid-cols-4 gap-0.5 px-2 py-1.5">
          {QUICK_REACTIONS.map((emoji) => (
            <ContextMenuItem
              key={emoji}
              onSelect={() => onReact(emoji)}
              className="flex h-8 w-8 items-center justify-center p-0 text-[18px] leading-none focus:bg-white/[0.08] focus:text-white"
              aria-label={`React with ${emoji}`}
            >
              {emoji}
            </ContextMenuItem>
          ))}
        </ContextMenuGroup>

        <ContextMenuSeparator className="bg-white/[0.08]" />

        <ContextMenuLabel className="px-2 pt-1.5 pb-0 text-[13px] font-semibold leading-tight text-white/95">
          {senderName}
        </ContextMenuLabel>
        <ContextMenuLabel className="px-2 pb-1.5 pt-0 text-[11px] font-normal text-white/45">
          {absoluteTime(message.timestamp)}
        </ContextMenuLabel>

        {reactions.length > 0 && (
          <>
            <ContextMenuSeparator className="bg-white/[0.08]" />
            <ContextMenuLabel className="px-2 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wide text-white/40">
              Seen by
            </ContextMenuLabel>
            <div className="flex flex-col gap-1 px-2 pb-1.5">
              {reactions.map(({ emoji, voters }) => (
                <div key={emoji} className="flex items-start gap-2 text-[12px] leading-snug">
                  <span className="shrink-0 text-[15px] leading-none">{emoji}</span>
                  <span className="min-w-0 break-words text-white/65">
                    {voters.map((voter, i) => (
                      <span key={voter.identity}>
                        {i > 0 && ', '}
                        <span className={voter.mine ? 'text-accent-400/90' : undefined}>
                          {voter.mine ? 'You' : voter.name}
                        </span>
                      </span>
                    ))}
                  </span>
                </div>
              ))}
            </div>
          </>
        )}

        <ContextMenuSeparator className="bg-white/[0.08]" />
        {poll && (
          <ContextMenuItem
            onClick={() => setResultsOpen(true)}
            className="gap-2 text-[13px] text-white/85 focus:bg-white/[0.08] focus:text-white"
          >
            <BarChart3 className="size-3.5 opacity-70" />
            View results
          </ContextMenuItem>
        )}
        <ContextMenuItem
          onClick={() => setInfoOpen(true)}
          className="gap-2 text-[13px] text-white/85 focus:bg-white/[0.08] focus:text-white"
        >
          <Info className="size-3.5 opacity-70" />
          Message info
        </ContextMenuItem>
        {hasCopyableText && (
          <ContextMenuItem
            onClick={copyText}
            className="gap-2 text-[13px] text-white/85 focus:bg-white/[0.08] focus:text-white"
          >
            <Copy className="size-3.5 opacity-70" />
            Copy text
          </ContextMenuItem>
        )}
      </ContextMenuContent>

      {poll && (
        <ChatPollResultsModal
          poll={poll}
          open={resultsOpen}
          onOpenChange={setResultsOpen}
          currentIdentity={currentIdentity}
          aboveElevatedDock={elevated}
        />
      )}

      <ChatMessageInfoModal
        message={message}
        senderName={senderName}
        open={infoOpen}
        onOpenChange={setInfoOpen}
        currentIdentity={currentIdentity}
        aboveElevatedDock={elevated}
      />
    </ContextMenu>
  )
}
