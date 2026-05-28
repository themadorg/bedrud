import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { DataTableBulkBar } from '#/components/admin/DataTableBulkBar'
import { DataTableFacetedFilter } from '#/components/admin/DataTableFacetedFilter'
import { DataTablePagination } from '#/components/admin/DataTablePagination'
import { DataTableSearch } from '#/components/admin/DataTableSearch'
import { DataTableToolbar } from '#/components/admin/DataTableToolbar'
import { type AdminRoom, RoomTable } from '#/components/admin/RoomTable'
import { useTableState } from '#/components/admin/useTableState'
import { Button } from '#/components/ui/button'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { useAdminContext } from '#/routes/dashboard/admin.tsx'
import { CreateRoomDialog, type RoomSettings } from '@/components/dashboard/CreateRoomDialog'

interface RoomsSearch {
  create?: boolean
}

export const Route = createFileRoute('/dashboard/admin/rooms')({
  validateSearch: (search: Record<string, unknown>): RoomsSearch => ({
    create: search.create === true || search.create === 'true',
  }),
  component: AdminRoomsPage,
})

const VISIBILITY_OPTS = [
  { label: 'Public', value: 'public' },
  { label: 'Private', value: 'private' },
]

const STATUS_OPTS = [
  { label: 'Live', value: 'active' },
  { label: 'Suspended', value: 'suspended' },
  { label: 'Archived', value: 'archived' },
]

const CAPACITY_OPTS = [
  { label: 'Empty', value: 'empty' },
  { label: '1–5', value: '1-5' },
  { label: '6–20', value: '6-20' },
  { label: '20+', value: '20+' },
]

const CREATED_OPTS = [
  { label: 'Today', value: 'today' },
  { label: 'Last 7 days', value: '7d' },
  { label: 'Last 30 days', value: '30d' },
]

function AdminRoomsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { isReadOnly } = useAdminContext()
  const [pendingRoomIds, setPendingRoomIds] = useState<Set<string>>(new Set())
  const [confirmBulkAction, setConfirmBulkAction] = useState<'suspend' | 'close' | null>(null)
  const [createOpen, setCreateOpen] = useState(false)

  const [statusFilter, setStatusFilter] = useState<string[]>([])

  const { create: autoCreate } = Route.useSearch()

  useEffect(() => {
    if (autoCreate) setCreateOpen(true)
  }, [autoCreate])

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'rooms', 'v2', ...statusFilter],
    queryFn: () => {
      let url = '/api/admin/rooms?limit=1000'
      if (statusFilter.length > 0) {
        url += '&status=' + statusFilter.join(',')
      }
      return api.get<{ rooms: AdminRoom[]; total: number; page: number; limit: number }>(url)
    },
  })

  const table = useTableState({
    items: (data?.rooms ?? []) as AdminRoom[],
    defaultSort: { key: 'createdAt', order: 'desc' },
  })

  const suspendRoom = useMutation({
    mutationFn: (id: string) => api.post(`/api/admin/rooms/${id}/suspend`, {}),
    onSuccess: (_data, roomId) => {
      setPendingRoomIds((prev) => new Set(prev).add(roomId))
      toast.success('Room suspension queued')
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to queue suspension')),
  })

  const unsuspendRoom = useMutation({
    mutationFn: (id: string) => api.post(`/api/admin/rooms/${id}/reactivate`, {}),
    onSuccess: () => {
      toast.success('Room reactivated')
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to reactivate room')),
  })

  const closeRoom = useMutation({
    mutationFn: (id: string) => api.delete(`/api/admin/rooms/${id}`),
    onSuccess: (_data, roomId) => {
      setPendingRoomIds((prev) => new Set(prev).add(roomId))
      toast.success('Room close queued')
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to close room')),
  })

  const deleteRoom = useMutation({
    mutationFn: (id: string) => api.delete(`/api/admin/rooms/${id}`),
    onSuccess: (_data, roomId) => {
      setPendingRoomIds((prev) => new Set(prev).add(roomId))
      toast.success('Room deletion queued')
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to delete room')),
  })

  async function handleCreate(data: {
    name?: string
    isPublic: boolean
    maxParticipants: number
    settings: RoomSettings
  }) {
    await api.post('/api/room/create', data)
    setCreateOpen(false)
    queryClient.invalidateQueries({ queryKey: ['admin', 'rooms', 'v2'] })
  }

  const updateLimit = useMutation({
    mutationFn: ({ id, max }: { id: string; max: number }) =>
      api.put(`/api/admin/rooms/${id}`, { maxParticipants: max }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update room limit')),
  })

  const bulkSuspend = useMutation({
    mutationFn: (ids: string[]) => api.post('/api/admin/rooms/bulk/suspend', { ids }),
    onSuccess: () => {
      toast.success(`${table.selectedIds.size} rooms queued for suspend`)
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
      table.clearSelection()
      setConfirmBulkAction(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to queue bulk suspend'))
      setConfirmBulkAction(null)
    },
  })

  const bulkClose = useMutation({
    mutationFn: (ids: string[]) => api.post('/api/admin/rooms/bulk/close', { ids }),
    onSuccess: () => {
      toast.success(`${table.selectedIds.size} rooms queued for close`)
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'] })
      table.clearSelection()
      setConfirmBulkAction(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to queue bulk close'))
      setConfirmBulkAction(null)
    },
  })

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Rooms</h1>
          <p className="text-xs text-muted-foreground">{data?.total ?? 0} rooms in this instance</p>
        </div>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Create room
        </Button>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-0 border-b">
        <Link
          to="/dashboard/admin/rooms"
          className="border-b-2 border-primary px-4 py-2 text-[11px] font-medium text-foreground"
        >
          All Rooms
        </Link>
        <Link
          to="/dashboard/admin/rooms/events"
          className="border-b-2 border-transparent px-4 py-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          Room events
        </Link>
      </div>

      {/* Toolbar */}
      <DataTableToolbar table={table}>
        <DataTableSearch
          value={table.filters.q ?? ''}
          onChange={(v) => table.setFilter('q', v)}
          placeholder="Search rooms…"
        />
        <DataTableFacetedFilter
          label="Visibility"
          options={VISIBILITY_OPTS}
          values={table.filters.visibility ?? []}
          onChange={(v) => table.setFilter('visibility', v)}
        />
        <DataTableFacetedFilter
          label="Status"
          options={STATUS_OPTS}
          values={statusFilter}
          onChange={(v) => {
            table.setFilter('status', v)
            setStatusFilter(v)
          }}
        />
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
        {table.hasActiveFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              table.resetFilters()
              setStatusFilter([])
            }}
            className="h-8 text-xs"
          >
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
            label: `Suspend (${table.selectedIds.size})`,
            onClick: () => setConfirmBulkAction('suspend'),
            variant: 'outline',
          },
          {
            label: `Close (${table.selectedIds.size})`,
            onClick: () => setConfirmBulkAction('close'),
            variant: 'destructive',
          },
        ]}
      />

      {/* Table */}
      <RoomTable
        rooms={table.paginated}
        isLoading={isLoading}
        table={table}
        onSuspend={(id) => suspendRoom.mutate(id)}
        onUnsuspend={(id) => unsuspendRoom.mutate(id)}
        onClose={(id) => closeRoom.mutate(id)}
        onDelete={(id) => deleteRoom.mutate(id)}
        onUpdateLimit={(id, max) => updateLimit.mutate({ id, max })}
        onRoomClick={(id) => navigate({ to: '/dashboard/admin/rooms/$roomId', params: { roomId: id } })}
        isReadOnly={isReadOnly}
        pendingRoomIds={pendingRoomIds}
        suspendPending={suspendRoom.isPending}
        deletePending={deleteRoom.isPending}
        closePending={closeRoom.isPending}
      />

      {/* Pagination */}
      <DataTablePagination
        total={table.total}
        page={table.page}
        limit={table.limit}
        onPageChange={table.setPage}
        onLimitChange={table.setLimit}
      />

      {/* Bulk confirm dialog */}
      {confirmBulkAction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-background border p-6 max-w-md w-full mx-4 space-y-4">
            <h2 className="text-sm font-semibold">
              {confirmBulkAction === 'suspend' ? 'Suspend' : 'Close'} {table.selectedIds.size} room
              {table.selectedIds.size !== 1 ? 's' : ''}?
            </h2>
            <p className="text-xs text-muted-foreground">
              {confirmBulkAction === 'suspend'
                ? 'This will end any active calls but preserve room data. Rooms can be reactivated later.'
                : 'This will end all active calls and permanently delete room data. This cannot be undone.'}
            </p>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setConfirmBulkAction(null)}>
                Cancel
              </Button>
              <Button
                variant={confirmBulkAction === 'close' ? 'destructive' : 'default'}
                size="sm"
                onClick={() => {
                  const ids = Array.from(table.selectedIds)
                  if (confirmBulkAction === 'suspend') bulkSuspend.mutate(ids)
                  else bulkClose.mutate(ids)
                }}
                disabled={bulkSuspend.isPending || bulkClose.isPending}
              >
                {confirmBulkAction === 'suspend' ? 'Suspend' : 'Close'} {table.selectedIds.size} room
                {table.selectedIds.size !== 1 ? 's' : ''}
              </Button>
            </div>
          </div>
        </div>
      )}

      <CreateRoomDialog open={createOpen} onOpenChange={setCreateOpen} onCreate={handleCreate} isAdmin />
    </div>
  )
}
