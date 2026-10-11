import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Fingerprint, Loader2, MailCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { FormattedMessage } from 'react-intl'
import { ApiError, api } from '#/lib/api'
import { type AuthResponse, useStoreAuthSession } from '#/lib/handle-auth-success'
import {
  autofillPasskeyLogin,
  loginWithPasskey,
  passkeyAutofillSupported,
  passkeyErrorMessage,
  shouldOfferPasskey,
} from '#/lib/passkey'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { OAuthButtons } from '@/components/auth/OAuthButtons'
import { PasskeyOffer } from '@/components/auth/PasskeyOffer'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

export const Route = createFileRoute('/auth/login')({
  head: () => ({ meta: [{ title: 'Sign In — Bedrud' }] }),
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' && search.redirect.startsWith('/') ? search.redirect : undefined,
  }),
  component: LoginPage,
})

const EMAIL_PATTERN = /\S+@\S+\.\S+/

function LoginPage() {
  const navigate = useNavigate()
  const { redirect } = Route.useSearch()
  const storeSession = useStoreAuthSession()

  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({})
  const [error, setError] = useState('')
  const [email, setEmail] = useState('')
  const [settings, setSettings] = useState<PublicSettings | null>(null)
  const [passkeyLoading, setPasskeyLoading] = useState(false)
  // Set after a password sign-in on an account with no passkey yet: the user id to offer one to.
  const [passkeyOfferFor, setPasskeyOfferFor] = useState<string | null>(null)

  // Email verification state
  const [unverifiedEmail, setUnverifiedEmail] = useState<string | null>(null)
  const [resendCooldown, setResendCooldown] = useState(0)
  const [resending, setResending] = useState(false)
  const cooldownInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const cancelledRef = useRef(false)
  const autofill = useRef<AbortController | null>(null)

  useEffect(() => {
    cancelledRef.current = false
    getPublicSettings().then((s) => {
      if (!cancelledRef.current) setSettings(s)
    })
    return () => {
      cancelledRef.current = true
      if (cooldownInterval.current) clearInterval(cooldownInterval.current)
    }
  }, [])

  const showPasskey = settings?.passkeysEnabled !== false
  const oauthProviders = settings?.oauthProviders ?? []

  function leave() {
    navigate({ to: redirect ?? '/dashboard' })
  }

  // Stores the session, then leaves — unless this was a password sign-in on an account with no
  // passkey, in which case adding one is offered first.
  async function completeSignIn(res: AuthResponse, viaPassword: boolean) {
    autofill.current?.abort()
    storeSession(res)
    if (viaPassword && showPasskey && (await shouldOfferPasskey(res.user.id))) {
      setPasskeyOfferFor(res.user.id)
      return
    }
    leave()
  }

  // Lists saved passkeys in the email field's autofill until one is picked, the page is left, or
  // the passkey button takes over — a browser serves one WebAuthn request at a time.
  function startPasskeyAutofill() {
    autofill.current?.abort()
    const controller = new AbortController()
    autofill.current = controller
    void (async () => {
      if (!(await passkeyAutofillSupported()) || controller.signal.aborted) return
      try {
        const res = await autofillPasskeyLogin(controller.signal)
        if (res) await completeSignIn(res, false)
      } catch (err) {
        if (controller.signal.aborted) return
        setError(passkeyErrorMessage(err, 'Passkey sign-in failed'))
        startPasskeyAutofill()
      }
    })()
  }
  const startPasskeyAutofillRef = useRef(startPasskeyAutofill)
  startPasskeyAutofillRef.current = startPasskeyAutofill

  // Held until the settings arrive, so a server with passkeys turned off is never asked.
  useEffect(() => {
    if (!settings || settings.passkeysEnabled === false) return
    startPasskeyAutofillRef.current()
    return () => autofill.current?.abort()
  }, [settings])

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

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const email = ((fd.get('username') as string) || '').trim()
    const password = fd.get('password') as string
    const errs: typeof fieldErrors = {}
    if (!email || !EMAIL_PATTERN.test(email)) errs.email = 'Enter a valid email'
    if (!password || password.length < 12) errs.password = 'At least 12 characters'
    if (Object.keys(errs).length) {
      setFieldErrors(errs)
      return
    }
    setFieldErrors({})
    setError('')
    setIsLoading(true)
    try {
      const res = await api.post<AuthResponse>('/api/auth/login', { email, password })
      await completeSignIn(res, true)
    } catch (err) {
      if (err instanceof ApiError && err.parsedBody?.requiresVerification) {
        setUnverifiedEmail(err.parsedBody.email as string)
        startCooldown(120)
        return
      }
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setIsLoading(false)
    }
  }

  async function handleResend() {
    if (resendCooldown > 0 || !unverifiedEmail) return
    setResending(true)
    try {
      await api.post('/api/auth/verify/resend', { email: unverifiedEmail })
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

  async function handlePasskeyLogin() {
    autofill.current?.abort()
    setPasskeyLoading(true)
    setError('')
    setFieldErrors({})
    try {
      const trimmed = email.trim()
      const res = await loginWithPasskey(EMAIL_PATTERN.test(trimmed) ? trimmed : undefined)
      await completeSignIn(res, false)
    } catch (err) {
      setError(passkeyErrorMessage(err, 'Passkey sign-in failed'))
      startPasskeyAutofill()
    } finally {
      setPasskeyLoading(false)
    }
  }

  if (passkeyOfferFor) {
    return <PasskeyOffer userId={passkeyOfferFor} onDone={leave} />
  }

  // ── Email verification interstitial ──────────────────────────────────
  if (unverifiedEmail) {
    return (
      <div className="space-y-7">
        <div className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
            <MailCheck className="h-7 w-7 text-primary" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">Check your email</h1>
          <p className="text-sm text-muted-foreground">
            Please verify your email before signing in. We sent a verification email to{' '}
            <span className="font-medium text-foreground">{unverifiedEmail}</span>
          </p>
        </div>

        <p className="text-center text-sm text-muted-foreground">
          Click the link in the email to verify your account. The link expires in 24 hours.
        </p>

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

  return (
    <div className="space-y-7">
      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Account</p>
        <h1 className="min-h-8 text-2xl font-semibold tracking-tight">
          <FormattedMessage id="auth.login.title" defaultMessage="Welcome back" />
        </h1>
        <p className="min-h-10 text-sm text-muted-foreground">
          <FormattedMessage id="auth.login.subtitle" defaultMessage="Sign in to your account to continue." />
        </p>
      </div>

      {error ? <Alert type="error" message={error} /> : null}

      <form method="post" action="#" onSubmit={handleSubmit} className="space-y-4" autoComplete="on" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="username">
            <FormattedMessage id="auth.login.email" defaultMessage="Email" />
          </Label>
          <Input
            id="username"
            name="username"
            type="email"
            inputMode="email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value)
              setFieldErrors((p) => ({ ...p, email: undefined }))
            }}
            placeholder="you@example.com"
            autoComplete="username webauthn"
            autoFocus
            required
          />
          {fieldErrors.email && <p className="text-xs text-destructive">{fieldErrors.email}</p>}
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <Label htmlFor="current-password">
              <FormattedMessage id="auth.login.password" defaultMessage="Password" />
            </Label>
            <Link
              to="/auth/forgot-password"
              className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              Forgot password?
            </Link>
          </div>
          <div className="relative">
            <Input
              id="current-password"
              name="password"
              type={showPassword ? 'text' : 'password'}
              placeholder="••••••••"
              autoComplete="current-password"
              className="pe-10"
              required
              onChange={() => setFieldErrors((p) => ({ ...p, password: undefined }))}
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

        <Button type="submit" className="w-full" disabled={isLoading || passkeyLoading}>
          {isLoading ? (
            <>
              <Loader2 className="me-2 h-4 w-4 animate-spin" />{' '}
              <FormattedMessage id="auth.login.signingIn" defaultMessage="Signing in…" />
            </>
          ) : (
            <FormattedMessage id="auth.login.signIn" defaultMessage="Sign in" />
          )}
        </Button>
      </form>

      {/* Passkey and OAuth are ways in on their own, not steps of the form above. */}
      {showPasskey || oauthProviders.length > 0 ? (
        <div className="space-y-4">
          <div className="relative">
            <Separator />
            <span className="absolute inset-0 flex items-center justify-center">
              <span className="bg-background px-3 text-xs text-muted-foreground">or</span>
            </span>
          </div>
          <div className="space-y-2">
            {showPasskey && (
              <Button
                type="button"
                variant="outline"
                onClick={() => void handlePasskeyLogin()}
                disabled={passkeyLoading || isLoading}
                className="w-full gap-2"
              >
                {passkeyLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Fingerprint className="h-4 w-4" />}
                {passkeyLoading ? 'Waiting for passkey…' : 'Sign in with a passkey'}
              </Button>
            )}
            {oauthProviders.length > 0 && <OAuthButtons availableProviders={oauthProviders} />}
          </div>
        </div>
      ) : null}

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
