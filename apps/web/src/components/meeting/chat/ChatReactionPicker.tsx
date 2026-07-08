import { useCallback } from 'react'
import { cn } from '@/lib/utils'
import { ChatEmojiPicker } from './ChatEmojiPicker'

interface Props {
  isLocal: boolean
  onReact: (emoji: string) => void
}

export function ChatReactionPicker({ isLocal, onReact }: Props) {
  const pickEmoji = useCallback(
    (emoji: string) => {
      onReact(emoji)
    },
    [onReact],
  )

  return (
    <ChatEmojiPicker
      onEmojiSelect={pickEmoji}
      mode="quick"
      side={isLocal ? 'left' : 'right'}
      align="center"
      size="sm"
      variant="boxed"
      ariaLabel="Add reaction"
      className={cn(
        'border-white/10 bg-[#0f0f1c]/90',
        'hover:text-white/55 data-[state=open]:opacity-100',
      )}
    />
  )
}
