// TODO oncoming feature
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowRight, Plus, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { mergeDashboardRooms, serverRoomsOnly } from '#/lib/dashboard-room-list'
import { parseJoinInput } from '#/lib/join-input'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { useUserStore } from '#/lib/user.store'
import { CreateRoomDialog } from '@/components/dashboard/CreateRoomDialog'
import { FilterChip } from '@/components/dashboard/FilterChip'
import { RoomCard } from '@/components/dashboard/RoomCard'
import { RoomSettingsDialog } from '@/components/dashboard/RoomSettingsDialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { getErrorMessage } from '@/lib/errors'

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

export const Route = createFileRoute('/dashboard/')({
  component: DashboardPage,
  head: () => ({ meta: [{ title: 'Dashboard — Bedrud' }] }),
  validateSearch: (search: Record<string, string>): { emailVerified?: string } => {
    if (search.emailVerified) return { emailVerified: search.emailVerified }
    return {}
  },
})

// ── Quick Join Bar ───────────────────────────────────────────────────────────

function QuickJoinBar({ onJoin, onCreate }: { onJoin: (name: string) => void; onCreate: () => void }) {
  const [value, setValue] = useState('')

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const roomName = parseJoinInput(value)
    if (!roomName) {
      toast.error('That does not look like a room name or a meeting link')
      return
    }
    onJoin(roomName)
  }

  return (
    <div className="flex items-center gap-2">
      <form
        onSubmit={handleSubmit}
        className="flex h-9 flex-1 items-center gap-2 rounded-lg border border-input bg-background px-3 focus-within:ring-2 focus-within:ring-ring"
      >
        <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        {/* Room names are lowercase and have no spaces, so the phone keyboard should not
            capitalise, correct or spell-check what is typed. This mirrors the Android field's
            `KeyboardType.Uri` with capitalisation and auto-correct off. */}
        <Input
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="Join by room name or invite link..."
          inputMode="url"
          autoComplete="off"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          className="h-full flex-1 border-none focus-visible:ring-0 px-0"
        />
        {/* Disabled rather than hidden while the field is empty: a button that appears as you type
            shifts the row under your thumb. Android disables it for the same reason. */}
        <Button type="submit" size="sm" className="gap-1" disabled={!value.trim()}>
          Join <ArrowRight className="h-3 w-3" />
        </Button>
      </form>
      {/* Desktop only: phones create a room from the floating button in the bottom navigation,
          which is where the Android client puts it too. */}
      <Button type="button" variant="default" size="sm" onClick={onCreate} className="max-lg:hidden">
        <Plus className="h-3.5 w-3.5" />
        New room
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

  const { data: rooms, isLoading } = useQuery({
    queryKey: ['rooms'],
    queryFn: () => api.get<Room[]>('/api/room/list'),
    refetchOnMount: 'always',
  })

  const { emailVerified } = Route.useSearch()

  useEffect(() => {
    if (emailVerified === 'true') {
      toast.success('Email verified successfully')
      navigate({ to: '/dashboard', search: {}, replace: true })
    }
  }, [emailVerified, navigate])

  const [createOpen, setCreateOpen] = useState(false)
  const [settingsRoom, setSettingsRoom] = useState<Room | null>(null)
  const [activeFilter, setActiveFilter] = useState<'all' | 'mine'>('all')
  const [query, setQuery] = useState('')

  function handleJoin(roomName: string) {
    addRecent(roomName)
    navigate({ to: '/m/$meetId', params: { meetId: roomName } })
  }

  const deleteRoom = useMutation({
    mutationFn: (roomId: string) => api.delete(`/api/room/${roomId}`),
    onMutate: async (roomId) => {
      await queryClient.cancelQueries({ queryKey: ['rooms'] })
      const prev = queryClient.getQueryData<Room[]>(['rooms'])
      queryClient.setQueryData<Room[]>(['rooms'], (old) => old?.filter((r) => r.id !== roomId))
      return { prev }
    },
    onSuccess: () => {
      toast.success('Room deleted')
      void queryClient.invalidateQueries({ queryKey: ['rooms'] })
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
  const entries = (
    activeFilter === 'all' ? mergeDashboardRooms(rooms ?? [], recentRooms) : serverRoomsOnly(rooms ?? [], recentRooms)
  ).filter((entry) => !normalizedQuery || entry.name.toLowerCase().includes(normalizedQuery))

  const firstName = user?.name?.split(' ')[0]

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="hidden lg:block">
        <h1 className="text-lg font-semibold tracking-tight">{firstName ? `${firstName}'s rooms` : 'Rooms'}</h1>
        <p className="text-sm text-muted-foreground">Create, join, or manage your meeting rooms.</p>
      </div>

      <QuickJoinBar onJoin={handleJoin} onCreate={() => setCreateOpen(true)} />

      {/* Tabs + Search */}
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <FilterChip label="All" selected={activeFilter === 'all'} onSelect={() => setActiveFilter('all')} />
          <FilterChip label="My Rooms" selected={activeFilter === 'mine'} onSelect={() => setActiveFilter('mine')} />
        </div>

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
        {isLoading ? (
          <div className="p-2">
            <SkeletonRows />
          </div>
        ) : entries.length > 0 ? (
          <div className="grid grid-cols-1 gap-2 p-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {entries.map((entry) => (
              <RoomCard
                key={entry.key}
                entry={entry}
                onJoin={() => handleJoin(entry.name)}
                onDelete={entry.kind === 'server' ? () => deleteRoom.mutate(entry.room.id) : undefined}
                onSettings={entry.kind === 'server' ? () => setSettingsRoom(entry.room) : undefined}
                onRemove={entry.kind === 'recent' ? () => removeRecent(entry.name) : undefined}
              />
            ))}
          </div>
        ) : (
          <div className="px-4 py-12 text-center">
            {/* Keyed on the query rather than on whether any room exists anywhere. The My Rooms chip
                can empty the list while recents still exist, and reporting that against an empty
                query reads as `No rooms match ""`. */}
            {normalizedQuery ? (
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
                <Button type="button" variant="default" size="sm" onClick={() => setCreateOpen(true)} className="mt-3">
                  <Plus className="h-3.5 w-3.5" />
                  New room
                </Button>
              </>
            )}
          </div>
        )}
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
