// TODO oncoming feature
import { createFileRoute, Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import {
  Activity,
  LayoutDashboard,
  LogOut,
  Radio,
  Settings,
  Shield,
  Users,
  Video,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import type { User as BedrudUser } from '#/lib/user.store'
import { useUserStore } from '#/lib/user.store'
import { loadRegisteredUser, requireRegisteredUser } from '#/lib/require-registered-user'
import { ThemeToggle } from '@/components/ThemeToggle'
import { MobileBottomNav } from '@/components/dashboard/MobileBottomNav'
import { HomeSettingsDialog } from '@/components/settings/HomeSettingsDialog'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/dashboard')({
  beforeLoad: requireRegisteredUser,
  loader: loadRegisteredUser,
  staleTime: Infinity,
  component: DashboardLayout,
})

const USER_NAV = [
  { to: '/dashboard' as const, label: 'Rooms', icon: LayoutDashboard, exact: true },
  { to: '/dashboard/settings' as const, label: 'Settings', icon: Settings },
]

const ADMIN_NAV = [
  { to: '/dashboard/admin' as const, label: 'Overview', icon: Shield, exact: true },
  { to: '/dashboard/admin/queue' as const, label: 'Queue', icon: Activity },
  // TODO oncoming feature
  // { to: '/dashboard/admin/recordings' as const, label: 'Recordings', icon: Radio },
  { to: '/dashboard/admin/users' as const, label: 'Users', icon: Users },
  { to: '/dashboard/admin/rooms' as const, label: 'Rooms', icon: Video },
  // "System" not "Settings" — personal audio/Krisp lives under Main → Settings
  { to: '/dashboard/admin/settings' as const, label: 'System', icon: Settings },
]

function NavLink({
  to,
  label,
  icon: Icon,
  exact,
  onClick,
}: {
  to: string
  label: string
  icon: React.ElementType
  exact?: boolean
  onClick?: () => void
}) {
  const { location } = useRouterState()
  const active = exact ? location.pathname === to || location.pathname === to + '/' : location.pathname.startsWith(to)
  return (
    <Link to={to} onClick={onClick}>
      <div
        className={cn(
          'flex items-center gap-2 px-2 py-1.5 text-xs font-medium transition-colors',
          active ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
        )}
      >
        <Icon className="h-3.5 w-3.5 shrink-0" />
        {label}
      </div>
    </Link>
  )
}

function SidebarContent({
  user,
  onLogout,
  onNavClick,
}: {
  user: BedrudUser | null
  onLogout: () => void
  onNavClick?: () => void
}) {
  const initials = user?.name
    ? user.name
        .split(' ')
        .map((n) => n[0])
        .join('')
        .toUpperCase()
        .slice(0, 2)
    : '?'

  return (
    <>
      <nav className="flex flex-1 flex-col gap-px overflow-y-auto p-2">
        <p className="mb-1 px-2 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/40">Main</p>
        {USER_NAV.map((item) => (
          <NavLink key={item.to} {...item} onClick={onNavClick} />
        ))}

        {user?.isAdmin && (
          <div className="mt-3">
            <div className="mb-1 flex items-center gap-2 px-2">
              <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/40">Admin</p>
              <span className="rounded border border-destructive/30 bg-destructive/10 px-1 py-px text-[9px] font-semibold uppercase text-destructive">
                Restricted
              </span>
            </div>
            {ADMIN_NAV.map((item) => (
              <NavLink key={item.to} {...item} onClick={onNavClick} />
            ))}
          </div>
        )}
      </nav>

      <div className="shrink-0 border-t p-2">
        <div className="group flex items-center gap-2 px-2 py-1.5 transition-colors hover:bg-accent">
          <Avatar className="h-6 w-6 shrink-0">
            {user?.avatarUrl && <AvatarImage src={user.avatarUrl} alt={user.name} />}
            <AvatarFallback className="bg-primary text-[9px] font-semibold text-primary-foreground">
              {initials}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium">{user?.name ?? '…'}</p>
            <p className="truncate text-[10px] text-muted-foreground">{user?.email ?? ''}</p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            type="button"
            onClick={onLogout}
            className="h-9 w-9 rounded p-1.5 text-muted-foreground opacity-60 transition-all hover:bg-destructive/10 hover:text-destructive hover:opacity-100"
            aria-label="Sign out"
          >
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </>
  )
}

function Sidebar({ user, onLogout }: { user: BedrudUser | null; onLogout: () => void }) {
  return (
    <aside className="hidden lg:flex fixed inset-y-0 start-0 z-50 w-52 flex-col border-e bg-card">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b px-4">
        <div className="flex h-6 w-6 items-center justify-center bg-primary">
          <Radio className="h-3 w-3 text-primary-foreground" />
        </div>
        <span className="font-mono text-xs font-semibold tracking-tight">bedrud</span>
      </div>
      <SidebarContent user={user} onLogout={onLogout} />
    </aside>
  )
}

function TopBar({ user }: { user: BedrudUser | null }) {
  const [settingsOpen, setSettingsOpen] = useState(false)
  const initials = user?.name
    ? user.name
        .split(' ')
        .map((n) => n[0])
        .join('')
        .toUpperCase()
        .slice(0, 2)
    : '?'

  return (
    <>
      <header className="sticky top-0 z-40 hidden h-11 items-center gap-2 border-b bg-background/90 px-4 backdrop-blur-sm lg:flex lg:ps-52">
        <div className="ms-auto flex items-center gap-1.5">
          <ThemeToggle />
          <Button
            variant="ghost"
            size="icon"
            type="button"
            onClick={() => setSettingsOpen(true)}
            className="h-auto w-auto rounded-full p-0"
            aria-label="Open settings"
          >
            <Avatar className="h-6 w-6">
              {user?.avatarUrl && <AvatarImage src={user.avatarUrl} alt={user.name} />}
              <AvatarFallback className="bg-primary text-[9px] font-semibold text-primary-foreground">
                {initials}
              </AvatarFallback>
            </Avatar>
          </Button>
        </div>
      </header>
      <HomeSettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} includeSecurity />
    </>
  )
}

function DashboardLayout() {
  const navigate = useNavigate()
  const user = useUserStore((s) => s.user)
  const clearAuth = useAuthStore((s) => s.clear)
  const clearUser = useUserStore((s) => s.clear)

  // SSR hydration fallback: beforeLoad/loader skip on the server.
  // This effect ensures auth init happens on the client — user data
  // is handled by the route loader to avoid duplicate /api/auth/me calls.
  useEffect(() => {
    if (user) return // already loaded
    let cancelled = false
    ;(async () => {
      await useAuthStore.getState().initialize()
      if (cancelled) return
      if (!useAuthStore.getState().tokens) {
        navigate({ to: '/auth' })
      }
      // user data fetch is handled by the route loader — no duplicate call here
    })()
    return () => {
      cancelled = true
    }
  }, [user, navigate])

  async function handleLogout() {
    try {
      const refreshToken = useAuthStore.getState().tokens?.refreshToken
      if (refreshToken) await api.post('/api/auth/logout', { refresh_token: refreshToken })
    } catch {
      /* ignore */
    } finally {
      clearAuth()
      clearUser()
      useRecentRoomsStore.getState().clear()
      navigate({ to: '/auth' })
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <Sidebar user={user} onLogout={handleLogout} />
      <TopBar user={user} />
      <MobileBottomNav />
      <main
        id="main-content"
        className="p-4 pb-[calc(8rem+env(safe-area-inset-bottom,0px))] lg:ps-52 lg:p-6 lg:pb-6"
      >
        <p className="mb-6 pt-3 font-mono text-3xl font-semibold leading-tight tracking-tight lg:hidden">bedrud</p>
        <Outlet />
      </main>
    </div>
  )
}
