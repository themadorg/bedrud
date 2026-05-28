import { X } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

interface FilterChip {
  key: string
  label: string
  value: string
}

interface DataTableFilterChipsProps {
  filters: FilterChip[]
  onRemove: (key: string) => void
  onClearAll: () => void
}

export function DataTableFilterChips({ filters, onRemove, onClearAll }: DataTableFilterChipsProps) {
  if (filters.length === 0) return null

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {filters.map((f, i) => (
        <Badge
          key={`${f.key}-${f.value}-${i}`}
          variant="secondary"
          className="gap-1 px-2 py-0.5 text-[10px] font-normal"
        >
          {f.label}: {f.value}
          <button
            type="button"
            onClick={() => onRemove(f.key)}
            className="ml-0.5 hover:text-foreground"
            aria-label={`Remove ${f.label} filter`}
          >
            <X className="h-2.5 w-2.5" />
          </button>
        </Badge>
      ))}
      <Button variant="ghost" size="sm" onClick={onClearAll} className="h-5 px-1.5 text-[10px] text-muted-foreground">
        Clear all
      </Button>
    </div>
  )
}
