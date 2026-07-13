import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, Loader2, MailCheck } from 'lucide-react'
import { useState } from 'react'
import { ApiError, api } from '#/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/forgot-password')({ component: ForgotPasswordPage })

function ForgotPasswordPage() {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [fieldError, setFieldError] = useState('')
  const [sent, setSent] = useState(false)

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const email = (fd.get('email') as string).trim()

    if (!email || !/\S+@\S+\.\S+/.test(email)) {
      setFieldError('Enter a valid email')
      return
    }
    setFieldError('')
    setError('')
    setIsLoading(true)
    try {
      await api.post('/api/auth/forgot-password', { email })
      setSent(true)
    } catch (err) {
      if (err instanceof ApiError && err.parsedBody?.error) {
        setError(err.parsedBody.error as string)
      } else {
        setError(err instanceof Error ? err.message : 'Something went wrong')
      }
    } finally {
      setIsLoading(false)
    }
  }

  if (sent) {
    return (
      <div className="space-y-7">
        <div className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
            <MailCheck className="h-7 w-7 text-primary" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">Check your email</h1>
          <p className="text-sm text-muted-foreground">
            If an account with that email exists, we&apos;ve sent a password reset link.
          </p>
        </div>

        <p className="text-center text-sm text-muted-foreground">
          Click the link in the email to reset your password. The link expires in 1 hour.
        </p>

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
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Reset your password</h1>
        <p className="text-sm text-muted-foreground">
          Enter your email and we&apos;ll send you a link to reset your password.
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

      <form method="post" action="#" onSubmit={handleSubmit} className="space-y-4" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            name="email"
            type="email"
            placeholder="you@example.com"
            autoComplete="email"
            autoFocus
            onChange={() => setFieldError('')}
          />
          {fieldError && <p className="text-xs text-destructive">{fieldError}</p>}
        </div>

        <Button type="submit" className="w-full" disabled={isLoading}>
          {isLoading ? (
            <>
              <Loader2 className="me-2 h-4 w-4 animate-spin" /> Sending…
            </>
          ) : (
            'Send reset link'
          )}
        </Button>
      </form>

      <p className="text-center text-sm text-muted-foreground">
        <Link
          to="/auth/login"
          search={{ redirect: undefined }}
          className="inline-flex items-center gap-1 font-medium text-foreground underline-offset-4 hover:underline"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back to sign in
        </Link>
      </p>
    </div>
  )
}
