import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Play, Trash2, Volume2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '#/lib/api'
import { resolveAvatarUrl } from '#/lib/avatar-url'
import { getErrorMessage } from '#/lib/errors'
import { WELCOME_SOUND_PLAYBACK_GAIN } from '#/lib/welcome-sound'
import { Button } from '@/components/ui/button'
import { AlertConfirmDialog } from './AlertConfirmDialog'

/**
 * Lets an admin hear a user's welcome sound and clear it, for moderation.
 * Renders nothing when the user has no welcome sound.
 */
export function AdminUserWelcomeSound({
  userId,
  welcomeSoundUrl,
  readOnly,
}: {
  userId: string
  welcomeSoundUrl?: string
  readOnly: boolean
}) {
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const previewRef = useRef<HTMLAudioElement | null>(null)

  useEffect(() => () => previewRef.current?.pause(), [])

  const clearSound = useMutation({
    mutationFn: () => api.delete(`/api/admin/users/${userId}/welcome-sound`),
    onSuccess: () => {
      previewRef.current?.pause()
      toast.success('Welcome sound cleared')
      setConfirmOpen(false)
      queryClient.invalidateQueries({ queryKey: ['admin', 'user', userId] })
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to clear the welcome sound')),
  })

  const source = resolveAvatarUrl(welcomeSoundUrl)
  if (!source) return null

  function playPreview() {
    if (!source) return
    previewRef.current?.pause()
    const audio = new Audio(source)
    audio.volume = WELCOME_SOUND_PLAYBACK_GAIN
    previewRef.current = audio
    void audio.play().catch(() => toast.error('Could not play the sound'))
  }

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-xl border bg-card px-5 py-4">
      <Volume2 className="h-4 w-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">Welcome sound</p>
        <p className="text-xs text-muted-foreground">Plays for the room when this user joins a meeting.</p>
      </div>
      <Button type="button" variant="outline" size="sm" className="gap-1.5" onClick={playPreview}>
        <Play className="h-3 w-3" />
        Play
      </Button>
      {!readOnly && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-1.5 text-destructive hover:bg-destructive/10"
          onClick={() => setConfirmOpen(true)}
        >
          <Trash2 className="h-3 w-3" />
          Clear
        </Button>
      )}
      <AlertConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Clear welcome sound"
        description="This deletes the user's welcome sound. They can upload a new one, which counts against their two changes a day."
        confirmLabel="Clear sound"
        onConfirm={() => clearSound.mutate()}
        isLoading={clearSound.isPending}
      />
    </div>
  )
}
