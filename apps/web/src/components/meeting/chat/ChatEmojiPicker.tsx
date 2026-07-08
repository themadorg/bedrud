import { Smile } from 'lucide-react'
import { lazy, Suspense, useCallback, useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { QUICK_REACTIONS } from './chatReactions'

const BedrudEmojiPicker = lazy(() =>
  import('@/components/meeting/chat/emoji-picker/EmojiPicker').then((m) => ({
    default: m.EmojiPicker,
  })),
)

interface Props {
  onEmojiSelect: (emoji: string) => void
  disabled?: boolean
  mode?: 'quick' | 'full'
  side?: 'top' | 'right' | 'bottom' | 'left'
  align?: 'start' | 'center' | 'end'
  size?: 'sm' | 'md'
  variant?: 'ghost' | 'boxed'
  className?: string
  ariaLabel?: string
}

export function ChatEmojiPicker({
  onEmojiSelect,
  disabled = false,
  mode = 'full',
  side = 'top',
  align = 'end',
  size = 'md',
  variant = 'boxed',
  className,
  ariaLabel = 'Insert emoji',
}: Props) {
  const [open, setOpen] = useState(false)
  const [expanded, setExpanded] = useState(false)

  const handleOpenChange = useCallback((next: boolean) => {
    setOpen(next)
    if (!next) setExpanded(false)
  }, [])

  const pickEmoji = useCallback(
    (emoji: string) => {
      onEmojiSelect(emoji)
      setOpen(false)
      setExpanded(false)
    },
    [onEmojiSelect],
  )

  const buttonSize = size === 'sm' ? 'h-7 w-7' : 'h-9 w-9'
  const iconSize = size === 'sm' ? 14 : 16

  const pickerContent =
    mode === 'full' || expanded ? (
      <Suspense
        fallback={
          <div className="flex h-[320px] w-[min(300px,85vw)] items-center justify-center rounded-xl border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] text-xs text-[var(--meet-fg-muted)]">
            Loading emojis…
          </div>
        }
      >
        <BedrudEmojiPicker onEmojiSelect={pickEmoji} autoFocus />
      </Suspense>
    ) : (
      <div className="flex w-[148px] flex-col gap-0.5 rounded-xl border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] p-1.5 shadow-[var(--meet-shadow)]">
        <div className="grid grid-cols-4 gap-0.5">
          {QUICK_REACTIONS.slice(0, 4).map((emoji) => (
            <button
              key={emoji}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pickEmoji(emoji)}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-[18px] leading-none hover:bg-[var(--meet-control-hover)]"
              aria-label={`Insert ${emoji}`}
            >
              {emoji}
            </button>
          ))}
        </div>
        <div className="grid grid-cols-4 gap-0.5">
          {QUICK_REACTIONS.slice(4, 8).map((emoji) => (
            <button
              key={emoji}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pickEmoji(emoji)}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-[18px] leading-none hover:bg-[var(--meet-control-hover)]"
              aria-label={`Insert ${emoji}`}
            >
              {emoji}
            </button>
          ))}
        </div>
        <button
          type="button"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setExpanded(true)}
          className="flex h-7 w-full items-center justify-center rounded-lg text-[13px] font-semibold leading-none text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control-hover)]"
          aria-label="More emojis"
        >
          ···
        </button>
      </div>
    )

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          onMouseDown={(e) => e.preventDefault()}
          className={cn(
            'mb-0 flex shrink-0 items-center justify-center transition-colors',
            buttonSize,
            variant === 'boxed' && 'rounded-xl border border-[var(--meet-border)] bg-[var(--meet-control)]',
            variant === 'ghost' && 'border-none bg-transparent p-0',
            open ? 'text-[var(--meet-accent)]' : 'text-[var(--meet-fg-muted)] hover:text-[var(--meet-accent)]',
            disabled && 'cursor-default opacity-40',
            className,
          )}
          aria-label={ariaLabel}
        >
          <Smile size={iconSize} />
        </button>
      </PopoverTrigger>
      <PopoverContent
        side={side}
        align={align}
        className="w-auto border-[var(--meet-border)] bg-transparent p-0 shadow-none"
      >
        {pickerContent}
      </PopoverContent>
    </Popover>
  )
}
