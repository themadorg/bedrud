import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'

import { DataTableFacetedFilter } from '#/components/admin/DataTableFacetedFilter'
import { DataTablePagination } from '#/components/admin/DataTablePagination'
import { DataTableSearch } from '#/components/admin/DataTableSearch'
import { DataTableToolbar } from '#/components/admin/DataTableToolbar'
import { RecentSignupsTable } from '#/components/admin/RecentSignupsTable'
import { Button } from '#/components/ui/button'
import { api } from '#/lib/api'
import type { RecentUser } from '#/lib/use-admin-overview'

export const Route = createFileRoute('/dashboard/admin/users_/recent-signups')({
  component: AdminRecentSignupsPage,
})

const PROVIDER_OPTS = [
  { label: 'Local', value: 'local' },
  { label: 'Google', value: 'google' },
  { label: 'GitHub', value: 'github' },
  { label: 'Guest', value: 'guest' },
]

function AdminRecentSignupsPage() {
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(50)
  const [search, setSearch] = useState('')
  const [provider, setProvider] = useState<string[]>([])
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')

  const params = new URLSearchParams()
  params.set('page', String(page))
  params.set('limit', String(limit))
  if (search) params.set('q', search)
  if (provider.length > 0) params.set('provider', provider.join(','))
  if (dateFrom) params.set('dateFrom', dateFrom)
  if (dateTo) params.set('dateTo', dateTo)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'recent-signups', page, limit, search, provider, dateFrom, dateTo],
    queryFn: () =>
      api.get<{ users: RecentUser[]; total: number; page: number; limit: number }>(
        `/api/admin/users/recent?${params.toString()}`,
      ),
  })

  const filtersActive = search || provider.length > 0 || dateFrom || dateTo

  function resetFilters() {
    setSearch('')
    setProvider([])
    setDateFrom('')
    setDateTo('')
    setPage(1)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Header with tabs */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Recent sign-ups</h1>
          <p className="text-xs text-muted-foreground">{data?.total ?? 0} new accounts</p>
        </div>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-0 border-b">
        <Link
          to="/dashboard/admin/users"
          className="border-b-2 border-transparent px-4 py-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          All Users
        </Link>
        <Link
          to="/dashboard/admin/users/recent-signups"
          className="border-b-2 border-primary px-4 py-2 text-[11px] font-medium text-foreground"
        >
          Recent sign-ups
        </Link>
      </div>

      {/* Toolbar */}
      <DataTableToolbar
        table={{
          activeFilterKeys: [
            ...(search ? [{ key: 'q', label: 'Search', value: search }] : []),
            ...provider.map((p) => ({ key: `provider:${p}`, label: 'Provider', value: p })),
            ...(dateFrom ? [{ key: 'dateFrom', label: 'From', value: dateFrom }] : []),
            ...(dateTo ? [{ key: 'dateTo', label: 'To', value: dateTo }] : []),
          ],
          hasActiveFilters: !!filtersActive,
          resetFilters,
          clearFilter: (key: string) => {
            if (key === 'q') setSearch('')
            if (key.startsWith('provider:')) {
              const removed = key.split(':')[1]
              setProvider((prev) => prev.filter((p) => p !== removed))
            }
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
          placeholder="Search by name or email…"
        />
        <DataTableFacetedFilter
          label="Provider"
          options={PROVIDER_OPTS}
          values={provider}
          onChange={(v) => {
            setProvider(v)
            setPage(1)
          }}
        />
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1">
            <label htmlFor="signups-date-from" className="text-[11px] text-muted-foreground">
              From
            </label>
            <input
              id="signups-date-from"
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
            <label htmlFor="signups-date-to" className="text-[11px] text-muted-foreground">
              To
            </label>
            <input
              id="signups-date-to"
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
            {error instanceof Error ? error.message : 'Failed to load recent sign-ups.'}
          </span>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-1.5 h-3 w-3" />
            Retry
          </Button>
        </div>
      ) : (
        <RecentSignupsTable users={data?.users ?? []} isLoading={isLoading} />
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
