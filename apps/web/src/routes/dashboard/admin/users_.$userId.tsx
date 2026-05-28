import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import {
  Activity,
  ArrowLeft,
  Calendar,
  Clock,
  Globe,
  Hash,
  Lock,
  LogOut,
  Mail,
  RefreshCw,
  Shield,
  Trash2,
  UserCheck,
  Users,
  UserX,
  Video,
} from 'lucide-react'
import type { CSSProperties } from 'react'
import { useEffect, useState } from 'react'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis } from 'recharts'
import { toast } from 'sonner'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '#/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '#/components/ui/tabs'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { useUserStore } from '#/lib/user.store'
import { useAdminContext } from '#/routes/dashboard/admin.tsx'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
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
      return { borderColor: '#f59e0b30', background: '#f59e0b15', color: '#fbbf24' }
    case 'guest':
      return { borderColor: '#a855f730', background: '#a855f715', color: '#c084fc' }
    default:
      return {}
  }
}

export const Route = createFileRoute('/dashboard/admin/users_/$userId')({ component: UserDetailPage })

interface UserDetail {
  id: string
  email: string
  name: string
  provider: string
  isActive: boolean
  accesses: string[] | null
  createdAt: string
}

interface Room {
  id: string
  name: string
  isPublic: boolean
  isActive: boolean
  maxParticipants: number
  createdAt: string
}

const PROVIDER_STYLE: Record<string, { bg: string; color: string }> = {
  local: { bg: 'color-mix(in oklab, var(--primary) 8%, transparent)', color: 'var(--accent-400)' },
  google: { bg: '#ef444415', color: '#f87171' },
  github: { bg: '#71717a15', color: '#a1a1aa' },
  guest: { bg: '#f59e0b15', color: '#fbbf24' },
  passkey: { bg: '#10b98115', color: '#34d399' },
}

function ProviderBadge({ provider }: { provider: string }) {
  const s = PROVIDER_STYLE[provider] ?? {
    bg: 'color-mix(in oklab, var(--primary) 8%, transparent)',
    color: 'var(--accent-400)',
  }
  return (
    <Badge
      variant="outline"
      className="text-xs font-semibold uppercase tracking-wider px-2.5 py-1"
      style={{ background: s.bg, color: s.color, borderColor: s.color + '30' }}
    >
      {provider}
    </Badge>
  )
}

/** Last 8 weeks, one bar per week */
function buildWeeklyChart(rooms: Room[]) {
  const weeks = Array.from({ length: 8 }, (_, i) => {
    const d = new Date()
    d.setDate(d.getDate() - (7 - i) * 7)
    return {
      label: d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
      start: new Date(d.getFullYear(), d.getMonth(), d.getDate() - d.getDay()),
      rooms: 0,
    }
  })
  for (const room of rooms) {
    const created = new Date(room.createdAt)
    for (let i = weeks.length - 1; i >= 0; i--) {
      if (created >= weeks[i].start) {
        weeks[i].rooms++
        break
      }
    }
  }
  return weeks.map(({ label, rooms }) => ({ label, rooms }))
}

function StatCard({
  value,
  label,
  icon: Icon,
  color,
}: {
  value: number | string
  label: string
  icon: React.ElementType
  color: string
}) {
  return (
    <div className="border p-4" style={{ borderColor: `${color}25`, background: `${color}07` }}>
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-xl font-bold tracking-tight" style={{ color }}>
            {value}
          </p>
          <p className="text-xs text-muted-foreground mt-0.5">{label}</p>
        </div>
        <Icon className="h-5 w-5 shrink-0 opacity-70" style={{ color }} />
      </div>
    </div>
  )
}

function daysSince(dateStr: string) {
  return Math.floor((Date.now() - new Date(dateStr).getTime()) / 86_400_000)
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
}

function UserDetailPage() {
  const { userId } = Route.useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const currentUser = useUserStore((s) => s.user)
  const { isReadOnly } = useAdminContext()
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [confirmEmail, setConfirmEmail] = useState('')
  const [forceLogoutDialogOpen, setForceLogoutDialogOpen] = useState(false)

  useEffect(() => {
    if (!deleteDialogOpen && !forceLogoutDialogOpen) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setDeleteDialogOpen(false)
        setForceLogoutDialogOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [deleteDialogOpen, forceLogoutDialogOpen])

  interface RoomSession {
    id: string
    roomId: string
    roomName: string
    joinedAt: string
    leftAt: string | null
    isActive: boolean
    durationSeconds: number
  }

  const [sessionPage, setSessionPage] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'user', userId],
    queryFn: () => api.get<{ user: UserDetail; rooms: Room[] }>(`/api/admin/users/${userId}`),
  })

  const sessionsQuery = useQuery({
    queryKey: ['admin', 'user', userId, 'sessions', sessionPage],
    queryFn: () =>
      api.get<{ sessions: RoomSession[]; total: number; page: number; limit: number }>(
        `/api/admin/users/${userId}/sessions?page=${sessionPage}&limit=20`,
      ),
  })

  const toggleStatus = useMutation({
    mutationFn: (active: boolean) => api.put(`/api/admin/users/${userId}/status`, { active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'user', userId] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update user status')),
  })

  const [pendingRole, setPendingRole] = useState<string | null>(null)

  const changeRole = useMutation({
    mutationFn: (accesses: string[]) => api.put(`/api/admin/users/${userId}/accesses`, { accesses }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'user', userId] })
      setPendingRole(null)
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to update user role'))
      setPendingRole(null)
    },
  })

  const deleteUser = useMutation({
    mutationFn: () => api.delete(`/api/admin/users/${userId}`),
    onSuccess: () => {
      toast.success('User deletion queued — will complete shortly')
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }), 5000)
      navigate({ to: '/dashboard/admin/users' })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to queue user deletion')),
  })

  const forceLogout = useMutation({
    mutationFn: () => api.post(`/api/admin/users/${userId}/force-logout`),
    onSuccess: () => {
      toast.success('All sessions revoked')
      setForceLogoutDialogOpen(false)
    },
    onError: () => {
      toast.error('Failed to revoke sessions')
    },
  })

  const user = data?.user
  const rooms = data?.rooms ?? []
  const currentRole = user ? detectRole(user.accesses) : 'user'

  const activeRooms = rooms.filter((r) => r.isActive).length
  const publicRooms = rooms.filter((r) => r.isPublic).length
  const weeklyData = user ? buildWeeklyChart(rooms) : []

  return (
    <div className="mx-auto max-w-4xl space-y-6 px-4">
      {/* Back + header */}
      <div className="flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={() => navigate({ to: '/dashboard/admin/users' })}
          aria-label="Go back to users"
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold tracking-tight truncate">{user?.name ?? userId}</h1>
          <p className="text-xs text-muted-foreground mt-0.5">User detail</p>
        </div>
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={() => queryClient.invalidateQueries({ queryKey: ['admin', 'user', userId] })}
          aria-label="Refresh user data"
        >
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-28" />
          ))}
        </div>
      ) : !user ? (
        <div className="border px-5 py-16 text-center" style={{ borderColor: 'var(--border)' }}>
          <p className="text-sm text-muted-foreground">User not found</p>
        </div>
      ) : (
        <>
          {/* ── Hero card ──────────────────────────────────── */}
          <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
            {/* gradient banner */}
            <div
              className="h-20 w-full"
              style={{
                background: 'linear-gradient(135deg, var(--primary) 0%, var(--accent-700) 60%, var(--primary) 100%)',
              }}
            />

            <div className="px-5 pb-5">
              {/* Avatar overlapping banner */}
              <div className="flex items-end justify-between -mt-9 mb-3">
                <div
                  className="flex h-16 w-16 items-center justify-center text-2xl font-bold text-white ring-4"
                  style={{
                    background: 'linear-gradient(135deg, var(--primary), var(--accent-700))',
                    boxShadow: '0 0 0 4px var(--background)',
                  }}
                >
                  {(user.name || user.email).charAt(0).toUpperCase()}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2 pb-1">
                  {!isReadOnly ? (
                    <>
                      <Select
                        value={pendingRole ?? currentRole}
                        onValueChange={(role) => setPendingRole(role)}
                        disabled={changeRole.isPending}
                      >
                        <SelectTrigger className="h-8 w-[130px] text-xs">
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
                      {pendingRole && (
                        <div className="flex items-center gap-1">
                          <Button
                            type="button"
                            size="sm"
                            onClick={() => changeRole.mutate(ROLE_ACCESS_MAP[pendingRole])}
                            disabled={changeRole.isPending}
                            className="h-7 px-2 text-[11px]"
                          >
                            {changeRole.isPending ? 'Saving…' : 'Confirm'}
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => setPendingRole(null)}
                            disabled={changeRole.isPending}
                            className="h-7 px-2 text-[11px]"
                          >
                            ×
                          </Button>
                        </div>
                      )}
                    </>
                  ) : (
                    <span
                      className="flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-semibold capitalize"
                      style={getRoleBadgeStyle(currentRole)}
                    >
                      {currentRole}
                    </span>
                  )}
                  {!isReadOnly && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => toggleStatus.mutate(!user.isActive)}
                      disabled={toggleStatus.isPending}
                      style={
                        user.isActive
                          ? { background: '#10b98115', color: '#10b981', borderColor: '#10b98130' }
                          : { background: '#ef444415', color: '#f87171', borderColor: '#ef444430' }
                      }
                    >
                      {user.isActive ? (
                        <>
                          <UserCheck className="h-3.5 w-3.5" />
                          Active
                        </>
                      ) : (
                        <>
                          <UserX className="h-3.5 w-3.5" />
                          Banned
                        </>
                      )}
                    </Button>
                  )}
                  {!isReadOnly && currentUser?.id !== user.id && (
                    <>
                      <Button
                        variant="outline"
                        size="sm"
                        type="button"
                        onClick={() => setForceLogoutDialogOpen(true)}
                        style={{
                          background: '#f59e0b15',
                          color: '#fbbf24',
                          borderColor: '#f59e0b30',
                        }}
                        aria-label="Force logout"
                      >
                        <LogOut className="h-3.5 w-3.5" />
                        Logout
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        type="button"
                        onClick={() => {
                          setConfirmEmail('')
                          setDeleteDialogOpen(true)
                        }}
                        style={{
                          background: '#ef444415',
                          color: '#f87171',
                          borderColor: '#ef444430',
                        }}
                        aria-label="Delete user"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        Delete
                      </Button>
                    </>
                  )}
                </div>
              </div>

              {/* Name + badges */}
              <p className="text-xl font-bold">{user.name || '—'}</p>
              <p className="text-sm text-muted-foreground mt-0.5">{user.email}</p>
              <div className="flex flex-wrap items-center gap-2 mt-2">
                <ProviderBadge provider={user.provider} />
                {(user.accesses ?? []).map((access) => (
                  <Badge
                    key={access}
                    variant="outline"
                    className="text-[10px] font-semibold capitalize px-2 py-0.5"
                    style={getRoleBadgeStyle(access)}
                  >
                    {access}
                  </Badge>
                ))}
                {!user.isActive && (
                  <Badge
                    variant="outline"
                    className="text-[10px] font-semibold px-2 py-0.5"
                    style={{ background: '#ef444415', color: '#f87171', borderColor: '#ef444430' }}
                  >
                    banned
                  </Badge>
                )}
              </div>

              {/* Detail grid */}
              <div className="mt-4 grid grid-cols-2 gap-4 border-t pt-4" style={{ borderColor: 'var(--border)' }}>
                {[
                  { icon: Hash, label: 'User ID', value: user.id, mono: true },
                  {
                    icon: Calendar,
                    label: 'Joined',
                    value: new Date(user.createdAt).toLocaleDateString(undefined, {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                    }),
                  },
                  { icon: Mail, label: 'Email', value: user.email },
                  { icon: Shield, label: 'Accesses', value: (user.accesses ?? []).join(', ') || 'none' },
                ].map(({ icon: Icon, label, value, mono }) => (
                  <div
                    key={label}
                    className="flex items-start gap-2.5 p-4"
                    style={{ background: 'color-mix(in oklab, var(--muted) 40%, transparent)' }}
                  >
                    <Icon className="h-3.5 w-3.5 mt-0.5 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <p className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</p>
                      <p className={cn('mt-0.5 truncate text-sm', mono ? 'font-mono text-[11px]' : 'font-medium')}>
                        {value}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* ── Stats row ──────────────────────────────────── */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <StatCard value={rooms.length} label="Rooms created" icon={Video} color="var(--primary)" />
            <StatCard value={activeRooms} label="Currently live" icon={Activity} color="#10b981" />
            <StatCard value={publicRooms} label="Public rooms" icon={Globe} color="var(--primary)" />
            <StatCard value={`${daysSince(user.createdAt)}d`} label="Member for" icon={Users} color="#f59e0b" />
          </div>

          {/* ── Tabs: Rooms / Room Sessions ──────────────── */}
          <Tabs defaultValue="rooms">
            <TabsList>
              <TabsTrigger value="rooms" className="text-xs">
                Rooms
              </TabsTrigger>
              <TabsTrigger value="sessions" className="text-xs">
                Room Sessions
              </TabsTrigger>
            </TabsList>

            <TabsContent value="rooms" className="mt-4 space-y-6">
              {/* ── Room activity chart ──────────────────── */}
              <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
                <div
                  className="flex items-center justify-between border-b px-5 py-3"
                  style={{
                    background:
                      'linear-gradient(135deg, color-mix(in oklab, var(--primary) 3%, transparent), color-mix(in oklab, var(--accent-700) 3%, transparent))',
                  }}
                >
                  <p className="text-sm font-semibold">Room creation activity</p>
                  <span className="text-xs text-muted-foreground">last 8 weeks</span>
                </div>
                <div className="p-5">
                  {rooms.length === 0 ? (
                    <div className="flex h-24 items-center justify-center">
                      <p className="text-xs text-muted-foreground">No rooms created yet</p>
                    </div>
                  ) : (
                    <ResponsiveContainer width="100%" height={100}>
                      <AreaChart data={weeklyData} margin={{ top: 4, right: 4, left: -10, bottom: 0 }}>
                        <defs>
                          <linearGradient id="uGrad" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.3} />
                            <stop offset="95%" stopColor="var(--primary)" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                        <XAxis
                          dataKey="label"
                          tick={{ fontSize: 10, fill: 'var(--muted-foreground)' }}
                          axisLine={false}
                          tickLine={false}
                        />
                        <Tooltip
                          // eslint-disable-next-line @typescript-eslint/no-explicit-any
                          formatter={(v: any) => [`${v} room${v !== 1 ? 's' : ''}`, 'Created']}
                          contentStyle={{
                            background: 'var(--card)',
                            border: '1px solid var(--border)',
                            borderRadius: '8px',
                            fontSize: '12px',
                          }}
                        />
                        <Area
                          type="monotone"
                          dataKey="rooms"
                          stroke="var(--primary)"
                          fill="url(#uGrad)"
                          strokeWidth={2}
                          dot={{ r: 3, fill: 'var(--primary)' }}
                        />
                      </AreaChart>
                    </ResponsiveContainer>
                  )}
                </div>
              </div>

              {/* ── Rooms table ──────────────────────────── */}
              <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
                <div
                  className="flex items-center justify-between border-b px-5 py-3"
                  style={{
                    background:
                      'linear-gradient(135deg, color-mix(in oklab, var(--accent-700) 3%, transparent), color-mix(in oklab, var(--primary) 3%, transparent))',
                  }}
                >
                  <p className="text-sm font-semibold">Rooms</p>
                  <span className="text-xs text-muted-foreground">{rooms.length} total</span>
                </div>

                {rooms.length === 0 ? (
                  <div className="flex flex-col items-center gap-2 py-10">
                    <Video className="h-7 w-7 text-muted-foreground opacity-40" />
                    <p className="text-sm text-muted-foreground">No rooms created by this user</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <div className="min-w-[460px]">
                      {/* header row */}
                      <div
                        className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 border-b px-5 py-3 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground"
                        style={{ borderColor: 'var(--border)' }}
                      >
                        <span>Name</span>
                        <span className="hidden sm:block">Visibility</span>
                        <span>Status</span>
                        <span>Cap.</span>
                        <span className="hidden sm:block">Created</span>
                      </div>
                      <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
                        {rooms.map((room) => (
                          <div
                            key={room.id}
                            className="grid grid-cols-[1fr_auto_auto_auto_auto] items-center gap-4 px-5 py-3.5 hover:bg-muted/20 transition-colors"
                          >
                            <Link
                              to="/dashboard/admin/rooms/$roomId"
                              params={{ roomId: room.id }}
                              className="text-sm font-mono font-medium hover:text-indigo-400 transition-colors truncate"
                            >
                              {room.name}
                            </Link>
                            <span
                              className="hidden sm:flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold"
                              style={
                                room.isPublic
                                  ? {
                                      background: 'color-mix(in oklab, var(--primary) 8%, transparent)',
                                      color: 'var(--primary)',
                                    }
                                  : {
                                      background: 'color-mix(in oklab, var(--accent-700) 8%, transparent)',
                                      color: 'var(--accent-400)',
                                    }
                              }
                            >
                              {room.isPublic ? <Globe className="h-3 w-3" /> : <Lock className="h-3 w-3" />}
                              {room.isPublic ? 'Public' : 'Private'}
                            </span>
                            <span
                              className="flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold"
                              style={
                                room.isActive
                                  ? { background: '#10b98115', color: '#10b981' }
                                  : { background: 'var(--muted)', color: 'var(--muted-foreground)' }
                              }
                            >
                              {room.isActive && <Activity className="h-2.5 w-2.5 animate-pulse" />}
                              {room.isActive ? 'Live' : 'Idle'}
                            </span>
                            <span className="text-xs text-muted-foreground text-center">{room.maxParticipants}</span>
                            <p className="hidden sm:block text-xs text-muted-foreground whitespace-nowrap">
                              {new Date(room.createdAt).toLocaleDateString(undefined, {
                                month: 'short',
                                day: 'numeric',
                                year: 'numeric',
                              })}
                            </p>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </TabsContent>

            <TabsContent value="sessions" className="mt-4">
              <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
                <div
                  className="flex items-center justify-between border-b px-5 py-3"
                  style={{
                    background:
                      'linear-gradient(135deg, color-mix(in oklab, var(--accent-700) 3%, transparent), color-mix(in oklab, var(--primary) 3%, transparent))',
                  }}
                >
                  <p className="text-sm font-semibold">Room Sessions</p>
                  <span className="text-xs text-muted-foreground">{sessionsQuery.data?.total ?? '—'} total</span>
                </div>

                {sessionsQuery.isLoading ? (
                  <div className="flex flex-col items-center gap-2 py-10">
                    <div className="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                    <p className="text-sm text-muted-foreground">Loading sessions...</p>
                  </div>
                ) : sessionsQuery.isError ? (
                  <div className="flex flex-col items-center gap-3 py-10">
                    <div
                      className="flex items-center gap-2 border px-3 py-2 text-sm"
                      style={{
                        borderColor: '#ef444430',
                        background: '#ef444415',
                        color: '#f87171',
                      }}
                    >
                      {sessionsQuery.error instanceof Error ? sessionsQuery.error.message : 'Failed to load sessions'}
                    </div>
                    <Button variant="outline" size="sm" onClick={() => sessionsQuery.refetch()}>
                      <RefreshCw className="mr-1.5 h-3 w-3" />
                      Retry
                    </Button>
                  </div>
                ) : !sessionsQuery.data?.sessions?.length ? (
                  <div className="flex flex-col items-center gap-2 py-10">
                    <Clock className="h-7 w-7 text-muted-foreground opacity-40" />
                    <p className="text-sm text-muted-foreground">No room sessions yet</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <div className="min-w-[500px]">
                      {/* header row */}
                      <div
                        className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 border-b px-5 py-3 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground"
                        style={{ borderColor: 'var(--border)' }}
                      >
                        <span>Room</span>
                        <span>Joined</span>
                        <span>Left</span>
                        <span>Duration</span>
                        <span>Status</span>
                      </div>
                      <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
                        {sessionsQuery.data.sessions.map((s) => {
                          const joined = new Date(s.joinedAt)
                          const left = s.leftAt ? new Date(s.leftAt) : null
                          return (
                            <div
                              key={s.id}
                              className="grid grid-cols-[1fr_auto_auto_auto_auto] items-center gap-4 px-5 py-3.5 hover:bg-muted/20 transition-colors"
                            >
                              <Link
                                to="/dashboard/admin/rooms/$roomId"
                                params={{ roomId: s.roomId }}
                                className="text-sm font-mono font-medium hover:text-indigo-400 transition-colors truncate"
                              >
                                {s.roomName || s.roomId.slice(0, 8)}
                              </Link>
                              <span
                                className="text-xs text-muted-foreground whitespace-nowrap"
                                title={joined.toLocaleString()}
                              >
                                {joined.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                              </span>
                              <span
                                className="text-xs text-muted-foreground whitespace-nowrap"
                                title={left?.toLocaleString() ?? ''}
                              >
                                {left ? left.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) : '—'}
                              </span>
                              <span className="text-xs text-muted-foreground whitespace-nowrap">
                                {s.isActive ? (
                                  <span className="flex items-center gap-1 text-emerald-400">
                                    <Activity className="h-2.5 w-2.5 animate-pulse" />
                                    {formatDuration(s.durationSeconds)}
                                  </span>
                                ) : (
                                  formatDuration(s.durationSeconds)
                                )}
                              </span>
                              <span>
                                {s.isActive ? (
                                  <span
                                    className="rounded-full px-2 py-0.5 text-[10px] font-semibold"
                                    style={{ background: '#10b98115', color: '#10b981' }}
                                  >
                                    Active
                                  </span>
                                ) : (
                                  <span
                                    className="rounded-full px-2 py-0.5 text-[10px] font-semibold"
                                    style={{ background: 'var(--muted)', color: 'var(--muted-foreground)' }}
                                  >
                                    Ended
                                  </span>
                                )}
                              </span>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  </div>
                )}

                {/* Pagination */}
                {sessionsQuery.data && sessionsQuery.data.total > sessionsQuery.data.limit && (
                  <div
                    className="flex items-center justify-between border-t px-5 py-3"
                    style={{ borderColor: 'var(--border)' }}
                  >
                    <p className="text-[10px] text-muted-foreground">
                      Page {sessionsQuery.data.page} of {Math.ceil(sessionsQuery.data.total / sessionsQuery.data.limit)}
                    </p>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={sessionPage <= 1}
                        onClick={() => {
                          setSessionPage((p) => Math.max(1, p - 1))
                        }}
                      >
                        Previous
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={sessionPage >= Math.ceil(sessionsQuery.data.total / sessionsQuery.data.limit)}
                        onClick={() => {
                          setSessionPage((p) => p + 1)
                        }}
                      >
                        Next
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            </TabsContent>
          </Tabs>
        </>
      )}

      <p className="text-center text-xs text-muted-foreground">
        <Link to="/dashboard/admin/users" className="hover:text-foreground underline-offset-4 hover:underline">
          ← Back to all users
        </Link>
      </p>

      {/* Force logout confirmation dialog */}
      <Dialog open={forceLogoutDialogOpen} onOpenChange={(open) => !open && setForceLogoutDialogOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Force logout</DialogTitle>
            <DialogDescription>Revoke all active sessions for this user</DialogDescription>
          </DialogHeader>

          <div className="p-3 text-sm bg-muted">
            <p className="font-medium text-foreground">{user?.name || '—'}</p>
            <p className="text-xs mt-0.5 text-muted-foreground">{user?.email}</p>
          </div>

          <p className="text-sm text-muted-foreground">
            This will revoke the user's refresh token, forcing them to log in again on all devices. Their current access
            token will remain valid until it expires.
          </p>

          {forceLogout.isError && (
            <div
              className="flex items-center gap-2 border px-3 py-2 text-sm"
              style={{
                borderColor: '#ef444430',
                background: '#ef444415',
                color: '#f87171',
              }}
            >
              {forceLogout.error instanceof Error ? forceLogout.error.message : 'Failed to revoke sessions'}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setForceLogoutDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => forceLogout.mutate()}
              disabled={forceLogout.isPending}
              style={{ background: '#f59e0b' }}
            >
              {forceLogout.isPending ? 'Revoking...' : 'Revoke sessions'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={(open) => !open && setDeleteDialogOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete user</DialogTitle>
            <DialogDescription>This action is permanent and cannot be undone</DialogDescription>
          </DialogHeader>

          <div className="p-3 text-sm bg-muted">
            <p className="font-medium text-foreground">{user?.name || '—'}</p>
            <p className="text-xs mt-0.5 text-muted-foreground">{user?.email}</p>
          </div>

          <p className="text-sm text-muted-foreground">
            All data associated with this user will be permanently removed including room participation, permissions,
            and refresh tokens.
          </p>

          <div className="space-y-1.5">
            <Label className="text-sm font-medium">
              Type <span className="font-mono">{user?.email}</span> to confirm
            </Label>
            <Input
              id="confirm-email"
              type="text"
              value={confirmEmail}
              onChange={(e) => setConfirmEmail(e.target.value)}
              placeholder={user?.email}
            />
          </div>

          {deleteUser.isError && (
            <div
              className="flex items-center gap-2 border px-3 py-2 text-sm"
              style={{
                borderColor: '#ef444430',
                background: '#ef444415',
                color: '#f87171',
              }}
            >
              {deleteUser.error instanceof Error ? deleteUser.error.message : 'Failed to delete user'}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteUser.mutate()}
              disabled={confirmEmail !== user?.email || deleteUser.isPending}
            >
              {deleteUser.isPending ? 'Deleting...' : 'Delete permanently'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
