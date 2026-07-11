import {
  AlertCircle,
  Check,
  Globe,
  Loader2,
  Lock,
  MessageSquare,
  Mic,
  Minus,
  Plus,
  ShieldCheck,
  UserCheck,
  Users,
  Video,
} from 'lucide-react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { getErrorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

interface RoomSettings {
  allowChat: boolean
  allowVideo: boolean
  allowAudio: boolean
  requireApproval: boolean
  e2ee: boolean
}

interface Room {
  id: string
  name: string
  isPublic: boolean
  maxParticipants: number
  settings: RoomSettings
}

interface Props {
  room: Room
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (
    roomId: string,
    data: { isPublic: boolean; maxParticipants: number; settings: RoomSettings },
  ) => Promise<void>
}

const FEATURES = [
  { key: 'allowAudio' as const, icon: Mic, label: 'Audio' },
  { key: 'allowVideo' as const, icon: Video, label: 'Video' },
  { key: 'allowChat' as const, icon: MessageSquare, label: 'Chat' },
  { key: 'e2ee' as const, icon: ShieldCheck, label: 'E2E' },
  { key: 'requireApproval' as const, icon: UserCheck, label: 'Gate' },
]

export function RoomSettingsDialog({ room, open, onOpenChange, onSave }: Props) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [isPublic, setIsPublic] = useState(room.isPublic)
  const [maxParticipants, setMaxParticipants] = useState(room.maxParticipants)
  const [settings, setSettings] = useState<RoomSettings>({ ...room.settings })

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      setIsPublic(room.isPublic)
      setMaxParticipants(room.maxParticipants)
      setSettings({ ...room.settings })
      setError(null)
    }
    onOpenChange(nextOpen)
  }

  function toggle(key: keyof RoomSettings) {
    setSettings((s) => ({ ...s, [key]: !s[key] }))
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setIsLoading(true)
    setError(null)
    try {
      await onSave(room.id, { isPublic, maxParticipants, settings })
      onOpenChange(false)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to save settings'))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="gap-0 overflow-hidden border p-0 max-w-[calc(var(--app-width,100svw)-2rem)] sm:max-w-sm">
        <DialogTitle className="sr-only">Room Settings</DialogTitle>
        <DialogDescription className="sr-only">Configure room visibility, capacity, and features</DialogDescription>
        <form onSubmit={handleSubmit}>
          {/* Room name as header */}
          <div className="px-5 pt-5 pb-4">
            <p className="font-mono text-lg font-semibold tracking-tight">{room.name}</p>
            <p className="mt-1 text-[11px] text-muted-foreground/50">Room settings</p>
          </div>

          {/* Visibility + Capacity — single row */}
          <div className="flex flex-wrap items-center gap-3 border-t px-5 py-3">
            <div className="flex items-center gap-0.5 border bg-background p-0.5">
              <Button
                type="button"
                variant={!isPublic ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => setIsPublic(false)}
                className={cn(!isPublic && 'bg-primary/10 text-primary')}
              >
                <Lock className="h-3 w-3" />
                Private
              </Button>
              <Button
                type="button"
                variant={isPublic ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => setIsPublic(true)}
                className={cn(isPublic && 'bg-primary/10 text-primary')}
              >
                <Globe className="h-3 w-3" />
                Public
              </Button>
            </div>

            <div className="ml-auto flex items-center gap-2">
              <Users className="h-3.5 w-3.5 text-muted-foreground/50" />
              <Button
                variant="outline"
                size="icon"
                onClick={() => setMaxParticipants((p) => Math.max(2, p - 5))}
                className="h-8 w-8"
              >
                <Minus className="h-3 w-3" />
              </Button>
              <span className="w-6 text-center font-mono text-xs font-medium">{maxParticipants}</span>
              <Button
                variant="outline"
                size="icon"
                onClick={() => setMaxParticipants((p) => Math.min(500, p + 5))}
                className="h-8 w-8"
              >
                <Plus className="h-3 w-3" />
              </Button>
            </div>
          </div>

          {/* Feature chips */}
          <div className="flex flex-wrap gap-1.5 border-t px-5 py-3">
            {FEATURES.map(({ key, icon: Icon, label }) => {
              const active = settings[key]
              return (
                <Badge
                  key={key}
                  variant={active ? 'default' : 'outline'}
                  className={cn(
                    'cursor-pointer gap-1.5 px-3 py-1.5 text-xs font-medium',
                    active
                      ? 'border-primary/30 bg-primary/10 text-primary hover:bg-primary/15'
                      : 'border-transparent bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground',
                  )}
                  onClick={() => toggle(key)}
                >
                  <Icon className="h-3 w-3" />
                  {label}
                </Badge>
              )
            })}
          </div>

          {/* Error */}
          {error && (
            <div className="mx-5 mb-3 flex items-center gap-2 border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" />
              {error}
            </div>
          )}

          {/* Action */}
          <div className="border-t px-5 py-3">
            <Button type="submit" disabled={isLoading} className="w-full">
              {isLoading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" /> Saving...
                </>
              ) : (
                <>
                  Save changes <Check className="h-3.5 w-3.5" />
                </>
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
