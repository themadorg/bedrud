import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Loader2, ShieldCheck } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/reset-password')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: typeof search.token === 'string' && search.token.length > 0 ? search.token : '',
  }),
  component: ResetPasswordPage,
})

function ResetPasswordPage() {
  const navigate = useNavigate()
  const { token } = Route.useSearch()

  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<{ password?: string; confirm?: string }>({})
  const [success, setSuccess] = useState(false)

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()

    if (!token) {
      setError('Invalid or missing reset link. Please request a new password reset.')
      return
    }

    const fd = new FormData(e.currentTarget)
    const newPassword = fd.get('newPassword') as string
    const confirm = fd.get('confirm') as string

    const errs: typeof fieldErrors = {}
    if (newPassword.length < 12) errs.password = 'At least 12 characters'
    if (newPassword !== confirm) errs.confirm = 'Passwords do not match'
    if (Object.keys(errs).length) {
      setFieldErrors(errs)
      return
    }

    setFieldErrors({})
    setError('')
    setIsLoading(true)
    try {
      await api.post('/api/auth/reset-password', { token, newPassword })
      setSuccess(true)
    } catch (err: any) {
      setError(err?.parsedBody?.error || err instanceof Error ? err.message : 'Failed to reset password')
    } finally {
      setIsLoading(false)
    }
  }

  if (!token && !success) {
    return (
      <div className="space-y-7">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">Invalid link</h1>
          <p className="text-sm text-muted-foreground">
            This password reset link is invalid or missing. Please request a new one.
          </p>
        </div>

        <div className="border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          No reset token found in the URL.
        </div>

        <p className="text-center text-sm text-muted-foreground">
          <Link to="/auth/forgot-password" className="font-medium text-foreground underline-offset-4 hover:underline">
            Request a new reset link
          </Link>
          {' · '}
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

  if (success) {
    return (
      <div className="space-y-7">
        <div className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
            <ShieldCheck className="h-7 w-7 text-primary" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">Password reset</h1>
          <p className="text-sm text-muted-foreground">Your password has been reset successfully.</p>
        </div>

        <p className="text-center text-sm text-muted-foreground">You can now sign in with your new password.</p>

        <Button className="w-full" onClick={() => navigate({ to: '/auth/login', search: { redirect: undefined } })}>
          Sign in
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Set new password</h1>
        <p className="text-sm text-muted-foreground">Enter your new password below.</p>
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

      <form onSubmit={handleSubmit} className="space-y-4" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="new-password">New password</Label>
          <div className="relative">
            <Input
              id="new-password"
              name="newPassword"
              type={showPassword ? 'text' : 'password'}
              placeholder="At least 12 characters"
              autoComplete="new-password"
              autoFocus
              className="pe-10"
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

        <div className="space-y-1.5">
          <Label htmlFor="confirm-password">Confirm password</Label>
          <Input
            id="confirm-password"
            name="confirm"
            type={showPassword ? 'text' : 'password'}
            placeholder="••••••••"
            autoComplete="new-password"
            onChange={() => setFieldErrors((p) => ({ ...p, confirm: undefined }))}
          />
          {fieldErrors.confirm && <p className="text-xs text-destructive">{fieldErrors.confirm}</p>}
        </div>

        <Button type="submit" className="w-full" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="me-2 h-4 w-4 animate-spin" /> Resetting…
            </>
          ) : (
            'Reset password'
          )}
        </Button>
      </form>

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
