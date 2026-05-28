import { DataTableFacetedFilter } from '@/components/admin/DataTableFacetedFilter'
import { DataTableFilterChips } from '@/components/admin/DataTableFilterChips'
import { DataTableSearch } from '@/components/admin/DataTableSearch'
import type { useTableState } from '@/components/admin/useTableState'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export const VISIBILITY_OPTS = [
  { label: 'Public', value: 'public' },
  { label: 'Private', value: 'private' },
]

export const STATUS_OPTS = [
  { label: 'Live', value: 'active' },
  { label: 'Suspended', value: 'suspended' },
]

export const CAPACITY_OPTS = [
  { label: 'Empty', value: 'empty' },
  { label: '1–5', value: '1-5' },
  { label: '6–20', value: '6-20' },
  { label: '20+', value: '20+' },
]

export const CREATED_OPTS = [
  { label: 'Today', value: 'today' },
  { label: 'Last 7 days', value: '7d' },
  { label: 'Last 30 days', value: '30d' },
]

interface AdminControlBarProps {
  table: ReturnType<typeof useTableState<any>>
  showTabs?: boolean
  advancedFilters?: boolean
}

export function AdminControlBar({ table, showTabs = false, advancedFilters = false }: AdminControlBarProps) {
  const currentTab = table.filters.statusTab ?? 'all'

  return (
    <div className="border bg-card">
      <div className="px-4 py-3 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <DataTableSearch
            value={table.filters.q ?? ''}
            onChange={(v) => table.setFilter('q', v)}
            placeholder="Search rooms, owner, tags..."
          />
          <DataTableFacetedFilter
            label="Status"
            options={STATUS_OPTS}
            values={table.filters.status ?? []}
            onChange={(v) => table.setFilter('status', v)}
          />
          <DataTableFacetedFilter
            label="Visibility"
            options={VISIBILITY_OPTS}
            values={table.filters.visibility ?? []}
            onChange={(v) => table.setFilter('visibility', v)}
          />
          {advancedFilters && (
            <>
              <div className="text-xs text-muted-foreground px-1">|</div>
              <DataTableFacetedFilter
                label="Capacity"
                options={CAPACITY_OPTS}
                values={table.filters.capacity ? [table.filters.capacity as string] : []}
                onChange={(v) => table.setFilter('capacity', v.length > 0 ? v[0] : null)}
              />
              <DataTableFacetedFilter
                label="Created"
                options={CREATED_OPTS}
                values={table.filters.created ? [table.filters.created as string] : []}
                onChange={(v) => table.setFilter('created', v.length > 0 ? v[0] : null)}
              />
            </>
          )}
          {table.hasActiveFilters && (
            <Button variant="ghost" size="sm" onClick={table.resetFilters} className="h-8 text-xs">
              Reset
            </Button>
          )}
        </div>

        {showTabs && (
          <Tabs
            value={currentTab}
            onValueChange={(v) => {
              table.setFilter('statusTab', v === 'all' ? null : v)
              if (v === 'all') {
                table.clearFilter('status')
              } else if (v === 'live') {
                table.setFilter('status', ['active'])
              } else if (v === 'private') {
                table.setFilter('visibility', ['private'])
              } else if (v === 'archived') {
                table.setFilter('status', ['suspended'])
              }
            }}
          >
            <TabsList className="h-8">
              <TabsTrigger value="all" className="text-xs px-3">
                All
              </TabsTrigger>
              <TabsTrigger value="live" className="text-xs px-3">
                Live
              </TabsTrigger>
              <TabsTrigger value="private" className="text-xs px-3">
                Private
              </TabsTrigger>
              <TabsTrigger value="archived" className="text-xs px-3">
                Archived
              </TabsTrigger>
            </TabsList>
          </Tabs>
        )}
      </div>

      <DataTableFilterChips
        filters={table.activeFilterKeys}
        onRemove={(key) => table.clearFilter(key)}
        onClearAll={table.resetFilters}
      />
    </div>
  )
}
