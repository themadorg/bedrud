import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Fingerprint, Loader2, MailCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { useUserStore } from '#/lib/user.store'
import { base64ToBuffer, bufferToBase64 } from '#/lib/webauthn'
import { OAuthButtons } from '@/components/auth/OAuthButtons'
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

interface AuthResponse {
  user: { id: string; email: string; name: string; provider: string; accesses: string[] | null; avatarUrl?: string }
  tokens: { accessToken: string; refreshToken: string }
}

function LoginPage() {
  const navigate = useNavigate()
  const { redirect } = Route.useSearch()
  const setTokens = useAuthStore((s) => s.setTokens)
  const setUser = useUserStore((s) => s.setUser)

  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [isPasskeyLoading, setIsPasskeyLoading] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({})
  const [error, setError] = useState('')
  const [settings, setSettings] = useState<PublicSettings | null>(null)

  // Email verification state
  const [unverifiedEmail, setUnverifiedEmail] = useState<string | null>(null)
  const [resendCooldown, setResendCooldown] = useState(0)
  const [resending, setResending] = useState(false)
  const cooldownInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const cancelledRef = useRef(false)

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
  const hasAltAuth = showPasskey || oauthProviders.length > 0

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

  function handleSuccess(res: AuthResponse) {
    setTokens(res.tokens)
    setUser({
      id: res.user.id,
      email: res.user.email,
      name: res.user.name,
      provider: res.user.provider,
      isSuperAdmin: res.user.accesses?.includes('superadmin') ?? false,
      isAdmin: (res.user.accesses?.includes('admin') || res.user.accesses?.includes('superadmin')) ?? false,
      accesses: res.user.accesses ?? [],
      avatarUrl: res.user.avatarUrl,
    })
    navigate({ to: redirect ?? '/dashboard' })
  }

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const email = (fd.get('email') as string).trim()
    const password = fd.get('password') as string
    const errs: typeof fieldErrors = {}
    if (!email || !/\S+@\S+\.\S+/.test(email)) errs.email = 'Enter a valid email'
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
      handleSuccess(res)
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

  async function handlePasskey() {
    setIsPasskeyLoading(true)
    setError('')
    try {
      const opts = await api.post<Record<string, unknown>>('/api/auth/passkey/login/begin')
      const cred = (await navigator.credentials.get({
        publicKey: {
          challenge: base64ToBuffer(opts['challenge'] as string),
          timeout: opts['timeout'] as number | undefined,
          rpId: opts['rpId'] as string | undefined,
          userVerification: opts['userVerification'] as UserVerificationRequirement | undefined,
          allowCredentials: (opts['allowCredentials'] as Array<{ id: string; type: 'public-key' }> | undefined)?.map(
            (c) => ({ id: base64ToBuffer(c.id), type: c.type }),
          ),
        },
      })) as PublicKeyCredential
      const assertion = cred.response as AuthenticatorAssertionResponse
      const res = await api.post<AuthResponse>('/api/auth/passkey/login/finish', {
        credentialId: bufferToBase64(cred.rawId),
        clientDataJSON: bufferToBase64(assertion.clientDataJSON),
        authenticatorData: bufferToBase64(assertion.authenticatorData),
        signature: bufferToBase64(assertion.signature),
      })
      handleSuccess(res)
    } catch (err) {
      if (err instanceof ApiError && err.parsedBody?.requiresVerification) {
        setUnverifiedEmail(err.parsedBody.email as string)
        startCooldown(120)
        return
      }
      // DOMException from user cancelling WebAuthn dialog
      if (err instanceof DOMException) {
        const friendly =
          err.name === 'NotAllowedError'
            ? 'Passkey cancelled or timeout'
            : err.name === 'AbortError'
              ? 'Passkey request was aborted'
              : 'Passkey authentication failed'
        setError(friendly)
        return
      }
      const msg = err instanceof Error ? err.message : 'Passkey authentication failed'
      setError(msg)
    } finally {
      setIsPasskeyLoading(false)
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

  // ── Email verification interstitial ──────────────────────────────────
  if (unverifiedEmail) {
    return (
      <div className="space-y-7">
        <div className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-primary/10">
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
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Sending…
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
      {/* Header */}
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Welcome back</h1>
        <p className="text-sm text-muted-foreground">Sign in to your account to continue.</p>
      </div>

      {/* Global error */}
      {error && (
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">{error}</div>
      )}

      {/* Email/password form */}
      <form onSubmit={handleSubmit} className="space-y-4" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            name="email"
            type="email"
            placeholder="you@example.com"
            autoComplete="email"
            autoFocus
            onChange={() => setFieldErrors((p) => ({ ...p, email: undefined }))}
          />
          {fieldErrors.email && <p className="text-xs text-destructive">{fieldErrors.email}</p>}
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <Label htmlFor="password">Password</Label>
            <Link
              to="/auth/forgot-password"
              className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              Forgot password?
            </Link>
          </div>
          <div className="relative">
            <Input
              id="password"
              name="password"
              type={showPassword ? 'text' : 'password'}
              placeholder="••••••••"
              autoComplete="current-password"
              className="pr-10"
              onChange={() => setFieldErrors((p) => ({ ...p, password: undefined }))}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setShowPassword((v) => !v)}
              className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8"
              tabIndex={-1}
              aria-label={showPassword ? 'Hide password' : 'Show password'}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </div>
          {fieldErrors.password && <p className="text-xs text-destructive">{fieldErrors.password}</p>}
        </div>

        <Button type="submit" className="w-full" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Signing in…
            </>
          ) : (
            'Sign in'
          )}
        </Button>
      </form>

      {/* Divider — only if alt auth methods exist */}
      {hasAltAuth && (
        <div className="relative">
          <Separator />
          <span className="absolute inset-0 flex items-center justify-center">
            <span className="bg-background px-3 text-xs text-muted-foreground">or continue with</span>
          </span>
        </div>
      )}

      {/* Passkey */}
      {showPasskey && (
        <Button variant="outline" className="w-full gap-2" onClick={handlePasskey} disabled={isPasskeyLoading}>
          {isPasskeyLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Fingerprint className="h-4 w-4" />}
          {isPasskeyLoading ? 'Authenticating…' : 'Sign in with Passkey'}
        </Button>
      )}

      {/* OAuth */}
      <OAuthButtons availableProviders={oauthProviders} />

      {/* Footer links */}
      <p className="text-center text-sm text-muted-foreground">
        No account?{' '}
        {settings?.registrationEnabled === false ? (
          <span className="text-muted-foreground/50">Registration (closed)</span>
        ) : (
          <Link to="/auth/register" className="font-medium text-foreground underline-offset-4 hover:underline">
            Register
          </Link>
        )}
        {' · '}
        {settings?.guestLoginEnabled === false ? (
          <span className="text-muted-foreground/50">Guest (disabled)</span>
        ) : (
          <Link to="/auth" className="font-medium text-foreground underline-offset-4 hover:underline">
            Guest mode
          </Link>
        )}
      </p>
    </div>
  )
}
