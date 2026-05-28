import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'

interface AdminBulkBarProps {
  selectedCount: number
  onClear: () => void
  children?: ReactNode
}

export function AdminBulkBar({ selectedCount, onClear, children }: AdminBulkBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center gap-2 border-b bg-muted/30 px-4 py-2.5">
      <span className="text-xs font-medium text-muted-foreground whitespace-nowrap shrink-0">
        {selectedCount} selected
      </span>
      <div className="flex items-center gap-1.5 flex-wrap">{children}</div>
      <Button
        variant="ghost"
        size="sm"
        type="button"
        onClick={onClear}
        className="ml-auto text-xs text-muted-foreground shrink-0"
      >
        Clear
      </Button>
    </div>
  )
}
