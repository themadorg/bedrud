import { createFileRoute } from '@tanstack/react-router'
import { AlertCircle, Check, Loader2, Shield } from 'lucide-react'
import React, { useState } from 'react'
import { api } from '#/lib/api'
import { useUserStore } from '#/lib/user.store'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/dashboard/settings/')({
  component: ProfilePage,
})

function Alert({ type, message }: { type: 'success' | 'error'; message: string }) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border px-3 py-3 text-xs',
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

function ProfilePage() {
  const user = useUserStore((s) => s.user)
  const setUser = useUserStore((s) => s.setUser)
  const [name, setName] = useState(user?.name ?? '')
  const [isLoading, setIsLoading] = useState(false)
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  async function handleSubmit(e: React.SyntheticEvent<HTMLFormElement>) {
    e.preventDefault()
    const trimmed = name.trim()
    if (trimmed.length < 2) {
      setStatus({ type: 'error', message: 'Name must be at least 2 characters' })
      return
    }
    setIsLoading(true)
    setStatus(null)
    try {
      const updated = await api.put<{
        id: string
        name: string
        email: string
        provider: string
        accesses: string[] | null
        avatarUrl?: string
      }>('/api/auth/me', { name: trimmed })
      if (user)
        setUser({
          ...user,
          name: updated.name,
          isSuperAdmin: updated.accesses?.includes('superadmin') ?? user.isSuperAdmin,
          isAdmin: (updated.accesses?.includes('admin') || updated.accesses?.includes('superadmin')) ?? user.isAdmin,
        })
      setStatus({ type: 'success', message: 'Name updated.' })
    } catch (err) {
      setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Failed to update name' })
    } finally {
      setIsLoading(false)
    }
  }

  const rows = [
    { label: 'Account ID', value: user?.id ?? '—', mono: true },
    {
      label: 'Sign-in method',
      value: user?.provider ? user.provider.charAt(0).toUpperCase() + user.provider.slice(1) : '—',
    },
    {
      label: 'Role',
      value: user?.accesses?.includes('superadmin')
        ? 'Superadmin'
        : user?.accesses?.includes('admin')
          ? 'Admin'
          : user?.accesses?.includes('moderator')
            ? 'Moderator'
            : user?.accesses?.includes('guest')
              ? 'Guest'
              : 'User',
    },
  ]

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* Profile form */}
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Profile</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">Update your display name</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 p-5">
          <form onSubmit={handleSubmit}>
            <div className="space-y-1.5">
              <Label htmlFor="settings-name" className="text-xs font-medium text-muted-foreground">
                Display name
              </Label>
              <Input
                id="settings-name"
                value={name}
                onChange={(e) => {
                  setName(e.target.value)
                  setStatus(null)
                }}
                placeholder="Your name"
                className="h-9 text-sm"
              />
            </div>
            {user?.email && (
              <div className="space-y-1.5">
                <span className="text-xs font-medium text-muted-foreground">Email</span>
                <Input value={user.email} disabled className="h-9 text-sm opacity-60" />
              </div>
            )}
            {status && <Alert {...status} />}
            <Button
              type="submit"
              variant="default"
              size="sm"
              disabled={isLoading || name.trim() === user?.name}
              className="gap-1.5"
            >
              {isLoading ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3" />}
              Save changes
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Account info */}
      <Card>
        <CardHeader className="border-b px-5 py-3">
          <CardTitle className="text-sm font-semibold">Account</CardTitle>
          <CardDescription className="text-xs text-muted-foreground">Your account details</CardDescription>
        </CardHeader>
        <CardContent className="divide-y p-5">
          {rows.map(({ label, value, mono }) => (
            <div key={label} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
              <span className="text-xs text-muted-foreground">{label}</span>
              <span className={cn('text-xs font-medium truncate max-w-[240px]', mono && 'font-mono')}>{value}</span>
            </div>
          ))}
          {user?.accesses?.includes('superadmin') && (
            <div className="pt-3 flex items-center gap-1.5 text-xs font-medium text-primary">
              <Shield className="h-3 w-3" /> Superadmin — full system access
            </div>
          )}
          {user?.accesses?.includes('admin') && !user?.accesses?.includes('superadmin') && (
            <div className="pt-3 flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--accent-400)' }}>
              <Shield className="h-3 w-3" /> Admin — full admin panel access
            </div>
          )}
          {user?.accesses?.includes('moderator') &&
            !user?.accesses?.includes('admin') &&
            !user?.accesses?.includes('superadmin') && (
              <div className="pt-3 flex items-center gap-1.5 text-xs font-medium" style={{ color: '#fbbf24' }}>
                <Shield className="h-3 w-3" /> Moderator — read-only admin view
              </div>
            )}
        </CardContent>
      </Card>
    </div>
  )
}
