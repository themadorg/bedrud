import { createFileRoute, Link, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { Lock, LogIn, Server, ShieldOff, UserPlus } from 'lucide-react'
import { useAuthStore } from '#/lib/auth.store'
import { isGuestToken } from '#/lib/jwt-user'
import { cn } from '#/lib/utils'
import { BedrudLogo } from '@/components/BedrudLogo'
import { ThemeToggle } from '@/components/ThemeToggle'

export const Route = createFileRoute('/auth')({
  beforeLoad: async ({ location }) => {
    if (typeof window === 'undefined') return
    await useAuthStore.getState().initialize()
    const tokens = useAuthStore.getState().tokens
    if (!tokens) return
    if (isGuestToken(tokens.accessToken)) {
      const path = location.pathname
      if (
        path === '/auth/login' ||
        path === '/auth/register' ||
        path.startsWith('/auth/forgot-password') ||
        path.startsWith('/auth/reset-password') ||
        path.startsWith('/auth/verify')
      ) {
        return
      }
      throw redirect({ to: '/' })
    }
    throw redirect({ to: '/dashboard' })
  },
  component: AuthLayout,
})

const TRUST = [
  { icon: Lock, label: 'End-to-end encrypted' },
  { icon: ShieldOff, label: 'Zero telemetry' },
  { icon: Server, label: 'Your infrastructure' },
] as const

function AuthModeNav() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const isLogin = pathname === '/auth/login'
  const isRegister = pathname === '/auth/register'

  if (!isLogin && !isRegister && pathname !== '/auth' && pathname !== '/auth/') return null

  const modes = [
    { to: '/auth/login' as const, label: 'Sign in', icon: LogIn, active: isLogin },
    { to: '/auth/register' as const, label: 'Register', icon: UserPlus, active: isRegister },
  ]

  return (
    <nav aria-label="Account options" className="mb-8 grid grid-cols-2 gap-1 bg-muted p-1">
      {modes.map(({ to, label, icon: Icon, active }) => (
        <Link
          key={to}
          to={to}
          search={to === '/auth/login' ? { redirect: undefined } : undefined}
          aria-current={active ? 'page' : undefined}
          className={cn(
            'flex h-9 items-center justify-center gap-1.5 text-xs font-medium transition-colors',
            active
              ? 'bg-background text-foreground shadow-sm ring-1 ring-border'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden />
          {label}
        </Link>
      ))}
    </nav>
  )
}

function AuthLayout() {
  return (
    <div className="flex min-h-screen bg-background">
      <aside className="dark relative hidden min-w-0 shrink-0 grow-0 flex-col justify-between overflow-hidden border-e border-border bg-background p-10 xl:p-14 lg:flex lg:w-[70%]">
        <div
          className="pointer-events-none absolute right-[8%] top-[18%] aspect-square w-[min(52%,28rem)] bg-primary opacity-[0.18] blur-[100px]"
          aria-hidden
        />

        <BedrudLogo size="lg" className="relative gap-2.5" />

        <div className="relative w-full max-w-2xl space-y-10">
          <div className="space-y-5">
            <p className="text-4xl font-bold leading-tight tracking-tight text-foreground xl:text-5xl">
              Talk to people,
              <br />
              <span className="text-primary">not the platform.</span>
            </p>
            <p className="max-w-xl text-base leading-relaxed text-muted-foreground xl:text-lg">
              Instant rooms. No installs. Open a room and start talking — with anyone, anywhere.
            </p>
          </div>
          <ul className="space-y-3">
            {TRUST.map(({ icon: Icon, label }) => (
              <li key={label} className="flex items-center gap-3 text-sm text-muted-foreground xl:text-base">
                <Icon className="h-4 w-4 shrink-0 text-primary xl:h-[1.125rem] xl:w-[1.125rem]" aria-hidden />
                {label}
              </li>
            ))}
          </ul>
        </div>

        <div className="relative flex items-center justify-between gap-4">
          <a
            href="https://bedrud.org?utm_source=app&utm_medium=footer"
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-muted-foreground transition-colors hover:text-foreground xl:text-sm"
          >
            <span suppressHydrationWarning>© {new Date().getFullYear()} Bedrud</span>
          </a>
          <ThemeToggle />
        </div>
      </aside>

      <div className="flex min-w-0 w-full flex-col max-lg:flex-1 lg:w-[30%] lg:shrink-0 lg:grow-0">
        <header className="flex w-full items-center justify-between px-6 py-3 sm:px-8 lg:hidden">
          <BedrudLogo />
          <ThemeToggle />
        </header>

        <main
          id="main-content"
          className="app-scroll-y flex w-full flex-1 flex-col justify-start overflow-y-auto px-6 pb-16 pt-8 sm:px-8 lg:px-10 lg:pb-12 lg:pt-14 xl:px-12"
        >
          <div className="w-full">
            <AuthModeNav />
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
