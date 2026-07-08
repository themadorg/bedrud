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
