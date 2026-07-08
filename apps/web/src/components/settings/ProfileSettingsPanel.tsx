import { Check, Loader2, Shield, Trash2, Upload } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { resolveAvatarUrl } from '#/lib/avatar-url'
import { getPalette } from '#/lib/participant-palette'
import { useProfileSyncStore } from '#/lib/profile-sync.store'
import type { User } from '#/lib/user.store'
import { useUserStore } from '#/lib/user.store'
import { Alert } from '@/components/ui/alert'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
import { isMeetingTone, panelSurfaceClass, type SettingsPanelTone } from './settingsPanelTone'

interface MeResponse {
  id: string
  name: string
  email: string
  provider: string
  accesses: string[] | null
  avatarUrl?: string
}

function mapMeToUser(u: MeResponse): User {
  return {
    id: u.id,
    email: u.email,
    name: u.name,
    provider: u.provider,
    isSuperAdmin: u.accesses?.includes('superadmin') ?? false,
    isAdmin: (u.accesses?.includes('admin') || u.accesses?.includes('superadmin')) ?? false,
    accesses: u.accesses ?? [],
    avatarUrl: u.avatarUrl,
  }
}

function profileInitials(name: string, email?: string): string {
  const source = name.trim() || email?.trim() || '?'
  const parts = source.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) return `${parts[0]![0] ?? ''}${parts[1]![0] ?? ''}`.toUpperCase()
  return source.slice(0, 2).toUpperCase()
}

function roleLabel(accesses: string[] | null | undefined): string {
  if (accesses?.includes('superadmin')) return 'Superadmin'
  if (accesses?.includes('admin')) return 'Admin'
  if (accesses?.includes('moderator')) return 'Moderator'
  if (accesses?.includes('guest')) return 'Guest'
  return 'User'
}

function roleBadgeClass(role: string, meeting: boolean): string {
  if (role === 'Superadmin' || role === 'Admin') {
    return meeting
      ? 'border-[color-mix(in_oklab,var(--accent-600)_28%,transparent)] bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
      : 'border-primary/30 bg-primary/10 text-primary'
  }
  if (role === 'Moderator') {
    return meeting
      ? 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-300'
      : 'border-amber-500/30 bg-amber-500/10 text-amber-600'
  }
  return meeting
    ? 'border-[var(--meet-border)] bg-[var(--meet-control)] text-[var(--meet-fg-muted)]'
    : 'border-border bg-muted/40 text-muted-foreground'
}

function sectionBorderClass(meeting: boolean) {
  return meeting ? 'border-[var(--meet-border)]' : 'border-border'
}

export function ProfileSettingsPanel({ tone = 'default' }: { tone?: SettingsPanelTone }) {
  const meeting = isMeetingTone(tone)
  const user = useUserStore((s) => s.user)
  const setUser = useUserStore((s) => s.setUser)
  const accessToken = useAuthStore((s) => s.tokens?.accessToken)
  const [name, setName] = useState(user?.name ?? '')
  const [isLoading, setIsLoading] = useState(false)
  const [isHydrating, setIsHydrating] = useState(false)
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [avatarRemoving, setAvatarRemoving] = useState(false)
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null)
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!accessToken) return
    let cancelled = false
    setIsHydrating(true)
    api
      .get<MeResponse>('/api/auth/me')
      .then((me) => {
        if (cancelled) return
        setUser(mapMeToUser(me))
        setName(me.name)
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setIsHydrating(false)
      })
    return () => {
      cancelled = true
    }
  }, [accessToken, setUser])

  useEffect(() => {
    if (user?.name) setName(user.name)
  }, [user?.name])

  const palette = useMemo(() => getPalette(user?.name ?? user?.email ?? '?'), [user?.name, user?.email])
  const initials = useMemo(() => profileInitials(user?.name ?? '', user?.email), [user?.name, user?.email])
  const role = roleLabel(user?.accesses)
  const displayedAvatarUrl = avatarPreview ?? resolveAvatarUrl(user?.avatarUrl)
  const hasCustomAvatar = Boolean(user?.avatarUrl)

  useEffect(() => {
    return () => {
      if (avatarPreview?.startsWith('blob:')) URL.revokeObjectURL(avatarPreview)
    }
  }, [avatarPreview])

  async function handleAvatarSelected(file: File | undefined) {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      setStatus({ type: 'error', message: 'Please choose an image file.' })
      return
    }
    if (file.size > 2 * 1024 * 1024) {
      setStatus({ type: 'error', message: 'Image must be 2 MB or smaller.' })
      return
    }
    setStatus(null)
    setAvatarUploading(true)
    if (avatarPreview?.startsWith('blob:')) URL.revokeObjectURL(avatarPreview)
    const previewUrl = URL.createObjectURL(file)
    setAvatarPreview(previewUrl)
    try {
      const form = new FormData()
      form.append('avatar', file)
      const updated = await api.post<MeResponse>('/api/auth/me/avatar', form)
      setUser(mapMeToUser(updated))
      useProfileSyncStore.getState().bump()
      URL.revokeObjectURL(previewUrl)
      setAvatarPreview(null)
      setStatus({ type: 'success', message: 'Profile photo updated.' })
    } catch (err) {
      URL.revokeObjectURL(previewUrl)
      setAvatarPreview(null)
      setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Failed to upload photo' })
    } finally {
      setAvatarUploading(false)
    }
  }

  async function handleRemoveAvatar() {
    setStatus(null)
    setAvatarRemoving(true)
    try {
      const updated = await api.delete<MeResponse>('/api/auth/me/avatar')
      setUser(mapMeToUser(updated))
      useProfileSyncStore.getState().bump()
      setAvatarPreview(null)
      setStatus({ type: 'success', message: 'Profile photo removed.' })
    } catch (err) {
      setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Failed to remove photo' })
    } finally {
      setAvatarRemoving(false)
    }
  }

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
      const updated = await api.put<MeResponse>('/api/auth/me', { name: trimmed })
      setUser(mapMeToUser(updated))
      useProfileSyncStore.getState().bump()
      setStatus({ type: 'success', message: 'Name updated.' })
    } catch (err) {
      setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Failed to update name' })
    } finally {
      setIsLoading(false)
    }
  }

  const accountRows = [
    { label: 'Account ID', value: user?.id ?? '—', mono: true },
    {
      label: 'Sign-in method',
      value: user?.provider ? user.provider.charAt(0).toUpperCase() + user.provider.slice(1) : '—',
    },
    { label: 'Role', value: role },
  ]

  if (!accessToken) {
    return (
      <div className={cn(panelSurfaceClass(tone), 'px-5 py-8 text-center')}>
        <p className={cn('text-sm', meeting ? 'text-white/70' : 'text-muted-foreground')}>
          Profile settings are available for signed-in accounts.
        </p>
      </div>
    )
  }

  return (
    <div className={panelSurfaceClass(tone)}>
      <div className={cn('flex items-center gap-4 border-b px-5 py-5', sectionBorderClass(meeting))}>
        <div className="relative shrink-0">
          <Avatar className={cn('h-16 w-16 ring-2', meeting ? 'ring-white/10' : 'ring-border')}>
            {displayedAvatarUrl ? (
              <AvatarImage
                key={displayedAvatarUrl}
                src={displayedAvatarUrl}
                alt={user?.name}
                referrerPolicy="no-referrer"
              />
            ) : null}
            <AvatarFallback className="text-lg font-bold text-white" style={{ background: palette.avatar }}>
              {initials}
            </AvatarFallback>
          </Avatar>
          {(avatarUploading || avatarRemoving) && (
            <div className="absolute inset-0 flex items-center justify-center rounded-full bg-black/45">
              <Loader2 className="h-5 w-5 animate-spin text-white" />
            </div>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-base font-semibold">{user?.name ?? (isHydrating ? 'Loading…' : '—')}</p>
          <p className={cn('truncate text-sm', meeting ? 'text-white/50' : 'text-muted-foreground')}>
            {user?.email ?? '—'}
          </p>
          <span
            className={cn(
              'mt-2 inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide',
              roleBadgeClass(role, meeting),
            )}
          >
            {(role === 'Superadmin' || role === 'Admin' || role === 'Moderator') && <Shield className="h-3 w-3" />}
            {role}
          </span>
        </div>
      </div>

      <div className={cn('space-y-3 border-b px-5 py-4', sectionBorderClass(meeting))}>
        <div>
          <p className="text-sm font-medium">Profile photo</p>
          <p className={cn('mt-0.5 text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
            Stored on the server and shown to others in meetings
          </p>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          accept="image/png,image/jpeg,image/webp,image/gif"
          className="hidden"
          onChange={(e) => {
            void handleAvatarSelected(e.target.files?.[0])
            e.target.value = ''
          }}
        />
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={avatarUploading || avatarRemoving || isHydrating}
            className={cn('gap-1.5', meeting && 'border-white/10 bg-white/[0.04] text-white/90 hover:bg-white/[0.08]')}
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className="h-3 w-3" />
            Change photo
          </Button>
          {hasCustomAvatar && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={avatarUploading || avatarRemoving || isHydrating}
              className={cn(
                'gap-1.5',
                meeting && 'border-white/10 bg-white/[0.04] text-white/90 hover:bg-white/[0.08]',
              )}
              onClick={() => void handleRemoveAvatar()}
            >
              <Trash2 className="h-3 w-3" />
              Remove
            </Button>
          )}
        </div>
      </div>

      <div className={cn('space-y-4 border-b px-5 py-4', sectionBorderClass(meeting))}>
        <div>
          <p className="text-sm font-medium">Display name</p>
          <p className={cn('mt-0.5 text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
            How others see you in meetings and chat
          </p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-3">
          <div className="space-y-1.5">
            <Label
              htmlFor="settings-name"
              className={cn('text-xs font-medium', meeting ? 'text-white/50' : 'text-muted-foreground')}
            >
              Name
            </Label>
            <Input
              id="settings-name"
              value={name}
              onChange={(e) => {
                setName(e.target.value)
                setStatus(null)
              }}
              placeholder="Your name"
              disabled={isHydrating}
              className={cn('h-9 text-sm', meeting && 'border-white/10 bg-white/[0.04] text-white/90')}
            />
          </div>
          {user?.email && (
            <div className="space-y-1.5">
              <Label className={cn('text-xs font-medium', meeting ? 'text-white/50' : 'text-muted-foreground')}>
                Email
              </Label>
              <Input
                value={user.email}
                disabled
                className={cn('h-9 text-sm opacity-60', meeting && 'border-white/10 bg-white/[0.04] text-white/90')}
              />
            </div>
          )}
          {status && <Alert {...status} />}
          <Button
            type="submit"
            variant="default"
            size="sm"
            disabled={isLoading || isHydrating || name.trim() === user?.name}
            className="gap-1.5"
          >
            {isLoading ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3" />}
            Save changes
          </Button>
        </form>
      </div>

      <div className="px-5 py-4">
        <p className="mb-3 text-sm font-medium">Account details</p>
        <div
          className={cn(
            'overflow-hidden rounded-lg border',
            sectionBorderClass(meeting),
            meeting ? 'bg-white/[0.02]' : 'bg-muted/20',
          )}
        >
          {accountRows.map(({ label, value, mono }, index) => (
            <div
              key={label}
              className={cn(
                'flex items-center justify-between gap-4 px-3 py-2.5',
                index > 0 && cn('border-t', sectionBorderClass(meeting)),
              )}
            >
              <span className={cn('text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>{label}</span>
              <span
                className={cn(
                  'max-w-[min(100%,280px)] truncate text-xs font-medium',
                  mono && 'font-mono',
                  meeting ? 'text-white/80' : undefined,
                )}
              >
                {value}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
