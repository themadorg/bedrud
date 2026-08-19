import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { AlertCircle, ArrowRight, Clock, LogOut, Settings, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '#/lib/api'
import { formatJoinRoomError } from '#/lib/errors'
import { useAuthStore } from '#/lib/auth.store'
import { isGuestToken } from '#/lib/jwt-user'
import { type RecentRoom, useRecentRoomsStore } from '#/lib/recent-rooms.store'
import type { User } from '#/lib/user.store'
import { isGuestUser, useUserStore } from '#/lib/user.store'
import { cn } from '#/lib/utils'
import { HomeSettingsDialog } from '@/components/settings/HomeSettingsDialog'
import { BedrudLogo } from '@/components/BedrudLogo'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

async function loadUserIfNeeded() {
  if (typeof window === 'undefined') return
  if (!useAuthStore.getState().tokens) return
  if (useUserStore.getState().user) return
  const u = await api.get<User & { accesses?: string[] }>('/api/auth/me')
  useUserStore.getState().setUser({
    id: u.id,
    email: u.email,
    name: u.name,
    provider: u.provider,
    isSuperAdmin: u.accesses?.includes('superadmin') ?? false,
    isAdmin: (u.accesses?.includes('admin') || u.accesses?.includes('superadmin')) ?? false,
    accesses: u.accesses ?? [],
    avatarUrl: u.avatarUrl,
  })
}

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    if (typeof window === 'undefined') return
    await useAuthStore.getState().initialize()
  },
  loader: loadUserIfNeeded,
  staleTime: Infinity,
  component: HomePage,
})

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

function JoinForm() {
  const navigate = useNavigate()
  const tokens = useAuthStore((s) => s.tokens)
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [checking, setChecking] = useState(false)
  const [host, setHost] = useState('')

  useEffect(() => {
    setHost(window.location.host)
  }, [])

  async function handleJoin(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const slug = code.trim().toLowerCase().replace(/\s+/g, '-')
    if (!slug) return
    setError(null)
    setChecking(true)
    try {
      await api.get<{ exists: boolean; name: string }>(`/api/room/check/${encodeURIComponent(slug)}`)
    } catch (err) {
      setError(formatJoinRoomError(err))
      setChecking(false)
      return
    }

    if (!tokens) {
      try {
        const guestName = `Guest-${Math.random().toString(36).slice(2, 6)}`
        const res = await api.post<AuthResponse>('/api/auth/guest-login', { name: guestName })
        setTokens(res.tokens, 'ephemeral')
        setUser({
          id: res.user.id,
          email: res.user.email,
          name: res.user.name,
          provider: res.user.provider,
          isSuperAdmin: false,
          isAdmin: false,
          accesses: res.user.accesses ?? [],
          avatarUrl: res.user.avatarUrl,
        })
      } catch (err) {
        setError(formatJoinRoomError(err))
        setChecking(false)
        return
      }
    }

    setChecking(false)
    navigate({ to: '/m/$meetId', params: { meetId: slug } })
  }

  return (
    <div className="space-y-2">
      <form
        onSubmit={handleJoin}
        className="group flex items-center gap-0 border-b-2 border-transparent transition-colors focus-within:border-primary"
      >
        <span className="hidden font-mono text-sm text-muted-foreground/30 select-none whitespace-nowrap sm:block">
          {host}/m/
        </span>
        <input
          value={code}
          onChange={(e) => {
            setCode(e.target.value)
            setError(null)
          }}
          placeholder="your-room"
          autoComplete="off"
          spellCheck={false}
          className="join-room-input h-10 flex-1 border-none bg-transparent ps-2 pe-1 font-mono text-sm outline-none placeholder:text-muted-foreground/40 sm:ps-1"
        />
        <Button type="submit" size="sm" disabled={!code.trim() || checking} className="shrink-0 h-7 gap-1">
          {checking ? (
            '…'
          ) : (
            <>
              <span>Join</span> <ArrowRight className="h-3 w-3" />
            </>
          )}
        </Button>
      </form>
      {error && (
        <div className="flex items-center gap-2 border-s-2 border-destructive bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <AlertCircle className="h-3 w-3 shrink-0" />
          {error}
        </div>
      )}
    </div>
  )
}

function HomeHeader({
  onOpenSettings,
}: {
  onOpenSettings: () => void
}) {
  const navigate = useNavigate()
  const tokens = useAuthStore((s) => s.tokens)
  const initialized = useAuthStore((s) => s.initialized)
  const user = useUserStore((s) => s.user)
  const clearAuth = useAuthStore((s) => s.clear)
  const clearUser = useUserStore((s) => s.clear)
  const guest = isGuestUser(user) || isGuestToken(tokens?.accessToken)
  const initials = user?.name
    ? user.name
        .split(' ')
        .map((n) => n[0])
        .join('')
        .toUpperCase()
        .slice(0, 2)
    : '?'

  async function handleLogout() {
    try {
      const refreshToken = useAuthStore.getState().tokens?.refreshToken
      if (refreshToken) await api.post('/api/auth/logout', { refresh_token: refreshToken })
    } catch {
      /* ignore */
    } finally {
      clearAuth()
      clearUser()
      navigate({ to: '/' })
    }
  }

  return (
    <header className="relative z-10 flex items-center justify-between px-6 py-3 sm:px-10">
      <BedrudLogo />
      <div className="flex items-center gap-3">
        <ThemeToggle />
        <Separator orientation="vertical" className="hidden h-3 sm:block" />
        {initialized && tokens ? (
          <>
            <div className="hidden items-center gap-2 sm:flex">
              <Avatar className="h-7 w-7">
                {user?.avatarUrl && <AvatarImage src={user.avatarUrl} alt={user.name} />}
                <AvatarFallback className="bg-primary text-[10px] font-semibold text-primary-foreground">
                  {initials}
                </AvatarFallback>
              </Avatar>
              <span className="max-w-[140px] truncate text-sm text-muted-foreground">{user?.name ?? 'Account'}</span>
            </div>
            {guest ? (
              <>
                <Button type="button" variant="outline" size="sm" className="gap-1.5" onClick={onOpenSettings}>
                  <Settings className="h-3.5 w-3.5" />
                  Settings
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-muted-foreground"
                  onClick={handleLogout}
                  aria-label="Sign out"
                >
                  <LogOut className="h-4 w-4" />
                </Button>
              </>
            ) : (
              <Link
                to="/dashboard"
                className="bg-primary px-3 py-1.5 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary-hover"
              >
                Dashboard
              </Link>
            )}
          </>
        ) : initialized ? (
          <>
            <Link
              to="/auth/login"
              search={{ redirect: undefined }}
              className="hidden text-sm text-muted-foreground transition-colors hover:text-foreground sm:block"
            >
              Sign in
            </Link>
            <Link
              to="/auth/register"
              className="bg-primary px-3 py-1.5 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary-hover"
            >
              Get started
            </Link>
          </>
        ) : null}
      </div>
    </header>
  )
}

function JoinHint({ guest }: { guest: boolean }) {
  const tokens = useAuthStore((s) => s.tokens)
  const user = useUserStore((s) => s.user)

  if (tokens && guest) {
    return (
      <p className="text-xs text-muted-foreground">
        Joined as {user?.name ?? 'guest'} ·{' '}
        <Link
          to="/auth/register"
          className="underline underline-offset-4 transition-colors hover:text-foreground"
        >
          Create an account
        </Link>{' '}
        to host rooms
      </p>
    )
  }

  if (tokens) {
    return (
      <p className="text-xs text-muted-foreground">
        Signed in as {user?.name ?? 'you'} ·{' '}
        <Link to="/dashboard" className="underline underline-offset-4 transition-colors hover:text-foreground">
          Create rooms
        </Link>{' '}
        ·{' '}
        <Link to="/new" className="underline underline-offset-4 transition-colors hover:text-foreground">
          New meeting
        </Link>
      </p>
    )
  }

  return (
    <p className="text-xs text-muted-foreground">
      <Link
        to="/auth/login"
        search={{ redirect: undefined }}
        className="underline underline-offset-4 transition-colors hover:text-foreground"
      >
        Sign in
      </Link>{' '}
      to create rooms &middot;{' '}
      <Link to="/auth" className="underline underline-offset-4 transition-colors hover:text-foreground">
        join as guest
      </Link>
    </p>
  )
}

function RecentMeetings({
  rooms,
  onJoin,
  onRemove,
  className,
}: {
  rooms: RecentRoom[]
  onJoin: (name: string) => void
  onRemove: (name: string) => void
  className?: string
}) {
  if (rooms.length === 0) return null

  return (
    <div className={cn('w-full max-w-md space-y-3', className)}>
      <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Recent meetings</p>
      <ul className="max-h-[min(50vh,360px)] space-y-2 overflow-y-auto">
        {rooms.slice(0, 8).map((recent) => (
          <li key={recent.name} className="group flex items-center justify-between gap-4 border border-border px-3 py-2.5">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <Clock className="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />
              <button
                type="button"
                onClick={() => onJoin(recent.name)}
                className="min-w-0 truncate text-left font-mono text-sm font-medium hover:underline"
              >
                {recent.name}
              </button>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <span className="hidden text-xs text-muted-foreground/50 sm:inline">{timeAgo(recent.joinedAt)}</span>
              <Button
                variant="ghost"
                size="icon"
                type="button"
                onClick={() => onRemove(recent.name)}
                className="h-7 w-7 opacity-100 hover:bg-destructive/10 hover:text-destructive sm:opacity-0 sm:group-hover:opacity-100"
                aria-label={`Remove ${recent.name} from recent`}
              >
                <X className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={() => onJoin(recent.name)}
                className="h-7 gap-1 px-2.5 text-xs"
              >
                Join <ArrowRight className="h-3 w-3" />
              </Button>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

function HomePage() {
  const navigate = useNavigate()
  const tokens = useAuthStore((s) => s.tokens)
  const user = useUserStore((s) => s.user)
  const guest = isGuestUser(user) || isGuestToken(tokens?.accessToken)
  const recentRooms = useRecentRoomsStore((s) => s.rooms)
  const removeRecent = useRecentRoomsStore((s) => s.remove)
  const [settingsOpen, setSettingsOpen] = useState(false)

  useEffect(() => {
    if (!tokens || user) return
    void loadUserIfNeeded()
  }, [tokens, user])

  const showRecent = guest && recentRooms.length > 0

  return (
    <div className="relative flex min-h-screen flex-col overflow-hidden bg-background text-foreground">
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
        <div
          className="absolute right-[8%] top-[18%] h-[420px] w-[420px] rounded-full opacity-[0.12] dark:opacity-[0.06] blur-[100px]"
          style={{ background: 'var(--spotlight-a)' }}
        />
      </div>

      <HomeHeader onOpenSettings={() => setSettingsOpen(true)} />

      <main className="relative z-10 flex flex-1 flex-col px-6 pb-12 pt-20 sm:px-10 sm:pt-28 lg:pt-36">
        <div
          className={cn(
            'flex w-full flex-col gap-12',
            showRecent &&
              '[@media(max-height:820px)_and_(min-width:768px)]:flex-row [@media(max-height:820px)_and_(min-width:768px)]:items-start [@media(max-height:820px)_and_(min-width:768px)]:justify-between lg:flex-row lg:items-start lg:justify-between',
            !showRecent && 'max-w-xl',
          )}
        >
          <div className="max-w-xl flex-1 space-y-12">
            <div className="space-y-4">
              <h1 className="text-3xl font-bold leading-tight tracking-tight sm:text-4xl md:text-5xl">
                Talk to people,
                <br />
                <span className="text-primary">not the platform.</span>
              </h1>
              <p className="max-w-sm text-sm leading-relaxed text-muted-foreground">
                Self-hosted voice rooms. Share a link, start talking. No account needed to join.
              </p>
            </div>

            <div className="max-w-md space-y-3">
              <JoinForm />
              <JoinHint guest={guest} />
            </div>
          </div>

          {showRecent ? (
            <RecentMeetings
              rooms={recentRooms}
              onJoin={(name) => navigate({ to: '/m/$meetId', params: { meetId: name } })}
              onRemove={removeRecent}
              className="w-full max-w-md shrink-0 lg:ms-auto lg:pt-2 [@media(max-height:820px)_and_(min-width:768px)]:ms-auto [@media(max-height:820px)_and_(min-width:768px)]:pt-2"
            />
          ) : null}
        </div>
      </main>

      <footer className="relative z-10 flex items-center gap-4 border-t px-6 py-3 text-xs text-muted-foreground sm:px-10">
        <a
          href="https://github.com/themadorg"
          target="_blank"
          rel="noopener noreferrer"
          className="transition-colors hover:text-foreground"
          suppressHydrationWarning
        >
          &copy; {new Date().getFullYear()} themadorg
        </a>
        <Separator orientation="vertical" className="h-3" />
        <a
          href="https://bedrud.org/en/docs/getting-started/quickstart/?utm_source=app&utm_medium=footer"
          target="_blank"
          rel="noopener noreferrer"
          className="transition-colors hover:text-foreground"
        >
          Docs
        </a>
        <a
          href="https://bedrud.org/github?utm_source=app&utm_medium=footer"
          target="_blank"
          rel="noopener noreferrer"
          className="transition-colors hover:text-foreground"
        >
          GitHub
        </a>
      </footer>

      {guest ? <HomeSettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} /> : null}
    </div>
  )
}
