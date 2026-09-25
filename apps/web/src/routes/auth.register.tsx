import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Fingerprint, KeyRound, Loader2, MailCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, api } from '#/lib/api'
import { type AuthResponse, useStoreAuthSession } from '#/lib/handle-auth-success'
import { passkeyErrorMessage, passkeysSupported, shouldOfferPasskey, signupWithPasskey } from '#/lib/passkey'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { PasskeyOffer } from '@/components/auth/PasskeyOffer'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/register')({
  head: () => ({ meta: [{ title: 'Sign Up — Bedrud' }] }),
  component: RegisterPage,
})

/** How the new account will sign in: a password (the default form) or a passkey only. */
type SignupMethod = 'password' | 'passkey'

function RegisterPage() {
  const navigate = useNavigate()
  const storeSession = useStoreAuthSession()
  const [method, setMethod] = useState<SignupMethod>('password')
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
  // Checked after mount: the server render has no browser to ask.
  const [browserHasPasskeys, setBrowserHasPasskeys] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [inviteToken, setInviteToken] = useState('')
  // Set after a password signup that signed straight in: the user id to offer a passkey to.
  const [passkeyOfferFor, setPasskeyOfferFor] = useState<string | null>(null)

  const [registeredEmail, setRegisteredEmail] = useState<string | null>(null)
  const [resendCooldown, setResendCooldown] = useState(0)
  const [resending, setResending] = useState(false)
  const cooldownInterval = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    setBrowserHasPasskeys(passkeysSupported())
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
  const passkeysOn = settings?.passkeysEnabled !== false

  function leave() {
    navigate({ to: '/dashboard' })
  }

  // Stores the session, then leaves — unless the account was just made with a password, in which
  // case adding a passkey is offered first.
  async function completeSignup(res: AuthResponse, viaPassword: boolean) {
    storeSession(res)
    if (viaPassword && passkeysOn && (await shouldOfferPasskey(res.user.id))) {
      setPasskeyOfferFor(res.user.id)
      return
    }
    leave()
  }

  function switchMethod(next: SignupMethod) {
    setMethod(next)
    setError('')
    setFieldErrors({})
  }

  // Checks shared by both methods; the password method adds its own on top.
  function validateCommon() {
    const errs: typeof fieldErrors = {}
    if (name.trim().length < 2) errs.name = 'At least 2 characters'
    const trimmedEmail = email.trim()
    if (!trimmedEmail || !/\S+@\S+\.\S+/.test(trimmedEmail)) errs.email = 'Enter a valid email'
    if (requiresToken && !inviteToken.trim()) errs.inviteToken = 'Invite token is required'
    return errs
  }

  async function handlePasswordSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const password = fd.get('password') as string
    const confirm = fd.get('confirm') as string

    const errs = validateCommon()
    if (password.length < 12) errs.password = 'At least 12 characters'
    if (password !== confirm) errs.confirm = 'Passwords do not match'
    if (Object.keys(errs).length) {
      setFieldErrors(errs)
      return
    }

    setFieldErrors({})
    setError('')
    setIsLoading(true)
    try {
      const body: Record<string, string> = { name: name.trim(), email: email.trim(), password }
      if (inviteToken.trim()) body.inviteToken = inviteToken.trim()
      const res = await api.post<AuthResponse | { requiresVerification: boolean; message: string; email: string }>(
        '/api/auth/register',
        body as any,
      )

      if ('requiresVerification' in res && res.requiresVerification) {
        setRegisteredEmail((res as any).email)
        startCooldown(120)
        return
      }

      await completeSignup(res as AuthResponse, true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setIsLoading(false)
    }
  }

  async function handlePasskeySubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = validateCommon()
    if (Object.keys(errs).length) {
      setFieldErrors(errs)
      return
    }

    setFieldErrors({})
    setError('')
    setIsLoading(true)
    try {
      const res = await signupWithPasskey(name.trim(), email.trim(), inviteToken.trim() || undefined)
      if ('requiresVerification' in res && res.requiresVerification) {
        setRegisteredEmail(res.email)
        startCooldown(120)
        return
      }
      if (!('user' in res) || !('tokens' in res)) return
      await completeSignup(res, false)
    } catch (err) {
      setError(passkeyErrorMessage(err, 'Passkey signup failed'))
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

  if (passkeyOfferFor) {
    return <PasskeyOffer userId={passkeyOfferFor} onDone={leave} />
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
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
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

  const passkeyMode = method === 'passkey'

  // Name, email and invite token belong to both methods, and their values carry across a switch.
  const nameField = (
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
  )

  const emailField = (
    <div className="space-y-1.5">
      <Label htmlFor="reg-username">Email</Label>
      <Input
        id="reg-username"
        name="username"
        type="email"
        inputMode="email"
        value={email}
        placeholder="you@example.com"
        autoComplete="username"
        required
        onChange={(e) => {
          setEmail(e.target.value)
          clearField('email')
        }}
      />
      {fieldErrors.email && <p className="text-xs text-destructive">{fieldErrors.email}</p>}
    </div>
  )

  const inviteField = requiresToken ? (
    <div className="space-y-1.5">
      <Label htmlFor="reg-invite" className="flex items-center gap-1.5">
        <KeyRound className="h-3.5 w-3.5" style={{ color: 'var(--accent-500)' }} />
        Invite token <span className="text-destructive">*</span>
      </Label>
      <Input
        id="reg-invite"
        name="inviteToken"
        value={inviteToken}
        placeholder="Paste your invite token…"
        autoComplete="off"
        spellCheck={false}
        onChange={(e) => {
          setInviteToken(e.target.value)
          clearField('inviteToken')
        }}
      />
      {fieldErrors.inviteToken && <p className="text-xs text-destructive">{fieldErrors.inviteToken}</p>}
      <p className="text-xs text-muted-foreground">Registration on this instance requires an invite token.</p>
    </div>
  ) : null

  return (
    <div className="space-y-7">
      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Account</p>
        <h1 className="min-h-8 text-2xl font-semibold tracking-tight">
          {passkeyMode ? 'Create an account with a passkey' : 'Create an account'}
        </h1>
        <p className="min-h-10 text-sm text-muted-foreground">
          {passkeyMode
            ? 'No password to remember: you sign in with your fingerprint, face, or screen lock.'
            : 'Create your account to host rooms and manage your profile.'}
        </p>
      </div>

      {error && (
        <div
          role="alert"
          aria-live="assertive"
          className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
        >
          {error}
        </div>
      )}

      {passkeyMode ? (
        <form
          method="post"
          action="#"
          onSubmit={handlePasskeySubmit}
          className="space-y-4"
          autoComplete="on"
          noValidate
        >
          {nameField}
          {emailField}
          {inviteField}
          <Button type="submit" className="w-full gap-2" disabled={isLoading}>
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Fingerprint className="h-4 w-4" />}
            {isLoading ? 'Waiting for passkey…' : 'Create account with a passkey'}
          </Button>
        </form>
      ) : (
        <form
          method="post"
          action="#"
          onSubmit={handlePasswordSubmit}
          className="space-y-4"
          autoComplete="on"
          noValidate
        >
          {nameField}
          {emailField}

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

          {inviteField}

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
      )}

      {/* The other method is a separate path, offered only after the form, never inside it. */}
      {passkeyMode ? (
        <p className="text-center text-sm text-muted-foreground">
          <Button
            type="button"
            variant="link"
            onClick={() => switchMethod('password')}
            disabled={isLoading}
            className="h-auto p-0 font-medium"
          >
            Use a password instead
          </Button>
        </p>
      ) : passkeysOn && browserHasPasskeys ? (
        <p className="text-center text-sm text-muted-foreground">
          Rather not have a password?{' '}
          <Button
            type="button"
            variant="link"
            onClick={() => switchMethod('passkey')}
            disabled={isLoading}
            className="h-auto p-0 font-medium"
          >
            Sign up with a passkey
          </Button>
        </p>
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
