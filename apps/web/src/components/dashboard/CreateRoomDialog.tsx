import {
  AlertCircle,
  ArrowRight,
  Globe,
  Loader2,
  Lock,
  MessageSquare,
  Mic,
  Minus,
  Pin,
  Plus,
  ShieldCheck,
  UserCheck,
  Video,
} from 'lucide-react'
import { useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getErrorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

export interface RoomSettings {
  allowChat: boolean
  allowVideo: boolean
  allowAudio: boolean
  requireApproval: boolean
  e2ee: boolean
  isPersistent: boolean
  // TODO oncoming feature
  recordingsAllowed?: boolean
}

export interface CreateRoomData {
  name?: string
  isPublic: boolean
  maxParticipants: number
  settings: RoomSettings
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreate: (data: CreateRoomData) => Promise<void>
  isAdmin?: boolean
}

const DEFAULT_SETTINGS: RoomSettings = {
  allowChat: true,
  allowVideo: false,
  allowAudio: true,
  requireApproval: false,
  e2ee: false,
  isPersistent: false,
  // TODO oncoming feature
  recordingsAllowed: true,
}

const FEATURES = [
  { key: 'allowAudio' as const, icon: Mic, label: 'Audio' },
  { key: 'allowVideo' as const, icon: Video, label: 'Video' },
  { key: 'allowChat' as const, icon: MessageSquare, label: 'Chat' },
  { key: 'e2ee' as const, icon: ShieldCheck, label: 'E2E' },
  { key: 'requireApproval' as const, icon: UserCheck, label: 'Gate' },
  { key: 'isPersistent' as const, icon: Pin, label: 'Persistent', adminOnly: true },
]

export function CreateRoomDialog({ open, onOpenChange, onCreate, isAdmin }: Props) {
  const [isLoading, setIsLoading] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [maxParticipants, setMaxParticipants] = useState(20)
  const [isPublic, setIsPublic] = useState(false)
  const [settings, setSettings] = useState<RoomSettings>(DEFAULT_SETTINGS)
  const [roomHost, setRoomHost] = useState('localhost:7070')

  useEffect(() => {
    setRoomHost(window.location.host)
  }, [])

  function toggle(key: keyof RoomSettings) {
    setSettings((s) => ({ ...s, [key]: !s[key] }))
  }

  function resetForm() {
    setIsLoading(false)
    setCreateError(null)
    setName('')
    setMaxParticipants(20)
    setIsPublic(false)
    setSettings(DEFAULT_SETTINGS)
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) resetForm()
    onOpenChange(nextOpen)
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setIsLoading(true)
    setCreateError(null)
    try {
      await onCreate({ name: name.trim() || undefined, isPublic, maxParticipants, settings })
      resetForm()
      onOpenChange(false)
    } catch (err) {
      setCreateError(getErrorMessage(err, 'Failed to create room'))
    } finally {
      setIsLoading(false)
    }
  }

  const displaySlug = name.trim() || 'auto-generated'

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="gap-0 overflow-hidden border p-0 max-w-[calc(100vw-2rem)] sm:max-w-md">
        <DialogTitle className="sr-only">Create Room</DialogTitle>
        <DialogDescription className="sr-only">Configure and create a new room</DialogDescription>
        <form onSubmit={handleSubmit}>
          {/* ── Name section ── */}
          <div className="px-6 pt-6 pb-5">
            <Label
              htmlFor="room-name"
              className="text-[10px] tracking-widest uppercase font-semibold text-muted-foreground/50"
            >
              Name
            </Label>
            <Input
              id="room-name"
              value={name}
              onChange={(e) => {
                const v = e.target.value
                  .toLowerCase()
                  .replace(/\s+/g, '-')
                  .replace(/[^a-z0-9-]/g, '')
                setName(v)
              }}
              placeholder="my-room"
              autoComplete="off"
              spellCheck={false}
              autoFocus
              className="mt-2 font-mono text-xl font-semibold tracking-tight placeholder:text-muted-foreground/30 px-0"
            />
            <p className="mt-1.5 font-mono text-[11px] text-muted-foreground/50">
              {roomHost}/m/{displaySlug}
            </p>
            {!name.trim() && (
              <p className="mt-0.5 text-[10px] text-muted-foreground/40">Leave blank to auto-generate a name</p>
            )}
          </div>

          {/* ── Access section ── */}
          <div className="border-t px-6 py-5">
            <span className="text-[10px] tracking-widest uppercase font-semibold text-muted-foreground/50">Access</span>
            <div className="mt-3">
              <RadioGroup
                value={isPublic ? 'public' : 'private'}
                onValueChange={(v) => setIsPublic(v === 'public')}
                className="grid grid-cols-2 gap-3"
              >
                {/* biome-ignore lint/a11y/noLabelWithoutControl: RadioGroupItem renders native radio button */}
                <label
                  className={cn(
                    'flex flex-col items-start gap-1 border p-3 text-left transition-colors cursor-pointer',
                    !isPublic
                      ? 'border-primary bg-primary/5'
                      : 'border bg-background text-muted-foreground hover:border-foreground/20',
                  )}
                >
                  <RadioGroupItem value="private" className="sr-only" />
                  <div className="flex items-center gap-2">
                    <Lock className={cn('h-4 w-4', !isPublic ? 'text-primary' : 'text-muted-foreground/60')} />
                    <span className={cn('text-sm font-medium', !isPublic ? 'text-primary' : 'text-muted-foreground')}>
                      Private
                    </span>
                  </div>
                  <span className="text-[11px] text-muted-foreground/60">Invite only</span>
                </label>
                {/* biome-ignore lint/a11y/noLabelWithoutControl: RadioGroupItem renders native radio button */}
                <label
                  className={cn(
                    'flex flex-col items-start gap-1 border p-3 text-left transition-colors cursor-pointer',
                    isPublic
                      ? 'border-primary bg-primary/5'
                      : 'border bg-background text-muted-foreground hover:border-foreground/20',
                  )}
                >
                  <RadioGroupItem value="public" className="sr-only" />
                  <div className="flex items-center gap-2">
                    <Globe className={cn('h-4 w-4', isPublic ? 'text-primary' : 'text-muted-foreground/60')} />
                    <span className={cn('text-sm font-medium', isPublic ? 'text-primary' : 'text-muted-foreground')}>
                      Public
                    </span>
                  </div>
                  <span className="text-[11px] text-muted-foreground/60">Anyone with link</span>
                </label>
              </RadioGroup>
            </div>
          </div>

          {/* ── Capacity section ── */}
          <div className="border-t px-6 py-5">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-[10px] tracking-widest uppercase font-semibold text-muted-foreground/50">
                  Capacity
                </span>
                <p className="mt-0.5 text-[11px] text-muted-foreground/60">{maxParticipants} seats</p>
              </div>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => setMaxParticipants((p) => Math.max(2, p - 5))}
                  className="h-9 w-9"
                >
                  <Minus className="h-3.5 w-3.5" />
                </Button>
                <span className="w-10 text-center font-mono text-base font-semibold tabular-nums">
                  {maxParticipants}
                </span>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => setMaxParticipants((p) => Math.min(500, p + 5))}
                  className="h-9 w-9"
                >
                  <Plus className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </div>

          {/* ── Features section ── */}
          <div className="border-t px-6 py-5">
            <span className="text-[10px] tracking-widest uppercase font-semibold text-muted-foreground/50">
              Features
            </span>
            <div className="mt-3 flex flex-wrap gap-2">
              {FEATURES.filter((f) => !f.adminOnly || isAdmin).map(({ key, icon: Icon, label }) => {
                const active = settings[key]
                return (
                  <Button
                    key={key}
                    type="button"
                    variant={active ? 'default' : 'outline'}
                    onClick={() => toggle(key)}
                    className={cn(
                      'gap-2 px-3.5 py-2 text-xs font-medium',
                      active
                        ? 'border-primary/30 bg-primary/10 text-primary'
                        : 'text-muted-foreground hover:text-foreground',
                    )}
                  >
                    <Icon className="h-3.5 w-3.5" />
                    {label}
                  </Button>
                )
              })}
            </div>
          </div>

          {/* ── Error ── */}
          {createError && (
            <div className="mx-6 mb-4 flex items-center gap-2 border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" />
              {createError}
            </div>
          )}

          {/* ── Action ── */}
          <div className="border-t px-6 py-4">
            <Button type="submit" disabled={isLoading} className="w-full">
              {isLoading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" /> Creating...
                </>
              ) : (
                <>
                  Create & join <ArrowRight className="h-4 w-4" />
                </>
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
