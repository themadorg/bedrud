import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'

import { DataTableFacetedFilter } from '#/components/admin/DataTableFacetedFilter'
import { DataTablePagination } from '#/components/admin/DataTablePagination'
import { DataTableSearch } from '#/components/admin/DataTableSearch'
import { DataTableToolbar } from '#/components/admin/DataTableToolbar'
import { RoomEventsTable } from '#/components/admin/RoomEventsTable'
import { Button } from '#/components/ui/button'
import { api } from '#/lib/api'
import type { AdminRoomEvent } from '#/lib/use-admin-overview'

export const Route = createFileRoute('/dashboard/admin/rooms_/events')({
  component: AdminRoomEventsPage,
})

const TYPE_OPTS = [
  { label: 'Room created', value: 'room_created' },
  { label: 'User joined', value: 'room_joined' },
]

function AdminRoomEventsPage() {
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(50)
  const [search, setSearch] = useState('')
  const [types, setTypes] = useState<string[]>([])
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')

  const params = new URLSearchParams()
  params.set('page', String(page))
  params.set('limit', String(limit))
  if (search) params.set('q', search)
  if (types.length > 0) params.set('type', types.join(','))
  if (dateFrom) params.set('dateFrom', dateFrom)
  if (dateTo) params.set('dateTo', dateTo)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'room-events', page, limit, search, types, dateFrom, dateTo],
    queryFn: () =>
      api.get<{ events: AdminRoomEvent[]; total: number; page: number; limit: number }>(
        `/api/admin/rooms/events?${params.toString()}`,
      ),
  })

  const filtersActive = search || types.length > 0 || dateFrom || dateTo

  function resetFilters() {
    setSearch('')
    setTypes([])
    setDateFrom('')
    setDateTo('')
    setPage(1)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Room events</h1>
          <p className="text-xs text-muted-foreground">{data?.total ?? 0} events recorded</p>
        </div>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-0 border-b">
        <Link
          to="/dashboard/admin/rooms"
          className="border-b-2 border-transparent px-4 py-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          All Rooms
        </Link>
        <Link
          to="/dashboard/admin/rooms/events"
          className="border-b-2 border-primary px-4 py-2 text-[11px] font-medium text-foreground"
        >
          Room events
        </Link>
      </div>

      {/* Toolbar */}
      <DataTableToolbar
        table={{
          activeFilterKeys: [
            ...(search ? [{ key: 'q', label: 'Search', value: search }] : []),
            ...types.map((t) => ({
              key: 'type',
              label: 'Type',
              value: t === 'room_created' ? 'Room created' : 'User joined',
            })),
            ...(dateFrom ? [{ key: 'dateFrom', label: 'From', value: dateFrom }] : []),
            ...(dateTo ? [{ key: 'dateTo', label: 'To', value: dateTo }] : []),
          ],
          hasActiveFilters: !!filtersActive,
          resetFilters,
          clearFilter: (key: string) => {
            if (key === 'q') setSearch('')
            if (key === 'type') setTypes([])
            if (key === 'dateFrom') setDateFrom('')
            if (key === 'dateTo') setDateTo('')
          },
        }}
      >
        <DataTableSearch
          value={search}
          onChange={(v) => {
            setSearch(v)
            setPage(1)
          }}
          placeholder="Search rooms or users…"
        />
        <DataTableFacetedFilter
          label="Type"
          options={TYPE_OPTS}
          values={types}
          onChange={(v) => {
            setTypes(v)
            setPage(1)
          }}
        />
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1">
            <label htmlFor="events-date-from" className="text-[11px] text-muted-foreground">
              From
            </label>
            <input
              id="events-date-from"
              type="date"
              value={dateFrom}
              onChange={(e) => {
                setDateFrom(e.target.value)
                setPage(1)
              }}
              className="h-8 w-36 rounded border bg-background px-2 text-[11px]"
            />
          </div>
          <div className="flex items-center gap-1">
            <label htmlFor="events-date-to" className="text-[11px] text-muted-foreground">
              To
            </label>
            <input
              id="events-date-to"
              type="date"
              value={dateTo}
              onChange={(e) => {
                setDateTo(e.target.value)
                setPage(1)
              }}
              className="h-8 w-36 rounded border bg-background px-2 text-[11px]"
            />
          </div>
        </div>
        {filtersActive && (
          <Button variant="ghost" size="sm" onClick={resetFilters} className="h-8 text-xs">
            Reset
          </Button>
        )}
      </DataTableToolbar>

      {/* Table */}
      {isError ? (
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-4 text-sm flex items-center justify-between">
          <span className="text-destructive">
            {error instanceof Error ? error.message : 'Failed to load room events.'}
          </span>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-1.5 h-3 w-3" />
            Retry
          </Button>
        </div>
      ) : (
        <RoomEventsTable events={data?.events ?? []} isLoading={isLoading} />
      )}

      {/* Pagination */}
      <DataTablePagination
        total={data?.total ?? 0}
        page={page}
        limit={limit}
        onPageChange={setPage}
        onLimitChange={(n) => {
          setLimit(n)
          setPage(1)
        }}
      />
    </div>
  )
}
