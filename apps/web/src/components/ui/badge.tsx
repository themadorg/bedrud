import { cva, type VariantProps } from 'class-variance-authority'
import * as React from 'react'

import { cn } from '@/lib/utils'

const badgeVariants = cva(
  // The corner lives only in `shape`: `cn` runs the classes through tailwind-merge, which keeps the
  // last radius it sees and drops the rest, so a corner here would be silently dead.
  'inline-flex items-center border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        // A label beside what it describes, as the Android client's BedrudBadge draws it.
        default: 'border-transparent bg-accent-container text-accent-container-foreground',
        primary: 'border-transparent bg-primary text-primary-foreground hover:bg-primary/80',
        secondary: 'border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80',
        destructive: 'border-transparent bg-destructive text-destructive-foreground hover:bg-destructive/80',
        outline: 'text-foreground',
      },
      // A badge is a pill; a filter chip keeps the shape scale's 8px chip corner, as the Android
      // client's BedrudBadge and FilterChip do.
      shape: {
        pill: 'rounded-full',
        chip: 'rounded-sm',
      },
    },
    defaultVariants: {
      variant: 'default',
      shape: 'pill',
    },
  },
)

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, shape, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant, shape }), className)} {...props} />
}

export { Badge, badgeVariants }
