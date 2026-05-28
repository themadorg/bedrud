import { Globe, Lock, Pin } from 'lucide-react'
import { useState } from 'react'

import { AlertConfirmDialog } from '@/components/admin/AlertConfirmDialog'
import { type AdminRoom, RowActionsDropdown } from '@/components/admin/RowActionsDropdown'
import type { useTableState } from '@/components/admin/useTableState'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'

function relativeTime(dateStr: string | null | undefined): string {
  if (!dateStr) return '—'
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diff = now - then
  if (diff < 0) return 'just now'
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

interface RoomTableProps {
  rooms: AdminRoom[]
  isLoading: boolean
  table: ReturnType<typeof useTableState<AdminRoom>>
  onSuspend: (id: string) => void
  onUnsuspend?: (id: string) => void
  onClose: (id: string) => void
  onDelete: (id: string) => void
  onUpdateLimit: (id: string, max: number) => void
  onRoomClick: (id: string) => void
  onOwnerClick?: (userId: string) => void
  isReadOnly?: boolean
  pendingRoomIds?: Set<string>
  suspendPending?: boolean
  deletePending?: boolean
  closePending?: boolean
}

export type { AdminRoom }

export function RoomTable({
  rooms,
  isLoading,
  table,
  onSuspend,
  onUnsuspend,
  onClose,
  onDelete,
  onUpdateLimit,
  onRoomClick,
  onOwnerClick,
  isReadOnly,
  pendingRoomIds,
  suspendPending,
  deletePending,
  closePending,
}: RoomTableProps) {
  const [confirmAction, setConfirmAction] = useState<{
    id: string
    type: 'suspend' | 'unsuspend' | 'close' | 'delete'
  } | null>(null)
  const [editCapDialog, setEditCapDialog] = useState<{ id: string; value: number } | null>(null)

  if (isLoading) {
    return (
      <div className="border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox checked={false} disabled />
              </TableHead>
              <TableHead>Room</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Vis.</TableHead>
              <TableHead>Users</TableHead>
              <TableHead className="hidden sm:table-cell">Owner</TableHead>
              <TableHead className="hidden lg:table-cell">Created</TableHead>
              <TableHead className="hidden lg:table-cell">Last activity</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {[1, 2, 3, 4].map((i) => (
              <TableRow key={i}>
                {Array.from({ length: 9 }, (_, j) => (
                  <TableCell key={j}>
                    <Skeleton className="h-3.5" />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    )
  }

  function handleConfirm() {
    if (!confirmAction) return
    switch (confirmAction.type) {
      case 'suspend':
        onSuspend(confirmAction.id)
        break
      case 'unsuspend':
        onUnsuspend?.(confirmAction.id)
        break
      case 'close':
        onClose(confirmAction.id)
        break
      case 'delete':
        onDelete(confirmAction.id)
        break
    }
    setConfirmAction(null)
  }

  const confirmLabels: Record<string, string> = {
    suspend: 'Suspend',
    unsuspend: 'Unsuspend',
    close: 'Close room',
    delete: 'Delete room',
  }
  const confirmDescriptions: Record<string, string> = {
    suspend: 'This will end any active call but preserve room data. The room can be reactivated later.',
    unsuspend: 'This will reactivate the room, making it joinable again.',
    close: 'This will end all active calls and permanently delete room data. This cannot be undone.',
    delete: 'This will permanently delete the room and all associated data. This cannot be undone.',
  }

  return (
    <>
      {/* Desktop table */}
      <div className="border overflow-hidden hidden sm:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={
                    table.isAllSelected || table.isIndeterminate
                      ? table.isIndeterminate
                        ? 'indeterminate'
                        : true
                      : false
                  }
                  onCheckedChange={table.selectPage}
                  aria-label="Select all"
                />
              </TableHead>
              <TableHead>
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('name')}
                >
                  Room {table.sortKey === 'name' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead>
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('isActive')}
                >
                  Status {table.sortKey === 'isActive' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="text-xs">Vis.</TableHead>
              <TableHead>
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('participantsCount')}
                >
                  Users {table.sortKey === 'participantsCount' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="hidden sm:table-cell">
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('createdBy')}
                >
                  Owner {table.sortKey === 'createdBy' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="hidden lg:table-cell">
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('createdAt')}
                >
                  Created {table.sortKey === 'createdAt' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="hidden lg:table-cell">
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('lastActivityAt')}
                >
                  Last activity {table.sortKey === 'lastActivityAt' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {rooms.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="text-center text-muted-foreground py-8 text-xs">
                  No rooms found
                </TableCell>
              </TableRow>
            ) : (
              rooms.map((room) => (
                <TableRow key={room.id} data-state={table.selectedIds.has(room.id) ? 'selected' : undefined}>
                  <TableCell>
                    <Checkbox
                      checked={table.selectedIds.has(room.id)}
                      onCheckedChange={() => table.selectOne(room.id)}
                      aria-label={`Select ${room.name}`}
                    />
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    <div className="flex items-center gap-1.5">
                      {room.settings?.isPersistent && <Pin className="h-3 w-3 shrink-0 text-primary" />}
                      <button
                        type="button"
                        onClick={() => onRoomClick(room.id)}
                        className="truncate hover:text-primary transition-colors text-left max-w-[180px]"
                      >
                        {room.name}
                      </button>
                    </div>
                  </TableCell>
                  <TableCell>
                    {pendingRoomIds?.has(room.id) ? (
                      <Badge
                        variant="outline"
                        className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-500 text-[10px]"
                      >
                        Queued…
                      </Badge>
                    ) : (
                      <Badge
                        variant="outline"
                        className={cn(
                          'gap-1 text-[10px] whitespace-nowrap',
                          room.isActive && (room.participantsCount ?? 0) > 0
                            ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-500'
                            : room.isActive
                              ? 'border-emerald-500/20 bg-emerald-500/5 text-emerald-600'
                              : 'border-border bg-muted text-muted-foreground',
                        )}
                      >
                        {(room.participantsCount ?? 0) > 0 && (
                          <span className="relative flex h-1.5 w-1.5">
                            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                            <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500" />
                          </span>
                        )}
                        {room.isActive ? 'Live' : 'Suspended'}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      className={cn(
                        'gap-1 text-[10px]',
                        room.isPublic
                          ? 'border-teal-500/30 bg-teal-500/10 text-teal-500'
                          : 'border-violet-500/30 bg-violet-500/10 text-violet-500',
                      )}
                    >
                      {room.isPublic ? <Globe className="h-3 w-3" /> : <Lock className="h-3 w-3" />}
                      {room.isPublic ? 'Pub' : 'Priv'}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs">
                    <div className="flex items-center gap-1">
                      <span>{room.participantsCount ?? 0}</span>
                      {(room.participantsCount ?? 0) > 0 && (
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell text-xs">
                    {room.ownerName ? (
                      <button
                        type="button"
                        onClick={() => onOwnerClick?.(room.createdBy)}
                        className="truncate hover:text-primary transition-colors text-left max-w-[120px]"
                      >
                        {room.ownerName}
                      </button>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="hidden lg:table-cell text-[11px] text-muted-foreground whitespace-nowrap">
                    {new Date(room.createdAt).toLocaleDateString(undefined, {
                      month: 'short',
                      day: 'numeric',
                      year: 'numeric',
                    })}
                  </TableCell>
                  <TableCell className="hidden lg:table-cell text-[11px] text-muted-foreground whitespace-nowrap">
                    {relativeTime(room.lastActivityAt)}
                  </TableCell>
                  <TableCell>
                    <RowActionsDropdown
                      room={room}
                      onView={() => onRoomClick(room.id)}
                      onEditCapacity={() => setEditCapDialog({ id: room.id, value: room.maxParticipants })}
                      onCopyId={() => navigator.clipboard.writeText(room.id)}
                      onSuspend={() => setConfirmAction({ id: room.id, type: 'suspend' })}
                      onUnsuspend={
                        room.isActive ? undefined : () => setConfirmAction({ id: room.id, type: 'unsuspend' })
                      }
                      onClose={() => setConfirmAction({ id: room.id, type: 'close' })}
                      onDelete={() => setConfirmAction({ id: room.id, type: 'delete' })}
                      isReadOnly={isReadOnly}
                    />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Mobile cards */}
      <div className="sm:hidden space-y-1">
        {rooms.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-xs">No rooms found</div>
        ) : (
          rooms.map((room) => (
            <div key={room.id} className="border-b p-3 space-y-1.5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 min-w-0">
                  <Checkbox
                    checked={table.selectedIds.has(room.id)}
                    onCheckedChange={() => table.selectOne(room.id)}
                    aria-label={`Select ${room.name}`}
                  />
                  <span className="font-mono text-xs truncate">{room.name}</span>
                </div>
                <RowActionsDropdown
                  room={room}
                  onView={() => onRoomClick(room.id)}
                  onEditCapacity={() => setEditCapDialog({ id: room.id, value: room.maxParticipants })}
                  onCopyId={() => navigator.clipboard.writeText(room.id)}
                  onSuspend={() => setConfirmAction({ id: room.id, type: 'suspend' })}
                  onUnsuspend={room.isActive ? undefined : () => setConfirmAction({ id: room.id, type: 'unsuspend' })}
                  onClose={() => setConfirmAction({ id: room.id, type: 'close' })}
                  onDelete={() => setConfirmAction({ id: room.id, type: 'delete' })}
                  isReadOnly={isReadOnly}
                />
              </div>
              <div className="flex items-center gap-2 text-[11px] text-muted-foreground flex-wrap">
                <Badge
                  variant="outline"
                  className={cn(
                    'gap-1 text-[10px]',
                    room.isActive && (room.participantsCount ?? 0) > 0
                      ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-500'
                      : room.isActive
                        ? 'border-emerald-500/20 bg-emerald-500/5 text-emerald-600'
                        : 'border-border bg-muted text-muted-foreground',
                  )}
                >
                  {(room.participantsCount ?? 0) > 0 && (
                    <span className="relative flex h-1.5 w-1.5">
                      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                      <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500" />
                    </span>
                  )}
                  {room.isActive ? 'Live' : 'Suspended'}
                </Badge>
                <Badge
                  variant="outline"
                  className={cn(
                    'text-[10px]',
                    room.isPublic
                      ? 'border-teal-500/30 bg-teal-500/10 text-teal-500'
                      : 'border-violet-500/30 bg-violet-500/10 text-violet-500',
                  )}
                >
                  {room.isPublic ? 'Public' : 'Private'}
                </Badge>
                {room.ownerName && <span className="text-xs">{room.ownerName}</span>}
                <span>·</span>
                <span>{room.participantsCount ?? 0} users</span>
                <span>·</span>
                <span>{new Date(room.createdAt).toLocaleDateString()}</span>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Confirm action dialog */}
      <AlertConfirmDialog
        open={confirmAction !== null}
        onOpenChange={(open) => !open && setConfirmAction(null)}
        title={confirmAction ? `${confirmLabels[confirmAction.type] ?? 'Confirm'} room` : ''}
        description={confirmAction ? (confirmDescriptions[confirmAction.type] ?? '') : ''}
        confirmLabel={confirmAction ? (confirmLabels[confirmAction.type] ?? 'Confirm') : ''}
        onConfirm={handleConfirm}
        variant={confirmAction?.type === 'close' || confirmAction?.type === 'delete' ? 'destructive' : 'default'}
        isLoading={suspendPending || deletePending || closePending}
      />

      {/* Edit capacity dialog */}
      <AlertConfirmDialog
        open={editCapDialog !== null}
        onOpenChange={(open) => !open && setEditCapDialog(null)}
        title="Edit capacity"
        description={
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">Set maximum participants for this room.</p>
            <Input
              type="number"
              min={1}
              value={editCapDialog?.value ?? 0}
              onChange={(e) => setEditCapDialog((prev) => (prev ? { ...prev, value: +e.target.value } : null))}
              className="w-full"
              autoFocus
            />
          </div>
        }
        confirmLabel="Save"
        onConfirm={() => {
          if (editCapDialog && editCapDialog.value > 0) {
            onUpdateLimit(editCapDialog.id, editCapDialog.value)
          }
          setEditCapDialog(null)
        }}
        variant="default"
      />
    </>
  )
}

// Re-export for backward compat
export type { AdminRoom as AdminRoomType } from '@/components/admin/RowActionsDropdown'
