import { createFileRoute, Link } from '@tanstack/react-router'
import { Eye, EyeOff, Fingerprint, KeyRound, Loader2, MailCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, api } from '#/lib/api'
import type { AuthResponse } from '#/lib/handle-auth-success'
import { useHandleAuthSuccess } from '#/lib/handle-auth-success'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { cn } from '#/lib/utils'
import { signupWithPasskey } from '@/components/auth/PasskeyButton'
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
  const [usePasskey, setUsePasskey] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')

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
          chatUploadMaxBytes: 10485760,
          chatUploadMaxDimension: 8192,
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
    const trimmedName = name.trim()
    const trimmedEmail = email.trim()
    const password = fd.get('password') as string
    const confirm = fd.get('confirm') as string
    const inviteToken = ((fd.get('inviteToken') as string) ?? '').trim()

    const errs: typeof fieldErrors = {}
    if (trimmedName.length < 2) errs.name = 'At least 2 characters'
    if (!trimmedEmail || !/\S+@\S+\.\S+/.test(trimmedEmail)) errs.email = 'Enter a valid email'

    if (usePasskey) {
      if (Object.keys(errs).length) {
        setFieldErrors(errs)
        return
      }
      setFieldErrors({})
      setError('')
      setIsLoading(true)
      try {
        const res = await signupWithPasskey(trimmedName, trimmedEmail)
        if ('requiresVerification' in res && res.requiresVerification) {
          setRegisteredEmail(res.email)
          startCooldown(120)
          return
        }
        handleAuthSuccess(res)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Passkey signup failed')
      } finally {
        setIsLoading(false)
      }
      return
    }

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
      const body: Record<string, string> = { name: trimmedName, email: trimmedEmail, password }
      if (inviteToken) body.inviteToken = inviteToken
      const res = await api.post<AuthResponse | { requiresVerification: boolean; message: string; email: string }>(
        '/api/auth/register',
        body as any,
      )

      if ('requiresVerification' in res && res.requiresVerification) {
        setRegisteredEmail((res as any).email)
        startCooldown(120)
        return
      }

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
      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Account</p>
        <h1 className="min-h-8 text-2xl font-semibold tracking-tight">Create an account</h1>
        <p className="min-h-10 text-sm text-muted-foreground">
          Create your account to host rooms and manage your profile.
        </p>
      </div>

      {error && (
        <div
          role="alert"
          aria-live="assertive"
          className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
        >
          {error}
        </div>
      )}

      <form method="post" action="#" onSubmit={handleSubmit} className="space-y-4" autoComplete="on" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="reg-name">Full name</Label>
          <Input
            id="reg-name"
            name="name"
            value={name}
            placeholder="Jane Smith"
            autoComplete="name"
            autoFocus
            required
            onChange={(e) => {
              setName(e.target.value)
              clearField('name')
            }}
          />
          {fieldErrors.name && <p className="text-xs text-destructive">{fieldErrors.name}</p>}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="reg-username">Email</Label>
          <Input
            id="reg-username"
            name="username"
            type="email"
            inputMode="email"
            value={email}
            placeholder="you@example.com"
            autoComplete="username webauthn"
            required
            onChange={(e) => {
              setEmail(e.target.value)
              clearField('email')
            }}
          />
          {fieldErrors.email && <p className="text-xs text-destructive">{fieldErrors.email}</p>}
        </div>

        {settings?.passkeysEnabled !== false && (
          <Button
            type="button"
            variant={usePasskey ? 'default' : 'outline'}
            onClick={() => {
              setUsePasskey((v) => !v)
              setError('')
              setFieldErrors((p) => ({
                ...p,
                password: undefined,
                confirm: undefined,
                inviteToken: undefined,
              }))
            }}
            aria-pressed={usePasskey}
            className={cn(
              'h-10 w-full gap-2 text-sm',
              usePasskey && 'border-primary/30 bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary',
            )}
          >
            <Fingerprint className="h-4 w-4" />
            {usePasskey ? 'Using passkey' : 'Use passkey'}
          </Button>
        )}

        {!usePasskey && (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="new-password">Password</Label>
              <div className="relative">
                <Input
                  id="new-password"
                  name="password"
                  type={showPassword ? 'text' : 'password'}
                  placeholder="At least 12 characters"
                  autoComplete="new-password"
                  className="pe-10"
                  required
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

            <div className="space-y-1.5">
              <Label htmlFor="new-password-confirm">Confirm password</Label>
              <Input
                id="new-password-confirm"
                name="confirm"
                type={showPassword ? 'text' : 'password'}
                placeholder="••••••••"
                autoComplete="new-password"
                required
                onChange={() => clearField('confirm')}
              />
              {fieldErrors.confirm && <p className="text-xs text-destructive">{fieldErrors.confirm}</p>}
            </div>

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
          </>
        )}

        <Button type="submit" className="w-full" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="me-2 h-4 w-4 animate-spin" />{' '}
              {usePasskey ? 'Setting up…' : 'Creating account…'}
            </>
          ) : usePasskey ? (
            <>
              <Fingerprint className="me-2 h-4 w-4" /> Create account with Passkey
            </>
          ) : (
            'Create account'
          )}
        </Button>
      </form>

      {settings?.guestLoginEnabled === false ? null : (
        <p className="text-center text-sm text-muted-foreground">
          <Link to="/auth" className="font-medium text-primary underline-offset-4 hover:underline">
            Continue as guest
          </Link>
        </p>
      )}
    </div>
  )
}
