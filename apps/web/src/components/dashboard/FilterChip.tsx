import { Check } from 'lucide-react'

import { badgeVariants } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  label: string
  selected: boolean
  onSelect: () => void
}

/**
 * One filter chip, matching the Android client's `FilterChip`: a leading check appears on the
 * selected chip only, so the active filter reads as "this one is on" rather than relying on the
 * fill colour alone.
 *
 * The corner and typography come from `badgeVariants`, which owns the chip shape for the whole app.
 * No radius class is added here — `cn` keeps only the last one it sees, so a second would silently
 * replace the token.
 */
export function FilterChip({ label, selected, onSelect }: Props) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        badgeVariants({ variant: selected ? 'default' : 'outline' }),
        'h-8 cursor-pointer gap-1.5 px-3',
        selected ? 'border-transparent' : 'border-input hover:bg-accent',
      )}
    >
      {selected && <Check className="h-3.5 w-3.5" />}
      {label}
    </button>
  )
}
