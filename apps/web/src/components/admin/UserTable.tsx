import { Link } from '@tanstack/react-router'
import { Copy, Shield, Trash2, User, UserCheck, UserX } from 'lucide-react'
import type { CSSProperties } from 'react'
import { useState } from 'react'

import { AlertConfirmDialog } from '@/components/admin/AlertConfirmDialog'
import type { useTableState } from '@/components/admin/useTableState'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'

const ROLE_OPTIONS = [
  { value: 'superadmin', label: 'Superadmin' },
  { value: 'admin', label: 'Admin' },
  { value: 'moderator', label: 'Moderator' },
  { value: 'user', label: 'User' },
  { value: 'guest', label: 'Guest' },
] as const

const ROLE_ACCESS_MAP: Record<string, string[]> = {
  superadmin: ['superadmin', 'user'],
  admin: ['admin', 'user'],
  moderator: ['moderator', 'user'],
  user: ['user'],
  guest: ['guest'],
}

function detectRole(accesses: string[] | null): string {
  if (!accesses || accesses.length === 0) return 'user'
  if (accesses.includes('superadmin')) return 'superadmin'
  if (accesses.includes('admin')) return 'admin'
  if (accesses.includes('moderator')) return 'moderator'
  if (accesses.includes('guest')) return 'guest'
  return 'user'
}

function getRoleBadgeStyle(access: string): CSSProperties {
  switch (access) {
    case 'superadmin':
      return {
        borderColor: 'color-mix(in oklab, var(--primary) 30%, transparent)',
        background: 'color-mix(in oklab, var(--primary) 8%, transparent)',
        color: 'var(--primary)',
      }
    case 'admin':
      return {
        borderColor: 'color-mix(in oklab, var(--accent-700) 30%, transparent)',
        background: 'color-mix(in oklab, var(--accent-700) 8%, transparent)',
        color: 'var(--accent-400)',
      }
    case 'moderator':
      return { borderColor: 'rgb(245 158 11 / 0.3)', background: 'rgb(245 158 11 / 0.15)', color: '#fbbf24' }
    case 'guest':
      return { borderColor: 'rgb(168 85 247 / 0.3)', background: 'rgb(168 85 247 / 0.15)', color: '#c084fc' }
    default:
      return {}
  }
}

export interface AdminUser {
  id: string
  email: string
  name: string
  provider: string
  isActive: boolean
  accesses: string[] | null
  createdAt: string
}

export type UserSortField = 'name' | 'email' | 'provider' | 'createdAt'

interface UserTableProps {
  users: AdminUser[]
  isLoading: boolean
  table: ReturnType<typeof useTableState<AdminUser>>
  currentUserId?: string | null
  onToggleStatus: (id: string, active: boolean) => void
  statusPending: boolean
  onRoleChange?: (id: string, accesses: string[]) => void
  rolePending?: boolean
  onDeleteUser?: (id: string) => void
  isReadOnly?: boolean
}

export type { AdminUser as AdminUserType }

export function UserTable({
  users,
  isLoading,
  table,
  currentUserId,
  onToggleStatus,
  statusPending,
  onRoleChange,
  rolePending,
  onDeleteUser,
  isReadOnly,
}: UserTableProps) {
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [roleDialog, setRoleDialog] = useState<{ id: string; role: string; accesses: string[] } | null>(null)

  if (isLoading) {
    return (
      <div className="border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox checked={false} disabled />
              </TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead className="hidden sm:table-cell">Provider</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Role</TableHead>
              <TableHead className="hidden sm:table-cell">Joined</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {[1, 2, 3, 4, 5].map((i) => (
              <TableRow key={i}>
                {[1, 2, 3, 4, 5, 6, 7].map((j) => (
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

  // Desktop + Mobile cells shared renderer
  function getKebabMenu(user: AdminUser) {
    const currentRole = detectRole(user.accesses)
    const isSelf = user.id === currentUserId
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7" aria-label="Row actions">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
              <title>Row actions</title>
              <circle cx="8" cy="3" r="1.5" />
              <circle cx="8" cy="8" r="1.5" />
              <circle cx="8" cy="13" r="1.5" />
            </svg>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem onClick={() => navigator.clipboard.writeText(user.id)}>
            <Copy className="me-2 h-3.5 w-3.5" />
            Copy ID
          </DropdownMenuItem>
          {!isReadOnly && !isSelf && (
            <>
              {user.isActive ? (
                <DropdownMenuItem onClick={() => onToggleStatus(user.id, false)} disabled={statusPending}>
                  <UserX className="me-2 h-3.5 w-3.5" />
                  Ban user
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onClick={() => onToggleStatus(user.id, true)} disabled={statusPending}>
                  <UserCheck className="me-2 h-3.5 w-3.5" />
                  Unban user
                </DropdownMenuItem>
              )}
              {onRoleChange && (
                <DropdownMenuItem
                  onClick={() =>
                    setRoleDialog({
                      id: user.id,
                      role: currentRole,
                      accesses: user.accesses ?? [],
                    })
                  }
                >
                  <Shield className="me-2 h-3.5 w-3.5" />
                  Change role
                </DropdownMenuItem>
              )}
            </>
          )}
          {onDeleteUser && !isReadOnly && !isSelf && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => setConfirmDelete(user.id)}
                className="text-destructive focus:text-destructive"
              >
                <Trash2 className="me-2 h-3.5 w-3.5" />
                Delete user
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    )
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
                  Name {table.sortKey === 'name' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead>
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('email')}
                >
                  Email {table.sortKey === 'email' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="hidden sm:table-cell">
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('provider')}
                >
                  Provider {table.sortKey === 'provider' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="text-xs">Status</TableHead>
              <TableHead className="text-xs">Role</TableHead>
              <TableHead className="hidden sm:table-cell">
                <button
                  type="button"
                  className="text-left hover:text-foreground transition-colors font-medium text-xs"
                  onClick={() => table.toggleSort('createdAt')}
                >
                  Joined {table.sortKey === 'createdAt' && (table.sortOrder === 'asc' ? '↑' : '↓')}
                </button>
              </TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground py-8 text-xs">
                  No users found
                </TableCell>
              </TableRow>
            ) : (
              users.map((user) => {
                const isSelf = user.id === currentUserId
                return (
                  <TableRow
                    key={user.id}
                    data-state={table.selectedIds.has(user.id) ? 'selected' : undefined}
                    className={cn(isSelf && 'opacity-50 pointer-events-none')}
                  >
                    <TableCell>
                      <Checkbox
                        checked={table.selectedIds.has(user.id)}
                        onCheckedChange={() => table.selectOne(user.id)}
                        aria-label={`Select ${user.name}`}
                      />
                    </TableCell>
                    <TableCell className="font-medium">
                      <Link
                        to="/dashboard/admin/users/$userId"
                        params={{ userId: user.id }}
                        className="text-xs hover:text-primary transition-colors"
                      >
                        {user.name}
                      </Link>
                      {isSelf && <span className="ml-1.5 text-[10px] text-muted-foreground">(you)</span>}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {user.provider === 'guest' ? (
                        <span className="inline-flex items-center gap-1.5">
                          <User className="h-3 w-3 shrink-0 opacity-50" />
                          <span>Guest</span>
                        </span>
                      ) : (
                        <span className="truncate block max-w-[200px]" title={user.email}>
                          {user.email}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="hidden sm:table-cell">
                      <Badge variant="outline" className="text-[10px] font-semibold uppercase tracking-wider">
                        {user.provider}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={cn(
                          'gap-1 px-2 py-0.5 text-[10px] font-semibold',
                          user.isActive
                            ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'border-destructive/30 bg-destructive/10 text-destructive',
                        )}
                      >
                        {user.isActive ? (
                          <>
                            <UserCheck className="h-3 w-3" /> Active
                          </>
                        ) : (
                          <>
                            <UserX className="h-3 w-3" /> Banned
                          </>
                        )}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(user.accesses ?? []).map((access) => (
                          <Badge
                            key={access}
                            variant="outline"
                            className="text-[10px] font-semibold capitalize"
                            style={getRoleBadgeStyle(access)}
                          >
                            {access}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="hidden sm:table-cell text-[11px] text-muted-foreground whitespace-nowrap">
                      {new Date(user.createdAt).toLocaleDateString(undefined, {
                        month: 'short',
                        day: 'numeric',
                        year: 'numeric',
                      })}
                    </TableCell>
                    <TableCell>{getKebabMenu(user)}</TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Mobile cards */}
      <div className="sm:hidden space-y-1">
        {users.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-xs">No users found</div>
        ) : (
          users.map((user) => {
            const isSelf = user.id === currentUserId
            return (
              <div key={user.id} className={cn('border-b p-3 space-y-1.5', isSelf && 'opacity-50')}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 min-w-0">
                    <Checkbox
                      checked={table.selectedIds.has(user.id)}
                      onCheckedChange={() => table.selectOne(user.id)}
                      aria-label={`Select ${user.name}`}
                    />
                    <Link
                      to="/dashboard/admin/users/$userId"
                      params={{ userId: user.id }}
                      className="font-medium text-xs hover:text-primary transition-colors"
                    >
                      {user.name}
                    </Link>
                    {isSelf && <span className="ml-1 text-[10px] text-muted-foreground">(you)</span>}
                  </div>
                  {getKebabMenu(user)}
                </div>
                <div className="flex items-center gap-2 text-[11px] text-muted-foreground flex-wrap">
                  {user.provider === 'guest' ? (
                    <span className="inline-flex items-center gap-1">
                      <User className="h-3 w-3" /> Guest
                    </span>
                  ) : (
                    <span className="truncate max-w-[150px]">{user.email}</span>
                  )}
                  <span>·</span>
                  <Badge variant="outline" className="text-[10px] uppercase">
                    {user.provider}
                  </Badge>
                  <span>·</span>
                  <Badge
                    variant="outline"
                    className={cn(
                      'px-2 py-0.5 text-[10px] font-semibold',
                      user.isActive
                        ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'border-destructive/30 bg-destructive/10 text-destructive',
                    )}
                  >
                    {user.isActive ? 'Active' : 'Banned'}
                  </Badge>
                  <span>·</span>
                  <span>{new Date(user.createdAt).toLocaleDateString()}</span>
                </div>
                <div className="flex flex-wrap gap-1">
                  {(user.accesses ?? []).map((access) => (
                    <Badge
                      key={access}
                      variant="outline"
                      className="text-[10px] capitalize"
                      style={getRoleBadgeStyle(access)}
                    >
                      {access}
                    </Badge>
                  ))}
                </div>
              </div>
            )
          })
        )}
      </div>

      {/* Role change dialog */}
      {roleDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-background border p-6 max-w-sm w-full mx-4 space-y-4">
            <h2 className="text-sm font-semibold">Change role</h2>
            <p className="text-xs text-muted-foreground">Current: {roleDialog.role}</p>
            <Select
              value={roleDialog.role}
              onValueChange={(role) =>
                setRoleDialog((prev) => (prev ? { ...prev, role, accesses: ROLE_ACCESS_MAP[role] } : prev))
              }
              disabled={rolePending}
            >
              <SelectTrigger className="w-full text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLE_OPTIONS.map(({ value, label }) => (
                  <SelectItem key={value} value={value} className="text-xs">
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => setRoleDialog(null)} disabled={rolePending}>
                Cancel
              </Button>
              <Button
                variant="default"
                size="sm"
                onClick={() => {
                  if (roleDialog && onRoleChange) {
                    onRoleChange(roleDialog.id, roleDialog.accesses)
                  }
                  setRoleDialog(null)
                }}
                disabled={rolePending}
              >
                {rolePending ? 'Saving…' : 'Save'}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirm */}
      <AlertConfirmDialog
        open={confirmDelete !== null}
        onOpenChange={(open) => !open && setConfirmDelete(null)}
        title="Delete user?"
        description="This will end all active calls and permanently remove all associated data. This cannot be undone."
        confirmLabel="Delete user"
        onConfirm={() => {
          if (confirmDelete && onDeleteUser) {
            onDeleteUser(confirmDelete)
          }
          setConfirmDelete(null)
        }}
        variant="destructive"
      />
    </>
  )
}
