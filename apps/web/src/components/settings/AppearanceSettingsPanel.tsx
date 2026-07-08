import { useMutation } from '@tanstack/react-query'
import { Check, Loader2, Monitor, Moon, Sun } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '#/lib/auth.store'
import { useInterfacePreferencesStore } from '#/lib/interface-preferences.store'
import { applyTheme, resolveTheme, type Theme, useThemeStore } from '#/lib/theme.store'
import { patchUserPreferences } from '#/lib/user-preferences'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { isMeetingTone, panelSurfaceClass, type SettingsPanelTone } from './settingsPanelTone'

const THEME_OPTIONS: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

function sectionBorderClass(meeting: boolean) {
  return meeting ? 'border-[var(--meet-border)]' : 'border-border'
}

function optionClass(active: boolean, meeting: boolean) {
  return cn(
    'flex min-w-0 flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-2.5 text-xs font-medium transition-colors',
    active
      ? meeting
        ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
        : 'bg-background text-foreground shadow-sm'
      : meeting
        ? 'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control-hover)] hover:text-[var(--meet-fg-strong)]'
        : 'text-muted-foreground hover:bg-muted/80 hover:text-foreground',
  )
}

export function AppearanceSettingsPanel({ tone = 'default' }: { tone?: SettingsPanelTone }) {
  const meeting = isMeetingTone(tone)
  const tokens = useAuthStore((s) => s.tokens)
  const theme = useThemeStore((s) => s.theme)
  const setTheme = useThemeStore((s) => s.setTheme)
  const showWelcomeScreen = useInterfacePreferencesStore((s) => s.showWelcomeScreen)
  const setShowWelcomeScreen = useInterfacePreferencesStore((s) => s.setShowWelcomeScreen)
  const [mounted, setMounted] = useState(false)

  const interfacePrefsRef = useRef({ showWelcomeScreen })
  interfacePrefsRef.current = { showWelcomeScreen }

  const syncMutation = useMutation({
    mutationFn: () => patchUserPreferences({ interface: interfacePrefsRef.current }),
  })
  const mutateRef = useRef(syncMutation.mutate)
  mutateRef.current = syncMutation.mutate

  useEffect(() => {
    setMounted(true)
  }, [])

  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional trigger to re-save on welcome screen toggle
  useEffect(() => {
    if (!tokens) return
    const timer = setTimeout(() => mutateRef.current(), 1000)
    return () => clearTimeout(timer)
  }, [showWelcomeScreen, tokens])

  useEffect(() => {
    if (!tokens) return
    return () => {
      void patchUserPreferences({ interface: interfacePrefsRef.current })
    }
  }, [tokens])

  const resolvedTheme = mounted ? resolveTheme(theme) : null

  function selectTheme(next: Theme) {
    setTheme(next)
    applyTheme(next)
  }

  const syncStatus = !tokens
    ? 'idle'
    : syncMutation.isPending
      ? 'saving'
      : syncMutation.isError
        ? 'error'
        : syncMutation.isSuccess
          ? 'saved'
          : 'idle'

  return (
    <div className={panelSurfaceClass(tone)}>
      <div className={cn('border-b px-5 py-5', sectionBorderClass(meeting))}>
        <p className="text-sm font-medium">Interface theme</p>
        <p className={cn('mt-0.5 text-xs', meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground')}>
          Choose light or dark appearance across Bedrud
        </p>
      </div>

      <div className="px-5 py-4">
        <div className={cn('flex gap-1 rounded-lg p-1', meeting ? 'bg-[var(--meet-control)]' : 'bg-muted/40')}>
          {THEME_OPTIONS.map(({ value, label, icon: Icon }) => (
            <button
              key={value}
              type="button"
              onClick={() => selectTheme(value)}
              className={optionClass(theme === value, meeting)}
            >
              <Icon className="h-3.5 w-3.5 shrink-0" />
              {label}
            </button>
          ))}
        </div>

        {mounted && theme === 'system' && resolvedTheme && (
          <p className={cn('mt-3 text-xs', meeting ? 'text-[var(--meet-fg-subtle)]' : 'text-muted-foreground')}>
            Using {resolvedTheme === 'dark' ? 'dark' : 'light'} mode from your system settings.
          </p>
        )}
      </div>

      <div className={cn('flex items-center justify-between gap-4 border-t px-5 py-4', sectionBorderClass(meeting))}>
        <div className="min-w-0">
          <p className="text-sm font-medium">Welcome screen</p>
          <p className={cn('mt-0.5 text-xs', meeting ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground')}>
            Show camera and microphone setup before joining a meeting
          </p>
        </div>
        <Switch checked={showWelcomeScreen} onCheckedChange={setShowWelcomeScreen} />
      </div>

      {syncStatus !== 'idle' && (
        <div className={cn('flex items-center justify-end gap-1.5 border-t px-5 py-2.5', sectionBorderClass(meeting))}>
          {syncStatus === 'saving' && (
            <Loader2
              className={cn(
                'h-3 w-3 animate-spin',
                meeting ? 'text-[var(--meet-fg-subtle)]' : 'text-muted-foreground/50',
              )}
            />
          )}
          {syncStatus === 'saved' && <Check className="h-3 w-3 text-emerald-500" />}
          <span className={cn('text-[11px]', meeting ? 'text-[var(--meet-fg-subtle)]' : 'text-muted-foreground/50')}>
            {syncStatus === 'saving' && 'Saving...'}
            {syncStatus === 'saved' && 'Saved'}
            {syncStatus === 'error' && 'Sync failed'}
          </span>
        </div>
      )}
    </div>
  )
}
