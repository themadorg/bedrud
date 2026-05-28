import { Button } from '@/components/ui/button'

interface BulkAction {
  label: string
  onClick: () => void
  variant?: 'default' | 'destructive' | 'outline'
}

interface DataTableBulkBarProps {
  selectedCount: number
  onClear: () => void
  actions: BulkAction[]
}

export function DataTableBulkBar({ selectedCount, onClear, actions }: DataTableBulkBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center gap-3 border bg-muted/30 px-4 py-3">
      <span className="text-xs font-medium text-muted-foreground whitespace-nowrap">{selectedCount} selected</span>
      <div className="flex items-center gap-2">
        {actions.map((action, i) => (
          <Button
            key={i}
            variant={action.variant ?? 'outline'}
            size="sm"
            type="button"
            onClick={action.onClick}
            className="gap-1.5"
          >
            {action.label}
          </Button>
        ))}
      </div>
      <Button
        variant="ghost"
        size="sm"
        type="button"
        onClick={onClear}
        className="ml-auto text-xs text-muted-foreground"
      >
        Clear selection
      </Button>
    </div>
  )
}
