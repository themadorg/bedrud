import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowRight, Loader2, UserRound } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { useUserStore } from '#/lib/user.store'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

export const Route = createFileRoute('/auth/')({ component: GuestPage })

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

function GuestPage() {
  const navigate = useNavigate()
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [settings, setSettings] = useState<PublicSettings | null>(null)

  useEffect(() => {
    getPublicSettings()
      .then(setSettings)
      .catch(() =>
        setSettings({
          serverName: '',
          registrationEnabled: true,
          tokenRegistrationOnly: false,
          guestLoginEnabled: true,
          passkeysEnabled: true,
          oauthProviders: [],
          requireEmailVerification: false,
          chatMaxMessageCount: 10000,
          chatMessageTTLHours: 2160,
          chatUploadMaxBytes: 10485760,
          chatUploadMaxDimension: 8192,
          // TODO oncoming feature
          recordingsEnabled: true,
        }),
      )
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (trimmed.length < 2) {
      setError('Name must be at least 2 characters')
      return
    }
    setError('')
    setIsLoading(true)
    try {
      const res = await api.post<AuthResponse>('/api/auth/guest-login', { name: trimmed })
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
      navigate({ to: '/dashboard' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setIsLoading(false)
    }
  }

  if (settings?.registrationEnabled === false) {
    return (
      <div className="space-y-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">Registration closed</h1>
          <p className="text-sm text-muted-foreground">This instance is not accepting new accounts.</p>
        </div>
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          The administrator has disabled new registrations.
        </div>
        <p className="text-center text-sm text-muted-foreground">
          Already have an account?{' '}
          <Link
            to="/auth/login"
            search={{ redirect: undefined }}
            className="font-medium text-foreground underline-offset-4 hover:underline"
          >
            Sign in
          </Link>
        </p>
      </div>
    )
  }

  if (settings?.guestLoginEnabled === false) {
    return (
      <div className="space-y-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">Guest login disabled</h1>
          <p className="text-sm text-muted-foreground">This instance does not allow guest access.</p>
        </div>
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          The administrator has disabled guest login.
        </div>
        <p className="text-center text-sm text-muted-foreground">
          <Link
            to="/auth/login"
            search={{ redirect: undefined }}
            className="font-medium text-foreground underline-offset-4 hover:underline"
          >
            Sign in
          </Link>
          {' · '}
          <Link to="/auth/register" className="font-medium text-foreground underline-offset-4 hover:underline">
            Register
          </Link>
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="space-y-1">
        <div className="mb-4 inline-flex h-11 w-11 items-center justify-center bg-accent text-accent-foreground">
          <UserRound className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Join as guest</h1>
        <p className="text-sm text-muted-foreground">No account needed — just pick a name and you're in.</p>
      </div>

      {/* Form */}
      <form method="post" action="#" onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="guest-name">Display name</Label>
          <Input
            id="guest-name"
            placeholder="What should we call you?"
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setError('')
            }}
            autoFocus
            autoComplete="nickname"
          />
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>

        <Button type="submit" className="w-full gap-2" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Joining…
            </>
          ) : (
            <>
              {' '}
              Continue as guest <ArrowRight className="h-4 w-4" />
            </>
          )}
        </Button>
      </form>

      {/* Divider */}
      <div className="relative">
        <Separator />
        <span className="absolute inset-0 flex items-center justify-center">
          <span className="bg-background px-3 text-xs text-muted-foreground">have an account?</span>
        </span>
      </div>

      {/* Auth links */}
      <div className="grid grid-cols-2 gap-3">
        <Link to="/auth/login" search={{ redirect: undefined }}>
          <Button variant="outline" className="w-full">
            Sign in
          </Button>
        </Link>
        <Link to="/auth/register">
          <Button variant="outline" className="w-full">
            Register
          </Button>
        </Link>
      </div>
    </div>
  )
}
