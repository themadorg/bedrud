import { Headphones } from 'lucide-react'

import { cn } from '@/lib/utils'

interface DeafenHeadphonesIconProps {
  size: number
  off?: boolean
  className?: string
}

export function DeafenHeadphonesIcon({ size, off = false, className }: DeafenHeadphonesIconProps) {
  return (
    <span className={cn('relative inline-flex shrink-0', className)}>
      <Headphones size={size} aria-hidden />
      {off && (
        <svg
          role="presentation"
          className="pointer-events-none absolute inset-0 h-full w-full"
          viewBox="0 0 24 24"
          fill="none"
        >
          <line x1="4" y1="4" x2="20" y2="20" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      )}
    </span>
  )
}

/** @deprecated Alias kept so stale HMR bundles that still call RailDeafenIcon do not crash. */
export function RailDeafenIcon({ off = false, className }: { off?: boolean; className?: string }) {
  return <DeafenHeadphonesIcon size={16} off={off} className={cn('h-4 w-4', className)} />
}
