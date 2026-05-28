import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Clock, Copy, KeyRound, Loader2, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { cn } from '#/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import type { InviteToken } from './types'

function tokenExpiry(tok: InviteToken): 'used' | 'expired' | 'valid' {
  if (tok.usedAt) return 'used'
  if (new Date() > new Date(tok.expiresAt)) return 'expired'
  return 'valid'
}

export function InviteTokensSection() {
  const queryClient = useQueryClient()
  const [expiresIn, setExpiresIn] = useState(72)
  const [tokenEmail, setTokenEmail] = useState('')
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [newToken, setNewToken] = useState<InviteToken | null>(null)
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)
  const [confirmGenerate, setConfirmGenerate] = useState(false)

  const { data: tokensData, isLoading: tokensLoading } = useQuery({
    queryKey: ['admin', 'invite-tokens'],
    queryFn: () => api.get<{ tokens: InviteToken[] }>('/api/admin/invite-tokens'),
  })

  const createToken = useMutation({
    mutationFn: () =>
      api.post<InviteToken>('/api/admin/invite-tokens', {
        email: tokenEmail || undefined,
        expiresInHours: expiresIn,
      }),
    onSuccess: (token) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'invite-tokens'] })
      setNewToken(token)
      setTokenEmail('')
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to create invite token')),
  })

  const deleteToken = useMutation({
    mutationFn: (id: string) => api.delete(`/api/admin/invite-tokens/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'invite-tokens'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to delete invite token')),
  })

  function copyToken(token: InviteToken) {
    void navigator.clipboard.writeText(token.token)
    setCopiedId(token.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const tokens = tokensData?.tokens ?? []
  const validCount = tokens.filter((t) => tokenExpiry(t) === 'valid').length

  return (
    <div className="border bg-card/50">
      <div className="flex items-start justify-between gap-3 border-b px-4 py-3 sm:px-5">
        <div className="min-w-0">
          <p className="text-sm font-semibold">Invite tokens</p>
          <p className="text-xs text-muted-foreground">Generate and manage registration tokens</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {validCount > 0 && (
            <span className="border border-emerald-500/30 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
              {validCount} active
            </span>
          )}
          <span className="text-[11px] text-muted-foreground">{tokens.length} total</span>
        </div>
      </div>

      {/* Generate form */}
      <div className="border-b px-4 py-3 sm:px-5">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <input
            value={tokenEmail}
            onChange={(e) => setTokenEmail(e.target.value)}
            placeholder="Lock to email (optional)"
            className="h-8 min-w-0 flex-1 border border-input bg-background px-2.5 text-xs outline-none focus:ring-1 focus:ring-ring placeholder:text-muted-foreground"
          />
          <div className="flex gap-2 sm:contents">
            <select
              value={expiresIn}
              onChange={(e) => setExpiresIn(+e.target.value)}
              className="h-8 w-28 shrink-0 border border-input bg-background px-2.5 text-xs outline-none cursor-pointer text-foreground"
            >
              <option value={24}>24 h</option>
              <option value={72}>72 h</option>
              <option value={168}>7 days</option>
              <option value={720}>30 days</option>
            </select>
            <button
              type="button"
              onClick={() => setConfirmGenerate(true)}
              disabled={createToken.isPending}
              className="inline-flex h-9 flex-1 shrink-0 items-center justify-center gap-1.5 bg-primary px-3 text-xs font-medium text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-50 sm:flex-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {createToken.isPending ? (
                <>
                  <Loader2 className="h-3 w-3 animate-spin" /> Generating...
                </>
              ) : (
                <>
                  <KeyRound className="h-3 w-3" /> Generate
                </>
              )}
            </button>
          </div>
        </div>

        {confirmGenerate && (
          <div className="mt-2 flex flex-wrap items-center gap-2 border bg-muted/30 px-3 py-2">
            <p className="flex-1 text-xs text-muted-foreground">
              Generate {tokenEmail ? `token for ${tokenEmail}` : 'invite token'}, expires in{' '}
              {expiresIn === 24 ? '24h' : expiresIn === 72 ? '72h' : expiresIn === 168 ? '7 days' : '30 days'}?
            </p>
            <div className="flex shrink-0 items-center gap-1.5">
              <button
                type="button"
                onClick={() => setConfirmGenerate(false)}
                className="px-2 py-1 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => {
                  setConfirmGenerate(false)
                  createToken.mutate()
                }}
                className="inline-flex items-center gap-1 bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground focus-visible:ring-2 focus-visible:ring-ring"
              >
                <Check className="h-3 w-3" /> Confirm
              </button>
            </div>
          </div>
        )}

        {newToken && (
          <div className="mt-2 flex items-center gap-2 border border-emerald-500/20 bg-emerald-500/5 px-3 py-2">
            <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
            <p className="flex-1 break-all font-mono text-[11px] text-emerald-600 dark:text-emerald-400">
              {newToken.token}
            </p>
            <button type="button" onClick={() => copyToken(newToken)} className="shrink-0 p-1 hover:bg-muted">
              {copiedId === newToken.id ? (
                <Check className="h-3.5 w-3.5 text-emerald-500" />
              ) : (
                <Copy className="h-3.5 w-3.5 text-muted-foreground" />
              )}
            </button>
            <button
              type="button"
              onClick={() => setNewToken(null)}
              className="shrink-0 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Dismiss"
            >
              ×
            </button>
          </div>
        )}
      </div>

      {/* Token list */}
      {tokensLoading ? (
        <div className="divide-y">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3 sm:px-5">
              <Skeleton className="h-3.5 w-40" />
              <div className="flex-1" />
              <Skeleton className="h-4 w-12" />
            </div>
          ))}
        </div>
      ) : tokens.length === 0 ? (
        <div className="flex flex-col items-center gap-1.5 py-10">
          <KeyRound className="h-5 w-5 text-muted-foreground/30" />
          <p className="text-xs text-muted-foreground">No tokens yet</p>
        </div>
      ) : (
        <div className="max-h-64 divide-y overflow-y-auto sm:max-h-80">
          {tokens.map((tok) => {
            const status = tokenExpiry(tok)
            const isInert = status !== 'valid'
            return (
              <div
                key={tok.id}
                className={cn(
                  'group flex items-center gap-2 px-3 py-3 transition-colors sm:gap-3 sm:px-5',
                  isInert ? 'opacity-50' : 'hover:bg-accent/30',
                )}
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-[11px] text-muted-foreground">{tok.token.slice(0, 20)}…</p>
                  <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                    {tok.email && (
                      <span className="max-w-[10rem] truncate text-[10px] text-muted-foreground/70">{tok.email}</span>
                    )}
                    <span className="flex items-center gap-0.5 text-[10px] text-muted-foreground/60">
                      <Clock className="h-2.5 w-2.5 shrink-0" />
                      {new Date(tok.expiresAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                    </span>
                  </div>
                </div>

                <span
                  className={cn(
                    'shrink-0 border px-2 py-0.5 text-[10px] font-semibold',
                    status === 'valid' &&
                      'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                    status === 'expired' && 'border-destructive/30 bg-destructive/10 text-destructive',
                    status === 'used' && 'border-border bg-muted text-muted-foreground',
                  )}
                >
                  {status === 'valid' ? 'Active' : status === 'expired' ? 'Expired' : 'Used'}
                </span>

                <div className="flex shrink-0 items-center gap-0.5 opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100">
                  <button
                    type="button"
                    onClick={() => copyToken(tok)}
                    disabled={isInert}
                    className="p-1.5 hover:bg-muted disabled:pointer-events-none focus-visible:ring-2 focus-visible:ring-ring"
                    title="Copy token"
                    aria-label="Copy token"
                  >
                    {copiedId === tok.id ? (
                      <Check className="h-3.5 w-3.5 text-emerald-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                    )}
                  </button>
                  {confirmDeleteId === tok.id ? (
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => {
                          deleteToken.mutate(tok.id)
                          setConfirmDeleteId(null)
                        }}
                        disabled={deleteToken.isPending}
                        className="bg-destructive px-2 py-0.5 text-[10px] font-semibold text-destructive-foreground focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        Del
                      </button>
                      <button
                        type="button"
                        onClick={() => setConfirmDeleteId(null)}
                        className="px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                        aria-label="Cancel delete"
                      >
                        ×
                      </button>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setConfirmDeleteId(tok.id)}
                      className="p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive focus-visible:ring-2 focus-visible:ring-ring"
                      title="Revoke token"
                      aria-label="Revoke token"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
