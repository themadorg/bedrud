import { Link } from '@tanstack/react-router'
import { cn } from '#/lib/utils'

const SIZES = {
  sm: { box: 'h-6 w-6', icon: 'h-3.5 w-3.5', text: 'text-xs' },
  md: { box: 'h-7 w-7', icon: 'h-4 w-4', text: 'text-xs' },
  lg: { box: 'h-9 w-9', icon: 'h-5 w-5', text: 'text-sm' },
} as const

function BedrudMark({ className, iconClassName }: { className?: string; iconClassName?: string }) {
  return (
    <span className={cn('flex shrink-0 items-center justify-center overflow-hidden bg-primary', className)}>
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={cn('text-primary-foreground', iconClassName)}
        aria-hidden
      >
        <path d="M16.247 7.761a6 6 0 0 1 0 8.478" />
        <path d="M19.075 4.933a10 10 0 0 1 0 14.134" />
        <path d="M4.925 19.067a10 10 0 0 1 0-14.134" />
        <path d="M7.753 16.239a6 6 0 0 1 0-8.478" />
        <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
      </svg>
    </span>
  )
}

interface BedrudLogoProps {
  className?: string
  showWordmark?: boolean
  size?: keyof typeof SIZES
}

export function BedrudLogo({ className, showWordmark = true, size = 'sm' }: BedrudLogoProps) {
  const s = SIZES[size]
  return (
    <Link
      to="/"
      className={cn('flex items-center gap-2 text-foreground transition-opacity hover:opacity-90', className)}
    >
      <BedrudMark className={cn(s.box, 'rounded-[4px]')} iconClassName={s.icon} />
      {showWordmark ? (
        <span className={cn('font-mono font-bold tracking-wider uppercase', s.text)}>bedrud</span>
      ) : null}
    </Link>
  )
}
