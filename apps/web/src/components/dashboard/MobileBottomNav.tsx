import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { Plus, Settings, User, Video } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { useUserStore } from '#/lib/user.store'
import { CreateRoomDialog, type CreateRoomData } from '@/components/dashboard/CreateRoomDialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { getErrorMessage } from '@/lib/errors'

export function MobileBottomNav() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const addRecent = useRecentRoomsStore((s) => s.add)
  const user = useUserStore((s) => s.user)
  const { location } = useRouterState()
  const [createOpen, setCreateOpen] = useState(false)

  const path = location.pathname
  const onRooms = path === '/dashboard' || path === '/dashboard/'
  const onProfile = path === '/profile' || path.startsWith('/profile/')
  const onSettings = path === '/settings' || path.startsWith('/settings/')
  const showFab = onRooms

  async function handleCreate(data: CreateRoomData) {
    try {
      const res = await api.post<{ name: string }>('/api/room/create', data)
      setCreateOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['rooms'] })
      addRecent(res.name)
      navigate({ to: '/m/$meetId', params: { meetId: res.name } })
    } catch (err) {
      toast.error(getErrorMessage(err, 'Failed to create room'))
    }
  }

  const items = [
    { id: 'rooms', label: 'Rooms', icon: Video, active: onRooms, to: '/dashboard' as const },
    { id: 'profile', label: 'Profile', icon: User, active: onProfile, to: '/profile' as const },
    { id: 'settings', label: 'Settings', icon: Settings, active: onSettings, to: '/settings' as const },
  ] as const

  return (
    <>
      {showFab ? (
        <Button
          type="button"
          size="icon"
          onClick={() => setCreateOpen(true)}
          className="meet-avatar-circle fixed end-5 z-40 h-14 w-14 bg-primary text-primary-foreground shadow-lg hover:bg-primary/90 lg:hidden bottom-[calc(4.75rem+env(safe-area-inset-bottom,0px))]"
          aria-label="New room"
        >
          <Plus className="h-6 w-6" />
        </Button>
      ) : null}

      <nav className="fixed inset-x-0 bottom-0 z-[60] border-t border-border bg-background/95 pb-[max(0.5rem,env(safe-area-inset-bottom,0px))] backdrop-blur-sm lg:hidden">
        <div className="flex h-16 items-center justify-around px-2">
          {items.map(({ id, label, icon: Icon, active, to }) => (
            <button
              key={id}
              type="button"
              onClick={() => navigate({ to })}
              className={cn(
                'flex min-w-0 flex-1 flex-col items-center gap-1 border-none bg-transparent px-1 py-1',
                active ? 'text-primary' : 'text-muted-foreground',
              )}
            >
              <span
                className={cn(
                  'meet-avatar-circle flex h-8 w-14 items-center justify-center transition-colors',
                  active ? 'bg-primary/20 text-primary' : 'bg-transparent',
                )}
              >
                <Icon className="h-5 w-5" strokeWidth={active ? 2.25 : 1.75} />
              </span>
              <span className={cn('text-[11px] font-medium leading-none', active && 'text-primary')}>{label}</span>
            </button>
          ))}
        </div>
      </nav>

      <CreateRoomDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreate={handleCreate}
        isAdmin={user?.isAdmin}
      />
    </>
  )
}
