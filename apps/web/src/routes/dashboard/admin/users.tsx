import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { AlertConfirmDialog } from '#/components/admin/AlertConfirmDialog'
import { DataTableBulkBar } from '#/components/admin/DataTableBulkBar'
import { DataTableFacetedFilter } from '#/components/admin/DataTableFacetedFilter'
import { DataTablePagination } from '#/components/admin/DataTablePagination'
import { DataTableSearch } from '#/components/admin/DataTableSearch'
import { DataTableToolbar } from '#/components/admin/DataTableToolbar'
import { useTableState } from '#/components/admin/useTableState'
import { Button } from '#/components/ui/button'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { useAdminContext } from '#/routes/dashboard/admin.tsx'

export const Route = createFileRoute('/dashboard/admin/users')({ component: AdminUsersPage })

const PROVIDER_OPTS = [
  { label: 'Local', value: 'local' },
  { label: 'Google', value: 'google' },
  { label: 'GitHub', value: 'github' },
  { label: 'Guest', value: 'guest' },
]

const ROLE_OPTS = [
  { label: 'Superadmin', value: 'superadmin' },
  { label: 'Admin', value: 'admin' },
  { label: 'Moderator', value: 'moderator' },
  { label: 'User', value: 'user' },
  { label: 'Guest', value: 'guest' },
]

const STATUS_OPTS = [
  { label: 'Active', value: 'active' },
  { label: 'Banned', value: 'banned' },
]

const CREATED_OPTS = [
  { label: 'Today', value: 'today' },
  { label: 'Last 7 days', value: '7d' },
  { label: 'Last 30 days', value: '30d' },
]

function detectRole(accesses: string[] | null): string {
  if (!accesses || accesses.length === 0) return 'user'
  if (accesses.includes('superadmin')) return 'superadmin'
  if (accesses.includes('admin')) return 'admin'
  if (accesses.includes('moderator')) return 'moderator'
  if (accesses.includes('guest')) return 'guest'
  return 'user'
}

function getRoleLabel(accesses: string[] | null): string {
  const role = detectRole(accesses)
  const found = ROLE_OPTS.find((r) => r.value === role)
  return found ? found.label : 'User'
}

function AdminUsersPage() {
  const queryClient = useQueryClient()
  const { isReadOnly, currentUserId } = useAdminContext()
  const [confirmBulkAction, setConfirmBulkAction] = useState<'ban' | 'promote' | 'delete' | null>(null)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'users', 'v2'],
    queryFn: () =>
      api.get<{ users: AdminUser[]; total: number; page: number; limit: number }>('/api/admin/users?limit=1000'),
  })

  const table = useTableState({
    items: (data?.users ?? []) as AdminUser[],
    defaultSort: { key: 'createdAt', order: 'desc' },
  })

  const toggleStatus = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      api.put(`/api/admin/users/${id}/status`, { active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update user status')),
  })

  const changeRole = useMutation({
    mutationFn: ({ id, accesses }: { id: string; accesses: string[] }) =>
      api.put(`/api/admin/users/${id}/accesses`, { accesses }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update user role')),
  })

  const deleteUser = useMutation({
    mutationFn: (id: string) => api.delete(`/api/admin/users/${id}`),
    onSuccess: () => {
      toast.success('User deletion queued. This may take a moment.')
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to delete user')),
  })

  const bulkBan = useMutation({
    mutationFn: (ids: string[]) => api.post('/api/admin/users/bulk/ban', { ids }),
    onSuccess: () => {
      toast.success(`${table.selectedIds.size} users banned`)
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      table.clearSelection()
      setConfirmBulkAction(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to ban users'))
      setConfirmBulkAction(null)
    },
  })

  const bulkPromote = useMutation({
    mutationFn: (ids: string[]) => api.post('/api/admin/users/bulk/promote', { ids }),
    onSuccess: () => {
      toast.success(`${table.selectedIds.size} users promoted`)
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      table.clearSelection()
      setConfirmBulkAction(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to promote users'))
      setConfirmBulkAction(null)
    },
  })

  const bulkDelete = useMutation({
    mutationFn: (ids: string[]) => api.post('/api/admin/users/bulk/delete', { ids }),
    onSuccess: () => {
      toast.success(`${table.selectedIds.size} user deletions queued`)
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      table.clearSelection()
      setConfirmBulkAction(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to queue deletion'))
      setConfirmBulkAction(null)
    },
  })

  const bulkActionLabels = {
    ban: {
      title: `Ban ${table.selectedIds.size} user${table.selectedIds.size !== 1 ? 's' : ''}?`,
      desc: 'This will log them out immediately and prevent future sign-ins.',
      color: 'destructive' as const,
    },
    promote: {
      title: `Promote ${table.selectedIds.size} user${table.selectedIds.size !== 1 ? 's' : ''}?`,
      desc: 'Opens the bulk promotion flow. Full role management (including Admin and Moderator) is available on the individual user detail page.',
      color: 'default' as const,
    },
    delete: {
      title: `Delete ${table.selectedIds.size} user${table.selectedIds.size !== 1 ? 's' : ''}?`,
      desc: 'This will end all active calls and permanently remove all associated data. This cannot be undone.',
      color: 'destructive' as const,
    },
  } as const

  const isBulkPending = bulkBan.isPending || bulkPromote.isPending || bulkDelete.isPending

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Users</h1>
          <p className="text-xs text-muted-foreground">{data?.total ?? 0} registered accounts</p>
        </div>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-0 border-b">
        <Link
          to="/dashboard/admin/users"
          className="border-b-2 border-primary px-4 py-2 text-[11px] font-medium text-foreground"
        >
          All Users
        </Link>
        <Link
          to="/dashboard/admin/users/recent-signups"
          className="border-b-2 border-transparent px-4 py-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          Recent sign-ups
        </Link>
      </div>

      {/* Toolbar */}
      <DataTableToolbar table={table}>
        <DataTableSearch
          value={table.filters.q ?? ''}
          onChange={(v) => table.setFilter('q', v)}
          placeholder="Search by name or email…"
        />
        <DataTableFacetedFilter
          label="Provider"
          options={PROVIDER_OPTS}
          values={table.filters.provider ?? []}
          onChange={(v) => table.setFilter('provider', v)}
        />
        <DataTableFacetedFilter
          label="Role"
          options={ROLE_OPTS}
          values={table.filters.role ?? []}
          onChange={(v) => table.setFilter('role', v)}
        />
        <DataTableFacetedFilter
          label="Status"
          options={STATUS_OPTS}
          values={table.filters.status ?? []}
          onChange={(v) => table.setFilter('status', v)}
        />
        <DataTableFacetedFilter
          label="Created"
          options={CREATED_OPTS}
          values={table.filters.created ? [table.filters.created as string] : []}
          onChange={(v) => table.setFilter('created', v.length > 0 ? v[0] : null)}
        />
        {table.hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={table.resetFilters} className="h-8 text-xs">
            Reset
          </Button>
        )}
      </DataTableToolbar>

      {/* Bulk bar */}
      <DataTableBulkBar
        selectedCount={table.selectedIds.size}
        onClear={table.clearSelection}
        actions={[
          {
            label: `Ban (${table.selectedIds.size})`,
            onClick: () => setConfirmBulkAction('ban'),
            variant: 'destructive',
          },
          {
            label: `Promote (${table.selectedIds.size})`,
            onClick: () => setConfirmBulkAction('promote'),
          },
          {
            label: `Delete (${table.selectedIds.size})`,
            onClick: () => setConfirmBulkAction('delete'),
            variant: 'destructive',
          },
        ]}
      />

      {/* Table */}
      {isError ? (
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-4 text-sm flex items-center justify-between">
          <span className="text-destructive">{error instanceof Error ? error.message : 'Failed to load users.'}</span>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-1.5 h-3 w-3" />
            Retry
          </Button>
        </div>
      ) : (
        <div className="border overflow-hidden">
          {isLoading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">Loading users…</div>
          ) : table.paginated.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">No users found.</div>
          ) : (
            <div className="divide-y">
              {table.paginated.map((user) => (
                <div key={user.id} className="flex items-center justify-between px-4 py-3 text-sm">
                  <div>
                    <div className="font-medium">{user.name}</div>
                    <div className="text-xs text-muted-foreground">{user.email}</div>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs px-2 py-0.5 rounded border text-muted-foreground">
                      {getRoleLabel(user.accesses)}
                    </span>
                    <Link
                      to="/dashboard/admin/users/$userId"
                      params={{ userId: user.id }}
                      className="text-xs text-primary hover:underline"
                    >
                      Manage roles →
                    </Link>
                    <div className="text-xs text-muted-foreground font-mono">{user.id.slice(0, 8)}…</div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Pagination */}
      <DataTablePagination
        total={table.total}
        page={table.page}
        limit={table.limit}
        onPageChange={table.setPage}
        onLimitChange={table.setLimit}
      />

      {/* Bulk confirm dialog */}
      <AlertConfirmDialog
        open={confirmBulkAction !== null}
        onOpenChange={(open) => !open && setConfirmBulkAction(null)}
        title={confirmBulkAction ? bulkActionLabels[confirmBulkAction].title : ''}
        description={confirmBulkAction ? bulkActionLabels[confirmBulkAction].desc : ''}
        confirmLabel={
          confirmBulkAction
            ? `${confirmBulkAction === 'ban' ? 'Ban' : confirmBulkAction === 'promote' ? 'Promote' : 'Delete'} ${table.selectedIds.size} user${table.selectedIds.size !== 1 ? 's' : ''}`
            : 'Confirm'
        }
        onConfirm={() => {
          if (!confirmBulkAction) return
          const allIds = Array.from(table.selectedIds)
          const ids = currentUserId ? allIds.filter((id) => id !== currentUserId) : allIds
          if (ids.length === 0) {
            toast.warning('Cannot perform action on yourself')
            setConfirmBulkAction(null)
            return
          }
          switch (confirmBulkAction) {
            case 'ban':
              bulkBan.mutate(ids)
              break
            case 'promote':
              bulkPromote.mutate(ids)
              break
            case 'delete':
              bulkDelete.mutate(ids)
              break
          }
        }}
        variant={
          confirmBulkAction && bulkActionLabels[confirmBulkAction].color === 'destructive' ? 'destructive' : 'default'
        }
        isLoading={isBulkPending}
      />
    </div>
  )
}
