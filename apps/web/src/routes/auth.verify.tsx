import { createFileRoute, Link } from '@tanstack/react-router'
import { CheckCircle, Info, Mail, XCircle } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/auth/verify')({
  component: VerifyPage,
  validateSearch: (search: Record<string, string>) => ({
    status: (search.status as string) || '',
    reason: (search.reason as string) || '',
  }),
})

function SuccessView() {
  return (
    <div className="space-y-7 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-success/10">
        <CheckCircle className="h-7 w-7 text-success" />
      </div>
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Email verified!</h1>
        <p className="text-sm text-muted-foreground">
          Your email has been successfully verified. You can now sign in to your account.
        </p>
      </div>
      <Link to="/auth/login" search={{ redirect: undefined }}>
        <Button className="w-full">Sign in</Button>
      </Link>
    </div>
  )
}

function AlreadyVerifiedView() {
  return (
    <div className="space-y-7 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-accent/10">
        <Info className="h-7 w-7 text-accent-foreground" />
      </div>
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Already verified</h1>
        <p className="text-sm text-muted-foreground">Your email is already verified. No further action needed.</p>
      </div>
      <Link to="/auth/login" search={{ redirect: undefined }}>
        <Button className="w-full">Sign in</Button>
      </Link>
    </div>
  )
}

function InvalidView({ reason }: { reason: string }) {
  const messages: Record<string, string> = {
    expired: 'The verification link has expired. Request a new one.',
    not_found: 'The user account associated with this link was not found.',
    missing_token: 'No verification token provided.',
    save_error: 'An error occurred while verifying your email. Please try again.',
  }

  return (
    <div className="space-y-7 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-destructive/10">
        <XCircle className="h-7 w-7 text-destructive" />
      </div>
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">Verification failed</h1>
        <p className="text-sm text-muted-foreground">
          {messages[reason] || 'The verification link is invalid or has expired.'}
        </p>
      </div>
      <div className="space-y-3">
        <ResendVerificationForm />
        <Link to="/auth/login" search={{ redirect: undefined }}>
          <Button variant="outline" className="w-full">
            Sign in
          </Button>
        </Link>
        <p className="text-xs text-muted-foreground">
          Need a new verification link?{' '}
          <Link
            to="/auth/login"
            search={{ redirect: undefined }}
            className="font-medium text-foreground underline-offset-4 hover:underline"
          >
            Sign in to resend
          </Link>
        </p>
      </div>
    </div>
  )
}

function ResendVerificationForm() {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  async function handleResend(e: React.FormEvent) {
    e.preventDefault()
    if (!email) return
    setIsLoading(true)
    try {
      await api.post('/api/auth/verify/resend', { email })
      setSent(true)
    } catch {
      // Always uniform — user gets same message regardless
      setSent(true)
    } finally {
      setIsLoading(false)
    }
  }

  if (sent) {
    return (
      <div className="rounded-md bg-accent/10 px-4 py-3">
        <p className="text-sm text-muted-foreground">If the account exists, a verification email has been sent.</p>
      </div>
    )
  }

  return (
    <form onSubmit={handleResend} className="space-y-3">
      <div className="space-y-1 text-left">
        <Label htmlFor="resend-email">Email address</Label>
        <div className="flex gap-2">
          <Input
            id="resend-email"
            type="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <Button type="submit" disabled={isLoading || !email}>
            <Mail className="mr-2 h-4 w-4" />
            {isLoading ? 'Sending…' : 'Resend'}
          </Button>
        </div>
      </div>
    </form>
  )
}

function VerifyPage() {
  const { status, reason } = Route.useSearch()

  switch (status) {
    case 'success':
      return <SuccessView />
    case 'already_verified':
      return <AlreadyVerifiedView />
    case 'invalid':
      return <InvalidView reason={reason} />
    default:
      return null
  }
}
