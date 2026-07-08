import { createFileRoute, Link } from '@tanstack/react-router'
import { Eye, EyeOff, Fingerprint, KeyRound, Loader2, MailCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, api } from '#/lib/api'
import type { AuthResponse } from '#/lib/handle-auth-success'
import { useHandleAuthSuccess } from '#/lib/handle-auth-success'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { PasskeyButton } from '@/components/auth/PasskeyButton'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/register')({
  head: () => ({ meta: [{ title: 'Sign Up — Bedrud' }] }),
  component: RegisterPage,
})

function RegisterPage() {
  const handleAuthSuccess = useHandleAuthSuccess()
  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<{
    name?: string
    email?: string
    password?: string
    confirm?: string
    inviteToken?: string
  }>({})
  const [settings, setSettings] = useState<PublicSettings | null>(null)

  // Email verification state
  const [registeredEmail, setRegisteredEmail] = useState<string | null>(null)
  const [resendCooldown, setResendCooldown] = useState(0)
  const [resending, setResending] = useState(false)
  const cooldownInterval = useRef<ReturnType<typeof setInterval> | null>(null)

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
          // TODO oncoming feature
          recordingsEnabled: true,
        }),
      )

    // Cleanup cooldown interval on unmount
    return () => {
      if (cooldownInterval.current) clearInterval(cooldownInterval.current)
    }
  }, [])

  // Start countdown for resend cooldown
  function startCooldown(seconds: number) {
    setResendCooldown(seconds)
    if (cooldownInterval.current) clearInterval(cooldownInterval.current)
    cooldownInterval.current = setInterval(() => {
      setResendCooldown((prev) => {
        if (prev <= 1) {
          if (cooldownInterval.current) clearInterval(cooldownInterval.current)
          return 0
        }
        return prev - 1
      })
    }, 1000)
  }

  const requiresToken = settings?.tokenRegistrationOnly === true

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const name = (fd.get('name') as string).trim()
    const email = (fd.get('email') as string).trim()
    const password = fd.get('password') as string
    const confirm = fd.get('confirm') as string
    const inviteToken = ((fd.get('inviteToken') as string) ?? '').trim()

    const errs: typeof fieldErrors = {}
    if (name.length < 2) errs.name = 'At least 2 characters'
    if (!email || !/\S+@\S+\.\S+/.test(email)) errs.email = 'Enter a valid email'
    if (password.length < 12) errs.password = 'At least 12 characters'
    if (password !== confirm) errs.confirm = 'Passwords do not match'
    if (requiresToken && !inviteToken) errs.inviteToken = 'Invite token is required'
    if (Object.keys(errs).length) {
      setFieldErrors(errs)
      return
    }

    setFieldErrors({})
    setError('')
    setIsLoading(true)
    try {
      const body: Record<string, string> = { name, email, password }
      if (inviteToken) body.inviteToken = inviteToken
      const res = await api.post<AuthResponse | { requiresVerification: boolean; message: string; email: string }>(
        '/api/auth/register',
        body as any,
      )

      // Handle email verification flow
      if ('requiresVerification' in res && res.requiresVerification) {
        setRegisteredEmail((res as any).email)
        startCooldown(120) // 2 min default cooldown
        return
      }

      // Normal login flow
      handleAuthSuccess(res as AuthResponse)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setIsLoading(false)
    }
  }

  async function handleResend() {
    if (resendCooldown > 0 || !registeredEmail) return
    setResending(true)
    try {
      await api.post('/api/auth/verify/resend', { email: registeredEmail })
      startCooldown(120)
    } catch (err) {
      if (err instanceof ApiError && err.parsedBody?.retryAfter) {
        startCooldown(Number(err.parsedBody.retryAfter))
      } else {
        startCooldown(60)
      }
    } finally {
      setResending(false)
    }
  }

  function clearField(field: keyof typeof fieldErrors) {
    setFieldErrors((p) => ({ ...p, [field]: undefined }))
  }

  // ── Check email screen ──────────────────────────────────────────
  if (registeredEmail) {
    return (
      <div className="space-y-7">
        {/* Header */}
        <div className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
            <MailCheck className="h-7 w-7 text-primary" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">Check your email</h1>
          <p className="text-sm text-muted-foreground">
            We sent a verification email to <span className="font-medium text-foreground">{registeredEmail}</span>
          </p>
        </div>

        <p className="text-center text-sm text-muted-foreground">
          Click the link in the email to verify your account. The link expires in 24 hours.
        </p>

        {/* Resend */}
        <div className="text-center">
          {resendCooldown > 0 ? (
            <p className="text-xs text-muted-foreground">
              Resend available in <span className="font-medium text-foreground">{resendCooldown}s</span>
            </p>
          ) : (
            <Button variant="outline" onClick={handleResend} disabled={resending}>
              {resending ? (
                <>
                  <Loader2 className="me-2 h-4 w-4 animate-spin" /> Sending…
                </>
              ) : (
                'Resend email'
              )}
            </Button>
          )}
        </div>

        <p className="text-center text-sm text-muted-foreground">
          <Link
            to="/auth/login"
            search={{ redirect: undefined }}
            className="font-medium text-foreground underline-offset-4 hover:underline"
          >
            Back to sign in
          </Link>
        </p>
      </div>
    )
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

  return (
    <div className="space-y-7">
      {/* Header */}
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Create an account</h1>
        <p className="text-sm text-muted-foreground">Free forever. No credit card required.</p>
      </div>

      {/* Global error */}
      {error && (
        <div
          role="alert"
          aria-live="assertive"
          className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
        >
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4" noValidate>
        {/* Name */}
        <div className="space-y-1.5">
          <Label htmlFor="reg-name">Full name</Label>
          <Input
            id="reg-name"
            name="name"
            placeholder="Jane Smith"
            autoComplete="name"
            autoFocus
            onChange={() => clearField('name')}
          />
          {fieldErrors.name && <p className="text-xs text-destructive">{fieldErrors.name}</p>}
        </div>

        {/* Email */}
        <div className="space-y-1.5">
          <Label htmlFor="reg-email">Email</Label>
          <Input
            id="reg-email"
            name="email"
            type="email"
            placeholder="you@example.com"
            autoComplete="email"
            onChange={() => clearField('email')}
          />
          {fieldErrors.email && <p className="text-xs text-destructive">{fieldErrors.email}</p>}
        </div>

        {/* Password */}
        <div className="space-y-1.5">
          <Label htmlFor="reg-password">Password</Label>
          <div className="relative">
            <Input
              id="reg-password"
              name="password"
              type={showPassword ? 'text' : 'password'}
              placeholder="At least 12 characters"
              autoComplete="new-password"
              className="pe-10"
              onChange={() => clearField('password')}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setShowPassword((v) => !v)}
              className="absolute end-1 top-1/2 -translate-y-1/2 h-8 w-8"
              tabIndex={-1}
              aria-label={showPassword ? 'Hide password' : 'Show password'}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </div>
          {fieldErrors.password && <p className="text-xs text-destructive">{fieldErrors.password}</p>}
        </div>

        {/* Confirm */}
        <div className="space-y-1.5">
          <Label htmlFor="reg-confirm">Confirm password</Label>
          <Input
            id="reg-confirm"
            name="confirm"
            type={showPassword ? 'text' : 'password'}
            placeholder="••••••••"
            autoComplete="new-password"
            onChange={() => clearField('confirm')}
          />
          {fieldErrors.confirm && <p className="text-xs text-destructive">{fieldErrors.confirm}</p>}
        </div>

        {/* Invite token — only shown when required by admin settings */}
        {requiresToken && (
          <div className="space-y-1.5">
            <Label htmlFor="reg-invite" className="flex items-center gap-1.5">
              <KeyRound className="h-3.5 w-3.5" style={{ color: 'var(--accent-500)' }} />
              Invite token <span className="text-destructive">*</span>
            </Label>
            <Input
              id="reg-invite"
              name="inviteToken"
              placeholder="Paste your invite token…"
              autoComplete="off"
              spellCheck={false}
              onChange={() => clearField('inviteToken')}
            />
            {fieldErrors.inviteToken && <p className="text-xs text-destructive">{fieldErrors.inviteToken}</p>}
            <p className="text-xs text-muted-foreground">Registration on this instance requires an invite token.</p>
          </div>
        )}

        <Button type="submit" className="w-full" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="me-2 h-4 w-4 animate-spin" /> Creating account…
            </>
          ) : (
            'Create account'
          )}
        </Button>
      </form>

      {settings?.passkeysEnabled !== false && (
        <>
          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-card px-2 text-muted-foreground flex items-center gap-1.5">
                <Fingerprint className="h-3.5 w-3.5" />
                Passkey
              </span>
            </div>
          </div>

          <div className="flex justify-center">
            <PasskeyButton onSuccess={handleAuthSuccess} />
          </div>
        </>
      )}

      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{' '}
        <Link
          to="/auth/login"
          search={{ redirect: undefined }}
          className="font-medium text-foreground underline-offset-4 hover:underline"
        >
          Sign in
        </Link>
        {' · '}
        <Link to="/auth" className="font-medium text-foreground underline-offset-4 hover:underline">
          Guest mode
        </Link>
      </p>
    </div>
  )
}
