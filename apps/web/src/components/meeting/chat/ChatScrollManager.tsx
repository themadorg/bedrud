import { ArrowDown } from 'lucide-react'

interface Props {
  show: boolean
  unreadCount: number
  onScrollToBottom: () => void
}

export function ChatScrollManager({ show, unreadCount, onScrollToBottom }: Props) {
  if (!show) return null

  const label = unreadCount === 1 ? '↑ 1 new message' : `↑ ${unreadCount} new messages`

  return (
    <>
      {/* Inline banner — pill centered just above the input */}
      {unreadCount > 0 && (
        <div className="absolute bottom-[calc(88px+env(safe-area-inset-bottom)+8px)] left-1/2 -translate-x-1/2 z-[6]">
          <button
            type="button"
            onClick={onScrollToBottom}
            className="border-none rounded-full px-3.5 py-[5px] text-white text-xs font-semibold cursor-pointer whitespace-nowrap shadow-[0_2px_8px_rgba(0,0,0,0.4)]"
            style={{ background: 'color-mix(in oklab, var(--primary) 85%, transparent)' }}
          >
            {label}
          </button>
        </div>
      )}

      {/* Floating ↓ button */}
      <button
        type="button"
        onClick={onScrollToBottom}
        aria-label="Scroll to latest messages"
        className="absolute bottom-[calc(88px+env(safe-area-inset-bottom)+56px)] right-3.5 w-[34px] h-[34px] rounded-full border border-white/[0.12] bg-[#1e1e32]/92 flex items-center justify-center cursor-pointer shadow-[0_2px_8px_rgba(0,0,0,0.4)] z-[5]"
        style={{ color: 'color-mix(in oklab, var(--accent-400) 90%, transparent)' }}
      >
        <ArrowDown size={14} />
        {unreadCount > 0 && (
          <span
            className="absolute -top-[5px] -right-[5px] text-white text-[9px] font-bold rounded-full w-4 h-4 flex items-center justify-center"
            style={{ background: 'color-mix(in oklab, var(--primary) 90%, transparent)' }}
          >
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>
    </>
  )
}
