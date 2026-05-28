import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Section, type SystemSettings } from '#/components/admin/settings'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface WebhookData {
  id: string
  name: string
  url: string
  secret: string
  events: string[]
  isActive: boolean
  lastSeen: string | null
  createdBy: string
  createdAt: string
  updatedAt: string
}

const WEBHOOK_EVENTS = [
  'room.created',
  'room.ended',
  'participant.joined',
  /* TODO oncoming feature */ 'recording.completed',
] as const

function truncateUrl(url: string, max = 40) {
  return url.length > max ? url.slice(0, max) + '…' : url
}

interface TestResult {
  status: string
  latencyMs?: number
  httpStatus?: number
}

export function WebhookSection(_props: {
  settings: SystemSettings
  setSettings?: (patch: Partial<SystemSettings>) => void
  saving?: boolean
}) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<WebhookData | null>(null)
  const [creating, setCreating] = useState(false)
  const [testResult, setTestResult] = useState<Record<string, TestResult>>({})

  const { data, isLoading } = useQuery<{ webhooks: WebhookData[] }>({
    queryKey: ['admin', 'webhooks'],
    queryFn: () => api.get<{ webhooks: WebhookData[] }>('/api/admin/webhooks'),
  })
  const webhooks = data?.webhooks ?? []

  const deleteMutation = useMutation<void, Error, string>({
    mutationFn: (id) => api.delete(`/api/admin/webhooks/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'webhooks'] })
      toast.success('Webhook deleted')
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to delete webhook')),
  })

  const toggleMutation = useMutation<void, Error, { id: string; isActive: boolean }>({
    mutationFn: ({ id, isActive }) => api.put(`/api/admin/webhooks/${id}`, { isActive }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'webhooks'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to toggle webhook')),
  })

  const testMutation = useMutation<TestResult, Error, string>({
    mutationFn: (id) => api.post<TestResult>(`/api/admin/webhooks/${id}/test`),
    onSuccess: (data, id) => {
      setTestResult((prev) => ({ ...prev, [id]: data }))
      if (data.status === 'success') toast.success('Webhook test succeeded')
      else toast.warning(`Webhook test: ${data.status}`)
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Webhook test failed')),
  })

  const rotateMutation = useMutation<{ secret: string }, Error, string>({
    mutationFn: (id) => api.post<{ secret: string }>(`/api/admin/webhooks/${id}/rotate-secret`),
    onSuccess: (data) => {
      toast.success('Secret rotated', {
        description: `New secret: ${data.secret}`,
        duration: 10000,
      })
      queryClient.invalidateQueries({ queryKey: ['admin', 'webhooks'] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to rotate secret')),
  })

  return (
    <>
      <Section title="Webhook Endpoints" description="Outbound HTTP callbacks for room lifecycle events.">
        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : webhooks.length === 0 ? (
          <p className="text-xs text-muted-foreground py-4">No webhooks configured.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b text-muted-foreground">
                  <th className="px-2 py-1.5 text-left font-normal">Name</th>
                  <th className="px-2 py-1.5 text-left font-mono font-normal">URL</th>
                  <th className="px-2 py-1.5 text-left font-normal">Events</th>
                  <th className="px-2 py-1.5 text-left font-normal">Active</th>
                  <th className="px-2 py-1.5 text-right font-normal">Actions</th>
                </tr>
              </thead>
              <tbody>
                {webhooks.map((wh) => (
                  <tr key={wh.id} className="border-b last:border-0">
                    <td className="px-2 py-1.5">{wh.name}</td>
                    <td className="px-2 py-1.5 font-mono text-muted-foreground">{truncateUrl(wh.url)}</td>
                    <td className="px-2 py-1.5">
                      <div className="flex flex-wrap gap-1">
                        {wh.events.map((e) => (
                          <span
                            key={e}
                            className="inline-flex items-center rounded-md border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
                          >
                            {e}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="px-2 py-1.5">
                      <Switch
                        checked={wh.isActive}
                        onCheckedChange={(v) => toggleMutation.mutate({ id: wh.id, isActive: v })}
                      />
                    </td>
                    <td className="px-2 py-1.5 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="sm" onClick={() => setEditing(wh)}>
                          Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={testMutation.isPending}
                          onClick={() => {
                            setTestResult((prev) => ({ ...prev, [wh.id]: { status: 'pending' } }))
                            testMutation.mutate(wh.id)
                          }}
                        >
                          Test
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => rotateMutation.mutate(wh.id)}>
                          <RefreshCw className="h-3 w-3" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive"
                          onClick={() => deleteMutation.mutate(wh.id)}
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                      {testResult[wh.id] && (
                        <div className="mt-0.5 text-[10px] text-muted-foreground">
                          {testResult[wh.id].status === 'success'
                            ? `✓ ${testResult[wh.id].latencyMs}ms`
                            : `✗ ${testResult[wh.id].status}`}
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <Button variant="outline" size="sm" className="mt-3" onClick={() => setCreating(true)}>
          <Plus className="h-3 w-3" />
          Add Webhook
        </Button>
      </Section>

      <WebhookDialog
        open={creating || editing !== null}
        webhook={editing}
        onClose={() => {
          setCreating(false)
          setEditing(null)
        }}
        onSaved={() => queryClient.invalidateQueries({ queryKey: ['admin', 'webhooks'] })}
      />
    </>
  )
}

function WebhookDialog({
  open,
  webhook,
  onClose,
  onSaved,
}: {
  open: boolean
  webhook: WebhookData | null
  onClose: () => void
  onSaved: () => void
}) {
  const isEdit = webhook !== null
  const [name, setName] = useState(webhook?.name ?? '')
  const [url, setUrl] = useState(webhook?.url ?? '')
  const [events, setEvents] = useState<string[]>(webhook?.events ?? [])
  const [newSecret, setNewSecret] = useState<string | null>(null)

  const createMutation = useMutation<WebhookData, Error, { name: string; url: string; events: string[] }>({
    mutationFn: (data) => api.post<WebhookData>('/api/admin/webhooks', data),
    onSuccess: (data) => {
      toast.success('Webhook created')
      setNewSecret(data.secret)
      onSaved()
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to create webhook')),
  })

  const updateMutation = useMutation<WebhookData, Error, { name?: string; url?: string; events?: string[] }>({
    mutationFn: (data) => api.put<WebhookData>(`/api/admin/webhooks/${webhook!.id}`, data),
    onSuccess: () => {
      toast.success('Webhook updated')
      onSaved()
      onClose()
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update webhook')),
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  function handleSave() {
    if (!name.trim() || !url.trim()) return
    if (isEdit) {
      updateMutation.mutate({ name, url, events })
    } else {
      createMutation.mutate({ name, url, events })
    }
  }

  function toggleEvent(event: string) {
    setEvents((prev) => (prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]))
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose()
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Webhook' : 'Add Webhook'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label className="text-xs">Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Webhook"
              className="h-8 text-xs"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">URL</Label>
            <Input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/webhook"
              className="h-8 font-mono text-xs"
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs">Events</Label>
            <div className="flex flex-wrap gap-2">
              {WEBHOOK_EVENTS.map((event) => {
                const eventId = `webhook-event-${event.replace(/\./g, '-')}`
                return (
                  <label key={event} htmlFor={eventId} className="flex items-center gap-1.5 text-xs cursor-pointer">
                    <Switch id={eventId} checked={events.includes(event)} onCheckedChange={() => toggleEvent(event)} />
                    <span className="text-muted-foreground">{event}</span>
                  </label>
                )
              })}
            </div>
          </div>

          {newSecret && (
            <div className="rounded-md border border-emerald-500/30 bg-emerald-500/5 p-3">
              <p className="text-[10px] font-medium text-emerald-400">Secret (save this now — won't be shown again)</p>
              <p className="mt-1 font-mono text-xs text-emerald-300 break-all">{newSecret}</p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleSave} disabled={isPending || !name.trim() || !url.trim()}>
            {isPending && <Loader2 className="mr-1.5 h-3 w-3 animate-spin" />}
            {isEdit ? 'Save' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
