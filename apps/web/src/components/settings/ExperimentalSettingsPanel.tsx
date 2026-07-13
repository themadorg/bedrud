import { useMutation, useQuery } from '@tanstack/react-query'
import { Check, Film, FlaskConical, Loader2, Package, PenLine } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { patchUserPreferences } from '#/lib/user-preferences'
import { fetchWebxdcConfig } from '@/components/meeting/webxdc/webxdcApi'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { isMeetingTone, panelSurfaceClass, type SettingsPanelTone } from './settingsPanelTone'

export function ExperimentalSettingsPanel({ tone = 'default' }: { tone?: SettingsPanelTone }) {
  const meeting = isMeetingTone(tone)
  const whiteboardEnabled = useExperimentalPreferencesStore((s) => s.whiteboardEnabled)
  const youtubeEnabled = useExperimentalPreferencesStore((s) => s.youtubeEnabled)
  const webxdcEnabled = useExperimentalPreferencesStore((s) => s.webxdcEnabled)
  const setWhiteboardEnabled = useExperimentalPreferencesStore((s) => s.setWhiteboardEnabled)
  const setYoutubeEnabled = useExperimentalPreferencesStore((s) => s.setYoutubeEnabled)
  const setWebxdcEnabled = useExperimentalPreferencesStore((s) => s.setWebxdcEnabled)

  const serverWebxdc = useQuery({
    queryKey: ['webxdc-config'],
    queryFn: fetchWebxdcConfig,
    staleTime: 60_000,
  })
  const serverWebxdcOn = serverWebxdc.data?.enabled === true

  const experimentalPrefsRef = useRef({ whiteboardEnabled, youtubeEnabled, webxdcEnabled })
  experimentalPrefsRef.current = { whiteboardEnabled, youtubeEnabled, webxdcEnabled }

  const syncMutation = useMutation({
    mutationFn: () => patchUserPreferences({ experimental: experimentalPrefsRef.current }),
  })
  const mutateRef = useRef(syncMutation.mutate)
  mutateRef.current = syncMutation.mutate

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — save on any toggle change
  useEffect(() => {
    const timer = setTimeout(() => mutateRef.current(), 1000)
    return () => clearTimeout(timer)
  }, [whiteboardEnabled, youtubeEnabled, webxdcEnabled])

  useEffect(() => {
    return () => {
      void patchUserPreferences({ experimental: experimentalPrefsRef.current })
    }
  }, [])

  useEffect(() => {
    if (serverWebxdc.isSuccess && !serverWebxdcOn && webxdcEnabled) {
      setWebxdcEnabled(false)
    }
  }, [serverWebxdc.isSuccess, serverWebxdcOn, webxdcEnabled, setWebxdcEnabled])

  const syncStatus = syncMutation.isPending
    ? 'saving'
    : syncMutation.isError
      ? 'error'
      : syncMutation.isSuccess
        ? 'saved'
        : 'idle'

  return (
    <div className={panelSurfaceClass(tone)}>
      <div className={cn('border-b px-5 py-3', meeting && 'border-white/[0.08]')}>
        <div className="flex items-center gap-2">
          <FlaskConical className={cn('h-4 w-4', meeting ? 'text-amber-400' : 'text-amber-600')} aria-hidden />
          <div>
            <p className="text-sm font-medium">Experimental</p>
            <p className={cn('text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              Unstable features — bugs may occur
            </p>
          </div>
        </div>
      </div>

      <div
        className={cn('flex items-center justify-between gap-4 border-b px-5 py-4', meeting && 'border-white/[0.08]')}
      >
        <div className="flex min-w-0 items-start gap-3">
          <PenLine className={cn('mt-0.5 h-4 w-4 shrink-0', meeting ? 'text-white/50' : 'text-muted-foreground')} />
          <div className="min-w-0">
            <p className="text-sm font-medium">Shared whiteboard</p>
            <p className={cn('text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              Show the whiteboard in meeting controls. You will still be asked to accept before each session.
            </p>
          </div>
        </div>
        <Switch checked={whiteboardEnabled} onCheckedChange={setWhiteboardEnabled} />
      </div>

      <div
        className={cn('flex items-center justify-between gap-4 border-b px-5 py-4', meeting && 'border-white/[0.08]')}
      >
        <div className="flex min-w-0 items-start gap-3">
          <Film className={cn('mt-0.5 h-4 w-4 shrink-0', meeting ? 'text-white/50' : 'text-muted-foreground')} />
          <div className="min-w-0">
            <p className="text-sm font-medium">YouTube watch party</p>
            <p className={cn('text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              Share a YouTube video with everyone in the room in sync.
            </p>
          </div>
        </div>
        <Switch checked={youtubeEnabled} onCheckedChange={setYoutubeEnabled} />
      </div>

      <div className="flex items-center justify-between gap-4 px-5 py-4">
        <div className="flex min-w-0 items-start gap-3">
          <Package className={cn('mt-0.5 h-4 w-4 shrink-0', meeting ? 'text-white/50' : 'text-muted-foreground')} />
          <div className="min-w-0">
            <p className="text-sm font-medium">WebXDC mini-apps</p>
            <p className={cn('text-xs', meeting ? 'text-white/50' : 'text-muted-foreground')}>
              {serverWebxdc.isLoading
                ? 'Checking server support…'
                : serverWebxdcOn
                  ? `Sandboxed .xdc apps in meetings (base: ${serverWebxdc.data?.baseDomain ?? 'configured'}). Server needs domain + wildcard DNS.`
                  : 'Unavailable: enable webxdc in server config (domain + baseDomain). Then turn this on.'}
            </p>
          </div>
        </div>
        <Switch
          checked={webxdcEnabled && serverWebxdcOn}
          disabled={!serverWebxdcOn}
          onCheckedChange={setWebxdcEnabled}
        />
      </div>

      {syncStatus !== 'idle' && (
        <div
          className={cn('flex items-center justify-end gap-1.5 border-t px-5 py-2.5', meeting && 'border-white/[0.08]')}
        >
          {syncStatus === 'saving' && (
            <Loader2 className={cn('h-3 w-3 animate-spin', meeting ? 'text-white/40' : 'text-muted-foreground/50')} />
          )}
          {syncStatus === 'saved' && <Check className="h-3 w-3 text-emerald-500" />}
          <span className={cn('text-[11px]', meeting ? 'text-white/40' : 'text-muted-foreground/50')}>
            {syncStatus === 'saving' && 'Saving...'}
            {syncStatus === 'saved' && 'Saved'}
            {syncStatus === 'error' && 'Sync failed'}
          </span>
        </div>
      )}
    </div>
  )
}
