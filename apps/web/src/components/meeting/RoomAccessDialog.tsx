import { Globe, Loader2, Lock } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getErrorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function RoomAccessDialog({ open, onOpenChange }: Props) {
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
      <DialogContent className="meet-dialog max-w-[min(92vw,360px)] gap-0 p-0 shadow-2xl">
        <DialogHeader className="border-b border-white/[0.08] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-white/90">Room access</DialogTitle>
          <DialogDescription className="text-white/50">
            {canManageRoomAccess ? 'Choose who can join this meeting.' : 'Only the room host can change access.'}
          </DialogDescription>
        </DialogHeader>

        <div className="px-4 py-4 space-y-3">
          <p className="font-mono text-sm text-white/70">{roomName}</p>
          <RadioGroup
            value={selected ? 'public' : 'private'}
            onValueChange={(v) => canManageRoomAccess && setSelected(v === 'public')}
            className="grid grid-cols-2 gap-2"
            disabled={!canManageRoomAccess}
          >
            <label
              className={cn(
                'flex flex-col items-start gap-1.5 rounded-lg border p-3 text-left transition-colors',
                !selected
                  ? 'border-primary/40 bg-primary/10'
                  : 'border-white/[0.08] bg-white/[0.03] text-white/50',
                canManageRoomAccess && 'cursor-pointer hover:border-white/20',
                !canManageRoomAccess && 'opacity-70 cursor-default',
              )}
            >
              <RadioGroupItem value="private" className="sr-only" disabled={!canManageRoomAccess} />
              <div className="flex items-center gap-2">
                <Lock className={cn('h-4 w-4', !selected ? 'text-primary' : 'text-white/40')} />
                <span className={cn('text-sm font-medium', !selected ? 'text-white' : 'text-white/50')}>Private</span>
              </div>
              <span className="text-[11px] text-white/40">Invite only</span>
            </label>
            <label
              className={cn(
                'flex flex-col items-start gap-1.5 rounded-lg border p-3 text-left transition-colors',
                selected
                  ? 'border-primary/40 bg-primary/10'
                  : 'border-white/[0.08] bg-white/[0.03] text-white/50',
                canManageRoomAccess && 'cursor-pointer hover:border-white/20',
                !canManageRoomAccess && 'opacity-70 cursor-default',
              )}
            >
              <RadioGroupItem value="public" className="sr-only" disabled={!canManageRoomAccess} />
              <div className="flex items-center gap-2">
                <Globe className={cn('h-4 w-4', selected ? 'text-primary' : 'text-white/40')} />
                <span className={cn('text-sm font-medium', selected ? 'text-white' : 'text-white/50')}>Public</span>
              </div>
              <span className="text-[11px] text-white/40">Anyone with the link</span>
            </label>
          </RadioGroup>
        </div>

        <DialogFooter className="border-t border-white/[0.08] px-4 py-3 sm:justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
            className="text-white/60 hover:text-white hover:bg-white/10"
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
