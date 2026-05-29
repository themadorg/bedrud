// TODO oncoming feature
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { Archive, ArrowRight, Check, Clock, Copy, Globe, Lock, Plus, Search, Settings2, Trash2, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { type RecentRoom, useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { useUserStore } from '#/lib/user.store'
import { CreateRoomDialog } from '@/components/dashboard/CreateRoomDialog'
import { RoomSettingsDialog } from '@/components/dashboard/RoomSettingsDialog'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getErrorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

interface Room {
  id: string
  name: string
  isPublic: boolean
  maxParticipants: number
  isActive: boolean
  mode: string
  settings: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee: boolean
  }
}

interface ArchivedRoom {
  id: string
  name: string
  createdAt: string
  deletedAt: string
  // TODO oncoming feature
  recordingCount: number
}

export const Route = createFileRoute('/dashboard/')({
  component: DashboardPage,
  head: () => ({ meta: [{ title: 'Dashboard — Bedrud' }] }),
  validateSearch: (search: Record<string, string>): { emailVerified?: string } => {
    if (search.emailVerified) return { emailVerified: search.emailVerified }
    return {}
  },
})

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeAgo(ts: number): string {
  const diff = Date.now() - ts
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

// ── Quick Join Bar ───────────────────────────────────────────────────────────

function QuickJoinBar({ onJoin, onCreate }: { onJoin: (name: string) => void; onCreate: () => void }) {
  const [value, setValue] = useState('')

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const slug = value.trim().toLowerCase().replace(/\s+/g, '-')
    if (!slug) return
    onJoin(slug)
  }

  return (
    <div className="flex items-center gap-2">
      <form
        onSubmit={handleSubmit}
        className="flex h-9 flex-1 items-center gap-2 rounded-lg border border-input bg-background px-3 focus-within:ring-2 focus-within:ring-ring"
      >
        <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="Join by room name or invite code..."
          autoComplete="off"
          spellCheck={false}
          className="h-full flex-1 border-none focus-visible:ring-0 px-0"
        />
        {value.trim() && (
          <Button type="submit" size="sm" className="gap-1">
            Join <ArrowRight className="h-3 w-3" />
          </Button>
        )}
      </form>
      <Button type="button" variant="default" size="sm" onClick={onCreate}>
        <Plus className="h-3.5 w-3.5" />
        <span className="hidden sm:inline">New room</span>
      </Button>
    </div>
  )
}

// ── Room Row ─────────────────────────────────────────────────────────────────

function RoomRow({
  room,
  onJoin,
  onDelete,
  onSettings,
  isDeleting,
}: {
  room: Room
  onJoin: () => void
  onDelete: () => void
  onSettings: () => void
  isDeleting?: boolean
}) {
  const [copied, setCopied] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  function copyLink() {
    void navigator.clipboard.writeText(`${window.location.origin}/m/${room.name}`)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  if (confirmDelete) {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2">
        <p className="text-sm text-destructive">
          Delete <span className="font-mono font-medium">{room.name}</span>?
        </p>
        <div className="flex items-center gap-1.5">
          <Button variant="ghost" size="sm" type="button" onClick={() => setConfirmDelete(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            type="button"
            disabled={isDeleting}
            onClick={() => {
              onDelete()
              setConfirmDelete(false)
            }}
          >
            {isDeleting ? 'Deleting…' : 'Delete'}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="group flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-accent/50">
      {/* Status dot + name */}
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <span
          className={cn('h-2 w-2 shrink-0 rounded-full', room.isActive ? 'bg-emerald-500' : 'bg-muted-foreground/20')}
          title={room.isActive ? 'Live' : 'Inactive'}
        />
        <button
          type="button"
          onClick={onJoin}
          className="min-w-0 truncate font-mono text-sm font-medium hover:underline"
        >
          {room.name}
        </button>
      </div>

      {/* Badges */}
      <div className="hidden items-center gap-1.5 sm:flex">
        <Badge
          variant="outline"
          className={cn(
            'gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-medium',
            room.isPublic
              ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400'
              : 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
          )}
        >
          {room.isPublic ? <Globe className="h-3 w-3" /> : <Lock className="h-3 w-3" />}
          {room.isPublic ? 'Public' : 'Private'}
        </Badge>
        {room.isActive && (
          <Badge
            variant="outline"
            className="gap-1 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400"
          >
            Live
          </Badge>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-0.5 opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100">
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={copyLink}
          className="h-7 w-7"
          title={copied ? 'Copied!' : 'Copy link'}
        >
          {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={onSettings}
          className="h-7 w-7"
          aria-label="Room settings"
        >
          <Settings2 className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={() => setConfirmDelete(true)}
          className="h-7 w-7 hover:bg-destructive/10 hover:text-destructive"
          title="Delete"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>

      {/* Join button */}
      <Button
        variant={room.isActive ? 'default' : 'outline'}
        size="sm"
        type="button"
        onClick={onJoin}
        className="h-7 gap-1 px-2.5 text-xs"
      >
        {room.isActive ? 'Join' : 'Open'}
        <ArrowRight className="h-3 w-3" />
      </Button>
    </div>
  )
}

// ── Archived Room Row ────────────────────────────────────────────────────────

function ArchivedRoomRow({ room }: { room: ArchivedRoom }) {
  const navigate = useNavigate()
  const createdAt = room.createdAt ? new Date(room.createdAt).toLocaleDateString() : '—'
  const deletedAt = room.deletedAt ? new Date(room.deletedAt).toLocaleDateString() : '—'

  return (
    <button
      type="button"
      onClick={() => navigate({ to: '/dashboard/archived/$roomId', params: { roomId: room.id } })}
      className="group flex w-full items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-accent/50 text-left"
    >
      <Archive className="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />
      <div className="min-w-0 flex-1">
        <span className="font-mono text-sm font-medium">{room.name}</span>
        <div className="flex items-center gap-3 text-xs text-muted-foreground/60">
          <span>Created {createdAt}</span>
          <span>Ended {deletedAt}</span>
        </div>
      </div>
      {/* TODO oncoming feature — recording count removed */}
    </button>
  )
}

// ── Recent Room Row ──────────────────────────────────────────────────────────

function RecentRoomRow({ recent, onJoin, onRemove }: { recent: RecentRoom; onJoin: () => void; onRemove: () => void }) {
  return (
    <div className="group flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-accent/50">
      <Clock className="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />
      <button
        type="button"
        onClick={onJoin}
        className="min-w-0 flex-1 truncate text-left font-mono text-sm font-medium hover:underline"
      >
        {recent.name}
      </button>
      <span className="text-xs text-muted-foreground/50">{timeAgo(recent.joinedAt)}</span>
      <div className="flex items-center gap-0.5 opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100">
        <Button
          variant="ghost"
          size="icon"
          type="button"
          onClick={onRemove}
          className="h-7 w-7 hover:bg-destructive/10 hover:text-destructive"
          aria-label="Remove from recent rooms"
        >
          <X className="h-3.5 w-3.5" />
        </Button>
      </div>
      <Button variant="outline" size="sm" type="button" onClick={onJoin} className="h-7 gap-1 px-2.5 text-xs">
        Join <ArrowRight className="h-3 w-3" />
      </Button>
    </div>
  )
}

// ── Skeleton ─────────────────────────────────────────────────────────────────

function SkeletonRows() {
  return (
    <div className="space-y-1">
      {[1, 2, 3].map((i) => (
        <div key={i} className="flex items-center gap-3 rounded-lg px-3 py-2.5">
          <Skeleton className="h-2 w-2 rounded-full" />
          <Skeleton className="h-4 w-32" />
          <Skeleton className="ml-auto h-4 w-16" />
        </div>
      ))}
    </div>
  )
}

// ── Main Page ────────────────────────────────────────────────────────────────

function DashboardPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const user = useUserStore((s) => s.user)
  const recentRooms = useRecentRoomsStore((s) => s.rooms)
  const addRecent = useRecentRoomsStore((s) => s.add)
  const removeRecent = useRecentRoomsStore((s) => s.remove)

  const { data: archivedData } = useQuery({
    queryKey: ['archived-rooms'],
    queryFn: () => api.get<{ rooms: ArchivedRoom[]; total: number }>('/api/room/archived'),
    staleTime: 30_000,
  })
  const archivedRooms = archivedData?.rooms ?? []

  const { emailVerified } = Route.useSearch()

  useEffect(() => {
    if (emailVerified === 'true') {
      toast.success('Email verified successfully')
      navigate({ to: '/dashboard', search: {}, replace: true })
    }
  }, [emailVerified, navigate])

  const [createOpen, setCreateOpen] = useState(false)
  const [settingsRoom, setSettingsRoom] = useState<Room | null>(null)
  const [tab, setTab] = useState<'rooms' | 'recent' | 'archived'>('rooms')
  const [query, setQuery] = useState('')

  const { data: rooms, isLoading } = useQuery({
    queryKey: ['rooms'],
    queryFn: () => api.get<Room[]>('/api/room/list'),
    refetchOnMount: 'always',
  })

  function handleJoin(roomName: string) {
    addRecent(roomName)
    navigate({ to: '/m/$meetId', params: { meetId: roomName } })
  }

  const deleteRoom = useMutation({
    mutationFn: (roomId: string) => api.delete(`/api/room/${roomId}`),
    onMutate: async (roomId) => {
      await queryClient.cancelQueries({ queryKey: ['rooms'] })
      const prev = queryClient.getQueryData<Room[]>(['rooms'])
      queryClient.setQueryData<Room[]>(['rooms'], (old) =>
        old?.map((r) => (r.id === roomId ? { ...r, isActive: false } : r)),
      )
      return { prev }
    },
    onSuccess: () => {
      toast.success('Room deletion queued — will complete shortly')
      setTimeout(() => queryClient.invalidateQueries({ queryKey: ['rooms'] }), 3000)
    },
    onError: (err, _roomId, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(['rooms'], ctx.prev)
      toast.error(getErrorMessage(err, 'Failed to delete room'))
    },
  })

  async function handleUpdateSettings(
    roomId: string,
    data: { isPublic: boolean; maxParticipants: number; settings: Room['settings'] },
  ) {
    await api.put(`/api/room/${roomId}/settings`, data)
    void queryClient.invalidateQueries({ queryKey: ['rooms'] })
  }

  async function handleCreate(data: {
    name?: string
    isPublic: boolean
    maxParticipants: number
    settings: Room['settings']
  }) {
    const res = await api.post<Room>('/api/room/create', data)
    setCreateOpen(false)
    void queryClient.invalidateQueries({ queryKey: ['rooms'] })
    addRecent(res.name)
    navigate({ to: '/m/$meetId', params: { meetId: res.name } })
  }

  const normalizedQuery = query.trim().toLowerCase()
  const filtered = (rooms ?? [])
    .filter((r) => !normalizedQuery || r.name.toLowerCase().includes(normalizedQuery))
    .sort((a, b) => {
      if (a.isActive !== b.isActive) return Number(b.isActive) - Number(a.isActive)
      return a.name.localeCompare(b.name)
    })

  const filteredRecent = recentRooms
    .filter((r, i, arr) => arr.findIndex((x) => x.name === r.name) === i)
    .filter((r) => !normalizedQuery || r.name.toLowerCase().includes(normalizedQuery))

  const firstName = user?.name?.split(' ')[0]

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      {/* Header */}
      <div>
        <h1 className="text-lg font-semibold tracking-tight">{firstName ? `${firstName}'s rooms` : 'Rooms'}</h1>
        <p className="text-sm text-muted-foreground">Create, join, or manage your meeting rooms.</p>
      </div>

      {/* Quick Join + New Room */}
      <QuickJoinBar onJoin={handleJoin} onCreate={() => setCreateOpen(true)} />

      {/* Tabs + Search */}
      <div className="flex items-center justify-between gap-3">
        <Tabs value={tab} onValueChange={(v) => setTab(v as 'rooms' | 'recent' | 'archived')}>
          <TabsList>
            <TabsTrigger value="rooms" className="text-sm gap-1.5">
              My Rooms
              {rooms && <span className="text-xs text-muted-foreground">{rooms.length}</span>}
            </TabsTrigger>
            <TabsTrigger value="recent" className="text-sm gap-1.5">
              Recent
              {recentRooms.length > 0 && <span className="text-xs text-muted-foreground">{recentRooms.length}</span>}
            </TabsTrigger>
            <TabsTrigger value="archived" className="text-sm gap-1.5">
              <Archive className="h-3.5 w-3.5" />
              Archived
              {archivedRooms.length > 0 && (
                <span className="text-xs text-muted-foreground">{archivedRooms.length}</span>
              )}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <div className="flex h-8 w-full max-w-48 items-center gap-2 rounded-lg border border-input bg-background px-2 focus-within:ring-2 focus-within:ring-ring">
          <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter..."
            className="h-full flex-1 px-0 text-xs border-none focus-visible:border-none focus-visible:ring-0"
          />
        </div>
      </div>

      {/* Content */}
      <div className="rounded-xl border bg-card/50">
        {tab === 'rooms' &&
          (isLoading ? (
            <div className="p-2">
              <SkeletonRows />
            </div>
          ) : filtered.length > 0 ? (
            <div className="divide-y divide-border/50 p-1">
              {filtered.map((room) => (
                <RoomRow
                  key={room.id}
                  room={room}
                  onJoin={() => handleJoin(room.name)}
                  onDelete={() => deleteRoom.mutate(room.id)}
                  onSettings={() => setSettingsRoom(room)}
                  isDeleting={deleteRoom.isPending}
                />
              ))}
            </div>
          ) : (
            <div className="px-4 py-12 text-center">
              {(rooms?.length ?? 0) > 0 ? (
                <>
                  <p className="text-sm font-medium">No rooms match "{query}"</p>
                  <Button variant="link" type="button" onClick={() => setQuery('')} className="mt-2 text-sm">
                    Clear filter
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-sm font-medium">No rooms yet</p>
                  <p className="mt-1 text-xs text-muted-foreground">Create your first room to get started.</p>
                  <Button
                    type="button"
                    variant="default"
                    size="sm"
                    onClick={() => setCreateOpen(true)}
                    className="mt-3"
                  >
                    <Plus className="h-3.5 w-3.5" />
                    New room
                  </Button>
                </>
              )}
            </div>
          ))}

        {tab === 'recent' &&
          (filteredRecent.length > 0 ? (
            <div className="divide-y divide-border/50 p-1">
              {filteredRecent.map((recent) => (
                <RecentRoomRow
                  key={recent.name}
                  recent={recent}
                  onJoin={() => handleJoin(recent.name)}
                  onRemove={() => removeRecent(recent.name)}
                />
              ))}
            </div>
          ) : (
            <div className="px-4 py-12 text-center">
              <Clock className="mx-auto h-5 w-5 text-muted-foreground/30" />
              <p className="mt-2 text-sm font-medium">No recent rooms</p>
              <p className="mt-1 text-xs text-muted-foreground">Rooms you join will appear here for quick access.</p>
            </div>
          ))}

        {tab === 'archived' &&
          (archivedRooms.length > 0 ? (
            <div className="divide-y divide-border/50 p-1">
              {archivedRooms.map((room) => (
                <ArchivedRoomRow key={room.id} room={room} />
              ))}
            </div>
          ) : (
            <div className="px-4 py-12 text-center">
              <Archive className="mx-auto h-5 w-5 text-muted-foreground/30" />
              <p className="mt-2 text-sm font-medium">No archived rooms</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Rooms you end will appear here with their recordings.
              </p>
            </div>
          ))}
      </div>

      <CreateRoomDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreate={handleCreate}
        isAdmin={user?.isAdmin}
      />
      {settingsRoom && (
        <RoomSettingsDialog
          room={settingsRoom}
          open={!!settingsRoom}
          onOpenChange={(open) => {
            if (!open) setSettingsRoom(null)
          }}
          onSave={handleUpdateSettings}
        />
      )}
    </div>
  )
}
