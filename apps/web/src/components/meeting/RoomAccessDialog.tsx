import { Globe, Loader2, Lock } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getErrorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Stack above expanded stage chrome (z-200) and elevated left docks (z-250). */
  aboveElevatedDock?: boolean
}

export function RoomAccessDialog({ open, onOpenChange, aboveElevatedDock = false }: Props) {
  const { roomId, roomName, isPublic, canManageRoomAccess, setRoomIsPublic } = useMeetingRoomContext()
  const [selected, setSelected] = useState(isPublic)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) setSelected(isPublic)
  }, [open, isPublic])

  async function handleSave() {
    if (!canManageRoomAccess || selected === isPublic) {
      onOpenChange(false)
      return
    }
    setSaving(true)
    try {
      await api.put(`/api/room/${roomId}/settings`, { isPublic: selected })
      setRoomIsPublic(selected)
      toast.success(selected ? 'Room is now public' : 'Room is now private')
      onOpenChange(false)
    } catch (err) {
      toast.error(getErrorMessage(err, 'Failed to update room access'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        aboveElevatedDock={aboveElevatedDock}
        className="meet-dialog max-w-[min(360px,calc(var(--app-width,100svw)-2rem))] gap-0 p-0 shadow-2xl"
      >
        <DialogHeader className="border-b border-[var(--meet-border)] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-[var(--meet-fg-strong)]">Room access</DialogTitle>
          <DialogDescription className="text-[var(--meet-fg-muted)]">
            {canManageRoomAccess ? 'Choose who can join this meeting.' : 'Only the room host can change access.'}
          </DialogDescription>
        </DialogHeader>

        <div className="px-4 py-4 space-y-3">
          <p className="font-mono text-sm text-[var(--meet-fg)]">{roomName}</p>
          <RadioGroup
            value={selected ? 'public' : 'private'}
            onValueChange={(v) => canManageRoomAccess && setSelected(v === 'public')}
            className="grid grid-cols-2 gap-2"
            disabled={!canManageRoomAccess}
          >
            <label
              htmlFor="room-access-private"
              className={cn(
                'flex flex-col items-start gap-1.5 rounded-lg border p-3 text-left transition-colors',
                !selected
                  ? 'border-primary/40 bg-primary/10 text-[var(--meet-fg-strong)]'
                  : 'border-[var(--meet-border)] bg-[var(--meet-surface-muted)] text-[var(--meet-fg-muted)]',
                canManageRoomAccess &&
                  'cursor-pointer hover:border-[color-mix(in_oklab,var(--meet-accent)_35%,transparent)]',
                !canManageRoomAccess && 'cursor-default opacity-70',
              )}
            >
              <RadioGroupItem
                id="room-access-private"
                value="private"
                className="sr-only"
                disabled={!canManageRoomAccess}
              />
              <div className="flex items-center gap-2">
                <Lock className={cn('h-4 w-4', !selected ? 'text-primary' : 'text-[var(--meet-fg-subtle)]')} />
                <span
                  className={cn(
                    'text-sm font-medium',
                    !selected ? 'text-[var(--meet-fg-strong)]' : 'text-[var(--meet-fg-muted)]',
                  )}
                >
                  Private
                </span>
              </div>
              <span className="text-[11px] text-[var(--meet-fg-muted)]">Invite only</span>
            </label>
            <label
              htmlFor="room-access-public"
              className={cn(
                'flex flex-col items-start gap-1.5 rounded-lg border p-3 text-left transition-colors',
                selected
                  ? 'border-primary/40 bg-primary/10 text-[var(--meet-fg-strong)]'
                  : 'border-[var(--meet-border)] bg-[var(--meet-surface-muted)] text-[var(--meet-fg-muted)]',
                canManageRoomAccess &&
                  'cursor-pointer hover:border-[color-mix(in_oklab,var(--meet-accent)_35%,transparent)]',
                !canManageRoomAccess && 'cursor-default opacity-70',
              )}
            >
              <RadioGroupItem
                id="room-access-public"
                value="public"
                className="sr-only"
                disabled={!canManageRoomAccess}
              />
              <div className="flex items-center gap-2">
                <Globe className={cn('h-4 w-4', selected ? 'text-primary' : 'text-[var(--meet-fg-subtle)]')} />
                <span
                  className={cn(
                    'text-sm font-medium',
                    selected ? 'text-[var(--meet-fg-strong)]' : 'text-[var(--meet-fg-muted)]',
                  )}
                >
                  Public
                </span>
              </div>
              <span className="text-[11px] text-[var(--meet-fg-muted)]">Anyone with the link</span>
            </label>
          </RadioGroup>
        </div>

        <DialogFooter className="gap-2 border-t border-[var(--meet-border)] px-4 py-3 sm:justify-end">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            className="text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)]"
          >
            {canManageRoomAccess ? 'Cancel' : 'Close'}
          </Button>
          {canManageRoomAccess && (
            <Button type="button" onClick={handleSave} disabled={saving || selected === isPublic}>
              {saving ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" /> Saving…
                </>
              ) : (
                'Save'
              )}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
