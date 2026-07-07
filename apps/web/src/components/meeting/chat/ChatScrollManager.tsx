import { ArrowDown } from 'lucide-react'
import { actionBubbleChrome } from './chatBubbleStyles'

interface Props {
  show: boolean
  unreadCount: number
  onScrollToBottom: () => void
}

export function ChatScrollManager({ show, unreadCount, onScrollToBottom }: Props) {
  if (!show) return null

  const label = unreadCount === 1 ? '1 new message' : `${unreadCount} new messages`

  return (
    <>
      {unreadCount > 0 && (
        <div className="absolute bottom-2 left-1/2 z-[6] -translate-x-1/2">
          <button
            type="button"
            onClick={onScrollToBottom}
            className="cursor-pointer whitespace-nowrap px-3.5 py-2 text-xs font-semibold leading-none"
            style={actionBubbleChrome()}
          >
            {label}
          </button>
        </div>
      )}

      <button
        type="button"
        onClick={onScrollToBottom}
        aria-label="Scroll to latest messages"
        className="absolute bottom-1.5 right-1 z-[5] flex h-7 w-7 cursor-pointer items-center justify-center border-none bg-transparent p-0 text-teal-400/90 hover:text-teal-400"
      >
        <ArrowDown size={14} />
        {unreadCount > 0 && (
          <span
            className="absolute -right-[5px] -top-[5px] flex h-4 w-4 items-center justify-center rounded-full text-[9px] font-bold text-primary-foreground"
            style={{
              background: 'color-mix(in oklab, var(--primary) 90%, transparent)',
              border: '1px solid color-mix(in oklab, var(--accent-400) 30%, transparent)',
            }}
          >
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>
    </>
  )
}
