import type { ReactNode } from 'react'

import { DataTableFilterChips } from '@/components/admin/DataTableFilterChips'

interface ActiveFilter {
  key: string
  label: string
  value: string
}

interface TableToolbarOptions {
  activeFilterKeys: ActiveFilter[]
  hasActiveFilters: boolean
  clearFilter: (key: string) => void
  resetFilters: () => void
}

interface DataTableToolbarProps {
  children: ReactNode
  table: TableToolbarOptions
}

export function DataTableToolbar({ children, table }: DataTableToolbarProps) {
  return (
    <div className="border bg-card px-4 py-3 space-y-3">
      <div className="flex flex-wrap items-center gap-2">{children}</div>
      <DataTableFilterChips
        filters={table.activeFilterKeys}
        onRemove={(key) => table.clearFilter(key)}
        onClearAll={table.resetFilters}
      />
    </div>
  )
}
