import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Search,
  Shield,
  ShieldOff,
  User,
  UserCheck,
  UserX,
} from 'lucide-react'
import { useState } from 'react'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '#/components/ui/select'
import { api } from '#/lib/api'
import { cn } from '@/lib/utils'

interface AdminUser {
  id: string
  email: string
  name: string
  provider: string
  isActive: boolean
  accesses: string[] | null
  createdAt: string
}

export const Route = createFileRoute('/dashboard/admin/users')({ component: AdminUsersPage })

function ProviderBadge({ provider }: { provider: string }) {
  return (
    <span className="rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
      {provider}
    </span>
  )
}

function StatusToggle({ user, onToggle, isPending }: { user: AdminUser; onToggle: () => void; isPending: boolean }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={isPending}
      className={cn(
        'flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold transition-opacity hover:opacity-80 disabled:opacity-50',
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
    </button>
  )
}

type SortField = 'name' | 'email' | 'provider' | 'createdAt'

function AdminUsersPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState<SortField>('createdAt')
  const [sortAsc, setSortAsc] = useState(false)
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(50)

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'users', page, limit],
    queryFn: () => api.get<{ users: AdminUser[]; total: number }>(`/api/admin/users?page=${page}&limit=${limit}`),
  })

  const toggleStatus = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      api.put(`/api/admin/users/${id}/status`, { active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'], exact: false }),
  })

  const toggleAdmin = useMutation({
    mutationFn: ({ id, accesses }: { id: string; accesses: string[] }) =>
      api.put(`/api/admin/users/${id}/accesses`, { accesses }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'], exact: false }),
  })

  function toggleSort(field: SortField) {
    if (sortField === field) setSortAsc((v) => !v)
    else {
      setSortField(field)
      setSortAsc(true)
    }
  }

  const users = (data?.users ?? [])
    .filter((u) => {
      const q = search.toLowerCase()
      return !q || u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q)
    })
    .sort((a, b) => {
      let cmp = 0
      if (sortField === 'name') cmp = a.name.localeCompare(b.name)
      else if (sortField === 'email') cmp = a.email.localeCompare(b.email)
      else if (sortField === 'provider') cmp = a.provider.localeCompare(b.provider)
      else cmp = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()
      return sortAsc ? cmp : -cmp
    })

  function SortIcon({ field }: { field: SortField }) {
    if (sortField !== field) return null
    return sortAsc ? <ChevronUp className="h-3 w-3 inline ml-0.5" /> : <ChevronDown className="h-3 w-3 inline ml-0.5" />
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 px-4">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-sm font-semibold">Users</h1>
          <p className="text-xs text-muted-foreground">{data?.total ?? 0} registered accounts</p>
        </div>

        <div className="flex items-center gap-2 border bg-background px-3 py-2.5 w-full sm:w-56 focus-within:ring-2 focus-within:ring-ring">
          <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or email…"
            className="flex-1 bg-transparent text-xs outline-none placeholder:text-muted-foreground"
          />
        </div>
      </div>

      {/* Table */}
      <div className="border overflow-hidden">
        <div className="overflow-x-auto">
          <div className="min-w-[560px]">
            <div className="grid grid-cols-[1fr_1fr_auto_auto_auto_auto] gap-4 border-b bg-muted/30 px-4 py-3.5 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
              <button
                type="button"
                className="text-left hover:text-foreground transition-colors"
                onClick={() => toggleSort('name')}
                aria-label={`Sort by name${sortField === 'name' ? (sortAsc ? ' ascending' : ' descending') : ''}`}
              >
                Name <SortIcon field="name" />
              </button>
              <button
                type="button"
                className="text-left hover:text-foreground transition-colors"
                onClick={() => toggleSort('email')}
                aria-label={`Sort by email${sortField === 'email' ? (sortAsc ? ' ascending' : ' descending') : ''}`}
              >
                Email <SortIcon field="email" />
              </button>
              <button
                type="button"
                className="hidden sm:block text-left hover:text-foreground transition-colors"
                onClick={() => toggleSort('provider')}
                aria-label={`Sort by provider${sortField === 'provider' ? (sortAsc ? ' ascending' : ' descending') : ''}`}
              >
                Provider <SortIcon field="provider" />
              </button>
              <span>Status</span>
              <span>Role</span>
              <button
                type="button"
                className="hidden sm:block text-left hover:text-foreground transition-colors"
                onClick={() => toggleSort('createdAt')}
                aria-label={`Sort by joined date${sortField === 'createdAt' ? (sortAsc ? ' ascending' : ' descending') : ''}`}
              >
                Joined <SortIcon field="createdAt" />
              </button>
            </div>

            <div className="divide-y">
              {isLoading ? (
                [...Array(5)].map((_, i) => (
                  <div key={i} className="grid grid-cols-[1fr_1fr_auto_auto_auto_auto] gap-4 px-4 py-4 animate-pulse">
                    <div className="h-3.5 bg-muted" />
                    <div className="h-3.5 bg-muted" />
                    <div className="h-5 w-16 bg-muted" />
                    <div className="h-5 w-16 bg-muted" />
                    <div className="h-5 w-8 bg-muted" />
                    <div className="h-3.5 w-20 bg-muted" />
                  </div>
                ))
              ) : users.length === 0 ? (
                <p className="px-4 py-8 text-center text-xs text-muted-foreground">No users found</p>
              ) : (
                users.map((user) => {
                  const isSuperadmin = user.accesses?.includes('superadmin')
                  return (
                    <div
                      key={user.id}
                      className="grid grid-cols-[1fr_1fr_auto_auto_auto_auto] items-center gap-4 px-4 py-4 hover:bg-muted/30 transition-colors"
                    >
                      <div className="min-w-0">
                        <Link
                          to="/dashboard/admin/users/$userId"
                          params={{ userId: user.id }}
                          className="truncate text-xs font-medium hover:text-primary transition-colors"
                        >
                          {user.name}
                        </Link>
                      </div>
                      <p
                        className="truncate text-xs text-muted-foreground"
                        title={user.provider === 'guest' ? user.email : undefined}
                      >
                        {user.provider === 'guest' ? (
                          <span className="inline-flex items-center gap-1.5">
                            <User className="h-3 w-3 shrink-0 opacity-50" />
                            <span>Guest</span>
                          </span>
                        ) : (
                          user.email
                        )}
                      </p>
                      <div className="hidden sm:block">
                        <ProviderBadge provider={user.provider} />
                      </div>
                      <StatusToggle
                        user={user}
                        isPending={toggleStatus.isPending}
                        onToggle={() => toggleStatus.mutate({ id: user.id, active: !user.isActive })}
                      />
                      <button
                        type="button"
                        onClick={() =>
                          toggleAdmin.mutate({
                            id: user.id,
                            accesses: isSuperadmin ? ['user'] : ['superadmin', 'user'],
                          })
                        }
                        disabled={toggleAdmin.isPending}
                        title={isSuperadmin ? 'Remove admin' : 'Make admin'}
                        className={cn(
                          'flex items-center justify-center h-6 w-6 border transition-opacity hover:opacity-80 disabled:opacity-50',
                          isSuperadmin
                            ? 'border-primary/30 bg-primary/10 text-primary'
                            : 'border-border bg-muted text-muted-foreground',
                        )}
                      >
                        {isSuperadmin ? <Shield className="h-3 w-3" /> : <ShieldOff className="h-3 w-3" />}
                      </button>
                      <p className="hidden sm:block text-[11px] text-muted-foreground whitespace-nowrap">
                        {new Date(user.createdAt).toLocaleDateString(undefined, {
                          month: 'short',
                          day: 'numeric',
                          year: 'numeric',
                        })}
                      </p>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between border-t bg-muted/30 px-4 py-3.5">
          <p className="text-[11px] text-muted-foreground">
            Page {page} of {Math.max(1, Math.ceil((data?.total ?? 0) / limit))}
          </p>

          <div className="flex items-center gap-2">
            <Select
              value={String(limit)}
              onValueChange={(v) => {
                setLimit(+v)
                setPage(1)
              }}
            >
              <SelectTrigger className="h-8 w-[72px] text-[11px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="25">25</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>

            <button
              type="button"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              className="inline-flex items-center justify-center h-8 w-8 border transition-opacity hover:opacity-80 disabled:opacity-30 disabled:cursor-not-allowed focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Previous page"
            >
              <ChevronLeft className="h-3.5 w-3.5" />
            </button>

            <button
              type="button"
              disabled={page * limit >= (data?.total ?? 0)}
              onClick={() => setPage((p) => p + 1)}
              className="inline-flex items-center justify-center h-8 w-8 border transition-opacity hover:opacity-80 disabled:opacity-30 disabled:cursor-not-allowed focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Next page"
            >
              <ChevronRight className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
