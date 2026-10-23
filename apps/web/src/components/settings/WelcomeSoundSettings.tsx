import { Loader2, Play, Trash2, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api } from '#/lib/api'
import { resolveAvatarUrl } from '#/lib/avatar-url'
import { getPublicSettings, type PublicSettings } from '#/lib/use-public-settings'
import { isGuestUser, useUserStore } from '#/lib/user.store'
import {
  checkWelcomeSoundFile,
  describeWelcomeSoundLimits,
  formatWelcomeSoundError,
  WELCOME_SOUND_PLAYBACK_GAIN,
} from '#/lib/welcome-sound'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { isMeetingTone, type SettingsPanelTone, sectionBorderClass } from './settingsPanelTone'

const WELCOME_SOUND_ENDPOINT = '/api/auth/me/welcome-sound'

type WelcomeSoundSettingsConfig = NonNullable<PublicSettings['welcomeSound']>

/**
 * Profile section for the welcome sound that plays for the room when this user joins.
 * Shown while the admin switch is on, and also while it is off if the user still has a
 * sound, so they can always remove it.
 */
export function WelcomeSoundSettings({ tone }: { tone: SettingsPanelTone }) {
  const meeting = isMeetingTone(tone)
  const user = useUserStore((s) => s.user)
  const setUser = useUserStore((s) => s.setUser)
  const [config, setConfig] = useState<WelcomeSoundSettingsConfig | null>(null)
  const [busy, setBusy] = useState<'upload' | 'remove' | null>(null)
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const previewRef = useRef<HTMLAudioElement | null>(null)

  useEffect(() => {
    let cancelled = false
    getPublicSettings()
      .then((settings) => {
        if (!cancelled) setConfig(settings.welcomeSound ?? null)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => () => previewRef.current?.pause(), [])

  const soundUrl = user?.welcomeSoundUrl
  const enabled = config?.enabled ?? false
  if (!user || isGuestUser(user) || !config || (!enabled && !soundUrl)) return null

  function applySoundUrl(welcomeSoundUrl: string | undefined) {
    if (user) setUser({ ...user, welcomeSoundUrl: welcomeSoundUrl || undefined })
  }

  function playPreview() {
    const source = resolveAvatarUrl(soundUrl)
    if (!source) return
    previewRef.current?.pause()
    const audio = new Audio(source)
    audio.volume = WELCOME_SOUND_PLAYBACK_GAIN
    previewRef.current = audio
    void audio.play().catch(() => setStatus({ type: 'error', message: 'Could not play the sound.' }))
  }

  async function handleFileSelected(file: File | undefined) {
    if (!file || !config) return
    const problem = checkWelcomeSoundFile(file, config.maxBytes)
    if (problem) {
      setStatus({ type: 'error', message: problem })
      return
    }
    setStatus(null)
    setBusy('upload')
    try {
      const form = new FormData()
      form.append('welcomeSound', file)
      const updated = await api.post<{ welcomeSoundUrl?: string }>(WELCOME_SOUND_ENDPOINT, form)
      applySoundUrl(updated.welcomeSoundUrl)
      setStatus({ type: 'success', message: 'Welcome sound saved.' })
    } catch (err) {
      setStatus({
        type: 'error',
        message: err instanceof Error ? formatWelcomeSoundError(err.message) : 'Failed to upload the sound.',
      })
    } finally {
      setBusy(null)
    }
  }

  async function handleRemove() {
    setStatus(null)
    setBusy('remove')
    try {
      previewRef.current?.pause()
      await api.delete(WELCOME_SOUND_ENDPOINT)
      applySoundUrl(undefined)
      setStatus({ type: 'success', message: 'Welcome sound removed.' })
    } catch (err) {
      setStatus({
        type: 'error',
        message: err instanceof Error ? formatWelcomeSoundError(err.message) : 'Failed to remove the sound.',
      })
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className={cn('space-y-3 border-b px-5 py-4', sectionBorderClass(meeting))}>
      <input
        ref={fileInputRef}
        type="file"
        accept="audio/mpeg,.mp3"
        className="hidden"
        onChange={(e) => {
          void handleFileSelected(e.target.files?.[0])
          e.target.value = ''
        }}
      />
      <div>
        <p className="text-sm font-medium">Welcome sound</p>
        <p className={cn('mt-0.5 text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
          {enabled
            ? `Plays for everyone in the room when you join. ${describeWelcomeSoundLimits(config)}. You can change it twice a day.`
            : 'Welcome sounds are turned off on this server. You can still remove yours.'}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {soundUrl && (
          <Button type="button" variant="outline" size="sm" className="gap-1.5" onClick={playPreview}>
            <Play className="h-3 w-3" />
            Play
          </Button>
        )}
        {enabled && (
          <Button
            type="button"
            variant={soundUrl ? 'outline' : 'default'}
            size="sm"
            className="gap-1.5"
            disabled={busy !== null}
            onClick={() => fileInputRef.current?.click()}
          >
            {busy === 'upload' ? <Loader2 className="h-3 w-3 animate-spin" /> : <Upload className="h-3 w-3" />}
            {soundUrl ? 'Replace' : 'Upload MP3'}
          </Button>
        )}
        {soundUrl && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="gap-1.5"
            disabled={busy !== null}
            onClick={() => void handleRemove()}
          >
            {busy === 'remove' ? <Loader2 className="h-3 w-3 animate-spin" /> : <Trash2 className="h-3 w-3" />}
            Remove
          </Button>
        )}
      </div>
      {status && <Alert {...status} />}
    </div>
  )
}
