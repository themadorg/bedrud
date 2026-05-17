import { createFileRoute, Link, redirect, useNavigate } from '@tanstack/react-router'
import { AlertCircle, ArrowRight, Radio } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { useUserStore } from '#/lib/user.store'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    if (typeof window === 'undefined') return
    await useAuthStore.getState().initialize()
    if (useAuthStore.getState().tokens) throw redirect({ to: '/dashboard' })
  },
  component: HomePage,
})

function JoinForm() {
  const navigate = useNavigate()
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [checking, setChecking] = useState(false)

  async function handleJoin(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const slug = code.trim().toLowerCase().replace(/\s+/g, '-')
    if (!slug) return
    setError(null)
    setChecking(true)
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
      const msg = err instanceof Error ? err.message : String(err)
      const status = parseInt(msg.substring(0, 3), 10)
      const jsonPart = msg.includes(':') ? msg.slice(msg.indexOf(':') + 1).trim() : ''
      let parsed: { error?: string; message?: string } = {}
      try {
        parsed = JSON.parse(jsonPart)
      } catch {
        /* ignore */
      }
      switch (status) {
        case 404:
          setError(parsed.error ?? parsed.message ?? 'Room not found')
          break
        case 403:
          setError(parsed.error?.includes('full') ? 'Room is full' : 'This room is private')
          break
        case 410:
          setError('Room is no longer active')
          break
        default:
          setError(parsed.error ?? parsed.message ?? 'Failed to join room')
      }
      setChecking(false)
      return
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
          {typeof window !== 'undefined' ? window.location.host : ''}/m/
        </span>
        <Input
          value={code}
          onChange={(e) => {
            setCode(e.target.value)
            setError(null)
          }}
          placeholder="your-room"
          autoComplete="off"
          spellCheck={false}
          className="h-10 flex-1 pl-2 pr-1 font-mono text-sm sm:pl-1 border-none focus-visible:ring-0"
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
        <div className="flex items-center gap-2 border-l-2 border-destructive bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <AlertCircle className="h-3 w-3 shrink-0" />
          {error}
        </div>
      )}
    </div>
  )
}

function HomePage() {
  return (
    <div className="relative flex min-h-screen flex-col overflow-hidden bg-background text-foreground">
      {/* Background glow — single radial per DESIGN.md rule */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden>
        <div
          className="absolute -right-24 -top-24 h-[500px] w-[500px] rounded-full opacity-[0.12] dark:opacity-[0.06] blur-[100px]"
          style={{ background: 'var(--spotlight-a)' }}
        />
      </div>

      {/* ── Top bar ──────────────────────────────────────────────────────── */}
      <header className="relative z-10 flex items-center justify-between px-6 py-3 sm:px-10">
        <div className="flex items-center gap-2">
          <div className="flex h-6 w-6 items-center justify-center bg-primary">
            <Radio className="h-3 w-3 text-primary-foreground" />
          </div>
          <span className="font-mono text-xs font-bold tracking-wider uppercase">bedrud</span>
        </div>
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <Separator orientation="vertical" className="hidden h-3 sm:block" />
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
        </div>
      </header>

      {/* ── Main ─────────────────────────────────────────────────────────── */}
      <main className="relative z-10 flex flex-1 flex-col px-6 pb-12 pt-20 sm:px-10 sm:pt-28 lg:pt-36">
        <div className="max-w-xl space-y-12">
          {/* Headline */}
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

          {/* Join + links */}
          <div className="max-w-md space-y-3">
            <JoinForm />
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
          </div>
        </div>
      </main>

      {/* ── Footer ───────────────────────────────────────────────────────── */}
      <footer className="relative z-10 flex items-center gap-4 border-t px-6 py-3 text-xs text-muted-foreground sm:px-10">
        <span>&copy; {new Date().getFullYear()} Bedrud</span>
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
    </div>
  )
}
