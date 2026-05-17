import { createFileRoute } from '@tanstack/react-router'
import { AlertCircle, Check, Loader2, Lock, LogIn } from 'lucide-react'
import React, { useState } from 'react'
import { api } from '#/lib/api'
import { useUserStore } from '#/lib/user.store'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/dashboard/settings/security')({
  component: SecurityPage,
})

function Alert({ type, message }: { type: 'success' | 'error'; message: string }) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border px-3 py-2 text-xs',
        type === 'success'
          ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          : 'border-destructive/30 bg-destructive/10 text-destructive',
      )}
    >
      {type === 'success' ? (
        <Check className="h-3.5 w-3.5 shrink-0" />
      ) : (
        <AlertCircle className="h-3.5 w-3.5 shrink-0" />
      )}
      {message}
    </div>
  )
}

function SecurityPage() {
  const user = useUserStore((s) => s.user)
  const [isLoading, setIsLoading] = useState(false)
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  const isOAuthOnly = user?.provider && !['local', 'passkey'].includes(user.provider)

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const currentPassword = fd.get('currentPassword') as string
    const newPassword = fd.get('newPassword') as string
    const confirmPassword = fd.get('confirmPassword') as string

    if (newPassword.length < 12) {
      setStatus({ type: 'error', message: 'New password must be at least 12 characters' })
      return
    }
    if (newPassword !== confirmPassword) {
      setStatus({ type: 'error', message: 'Passwords do not match' })
      return
    }

    setIsLoading(true)
    setStatus(null)
    try {
      await api.put('/api/auth/password', { currentPassword, newPassword })
      setStatus({ type: 'success', message: 'Password updated.' })
      ;(e.target as HTMLFormElement).reset()
    } catch (err) {
      setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Failed to update password' })
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* Password section */}
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Password</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">Change your account password</CardDescription>
        </CardHeader>
        <CardContent className="p-5">
          {isOAuthOnly ? (
            <div className="flex items-start gap-2.5 border px-3 py-3 text-xs">
              <LogIn className="h-3.5 w-3.5 shrink-0 mt-0.5 text-muted-foreground" />
              <p className="text-muted-foreground">
                Your account uses <span className="font-medium text-foreground capitalize">{user?.provider}</span> for
                sign-in. Password management is handled by your identity provider.
              </p>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="currentPassword" className="text-xs font-medium text-muted-foreground">
                  Current password
                </Label>
                <Input
                  id="currentPassword"
                  name="currentPassword"
                  type="password"
                  placeholder="••••••••"
                  required
                  onChange={() => setStatus(null)}
                  className="h-9 text-sm"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="newPassword" className="text-xs font-medium text-muted-foreground">
                  New password
                </Label>
                <Input
                  id="newPassword"
                  name="newPassword"
                  type="password"
                  placeholder="Min. 6 characters"
                  required
                  onChange={() => setStatus(null)}
                  className="h-9 text-sm"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="confirmPassword" className="text-xs font-medium text-muted-foreground">
                  Confirm new password
                </Label>
                <Input
                  id="confirmPassword"
                  name="confirmPassword"
                  type="password"
                  placeholder="••••••••"
                  required
                  onChange={() => setStatus(null)}
                  className="h-9 text-sm"
                />
              </div>
              {status && <Alert {...status} />}
              <Button type="submit" variant="default" size="sm" disabled={isLoading} className="gap-1.5">
                {isLoading ? <Loader2 className="h-3 w-3 animate-spin" /> : <Lock className="h-3 w-3" />}
                Update password
              </Button>
            </form>
          )}
        </CardContent>
      </Card>

      {/* Security info */}
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Security info</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">Your authentication details</CardDescription>
        </CardHeader>
        <CardContent className="divide-y p-5">
          <div className="flex items-center justify-between py-3 first:pt-0">
            <span className="text-xs text-muted-foreground">Auth method</span>
            <span className="text-xs font-medium capitalize">{user?.provider ?? '—'}</span>
          </div>
          <div className="flex items-center justify-between py-3">
            <span className="text-xs text-muted-foreground">Password</span>
            <span className="text-xs font-medium">{isOAuthOnly ? 'Managed by provider' : 'Set'}</span>
          </div>
          <div className="flex items-center justify-between py-3 last:pb-0">
            <span className="text-xs text-muted-foreground">Two-factor auth</span>
            <span className="text-xs font-medium text-muted-foreground">Not available</span>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
