import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowRight, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { getErrorMessage } from '#/lib/errors'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { useUserStore } from '#/lib/user.store'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/')({ component: GuestPage })

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

function ClosedState({
  title,
  message,
  detail,
  allowRegister,
}: {
  title: string
  message: string
  detail: string
  allowRegister?: boolean
}) {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Guest</p>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="text-sm text-muted-foreground">{message}</p>
      </div>
      <Alert type="error" message={detail} />
      <p className="text-sm text-muted-foreground">
        Already have an account?{' '}
        <Link
          to="/auth/login"
          search={{ redirect: undefined }}
          className="font-medium text-primary underline-offset-4 hover:underline"
        >
          Sign in
        </Link>
        {allowRegister ? (
          <>
            {' · '}
            <Link to="/auth/register" className="font-medium text-primary underline-offset-4 hover:underline">
              Register
            </Link>
          </>
        ) : null}
      </p>
    </div>
  )
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
      navigate({ to: '/' })
    } catch (err) {
      setError(getErrorMessage(err, 'Something went wrong'))
    } finally {
      setIsLoading(false)
    }
  }

  if (settings?.registrationEnabled === false) {
    return (
      <ClosedState
        title="Registration closed"
        message="This instance is not accepting new accounts."
        detail="The administrator has disabled new registrations."
      />
    )
  }

  if (settings?.guestLoginEnabled === false) {
    return (
      <ClosedState
        title="Guest login disabled"
        message="This instance does not allow guest access."
        detail="The administrator has disabled guest login."
        allowRegister={settings.registrationEnabled !== false}
      />
    )
  }

  return (
    <div className="space-y-8">
      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Guest</p>
        <h1 className="text-2xl font-semibold tracking-tight">Join without an account</h1>
        <p className="text-sm text-muted-foreground">Pick a display name. You can register later if you want rooms of your own.</p>
      </div>

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
        </div>

        {error ? <Alert type="error" message={error} /> : null}

        <Button type="submit" className="w-full gap-2" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Joining…
            </>
          ) : (
            <>
              Continue as guest <ArrowRight className="h-4 w-4" />
            </>
          )}
        </Button>
      </form>
    </div>
  )
}
