import { Ban, Globe, KeyRound } from 'lucide-react'
import { cn } from '#/lib/utils'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { InviteTokensSection } from './invite-tokens-section'
import { Section } from './shared'
import type { RegMode, SystemSettings } from './types'

function getMode(s: SystemSettings): RegMode {
  if (!s.registrationEnabled) return 'closed'
  if (s.tokenRegistrationOnly) return 'invite'
  return 'open'
}

function modeToSettings(mode: RegMode): Pick<SystemSettings, 'registrationEnabled' | 'tokenRegistrationOnly'> {
  if (mode === 'open') return { registrationEnabled: true, tokenRegistrationOnly: false }
  if (mode === 'invite') return { registrationEnabled: true, tokenRegistrationOnly: true }
  return { registrationEnabled: false, tokenRegistrationOnly: false }
}

const MODES: { id: RegMode; icon: React.ElementType; label: string; description: string }[] = [
  { id: 'open', icon: Globe, label: 'Open', description: 'Anyone can create an account' },
  { id: 'invite', icon: KeyRound, label: 'Invite-only', description: 'Valid invite token required' },
  { id: 'closed', icon: Ban, label: 'Closed', description: 'No new registrations' },
]

export function GeneralTab({
  settings,
  onPatch,
  saving,
}: {
  settings: SystemSettings
  onPatch: (p: Partial<SystemSettings>) => void
  saving: boolean
}) {
  const currentMode = getMode(settings)
  return (
    <div className="space-y-4">
      <Section title="Registration" description="Who can create accounts">
        <RadioGroup
          value={currentMode}
          onValueChange={(val) => onPatch(modeToSettings(val as RegMode))}
          className="space-y-2"
          disabled={saving}
        >
          {MODES.map(({ id, icon: Icon, label, description }) => {
            const active = currentMode === id
            return (
              <Label
                key={id}
                htmlFor={id}
                className={cn(
                  'flex w-full cursor-pointer items-center gap-3 border p-3 h-auto text-left has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring',
                  active
                    ? id === 'closed'
                      ? 'border-destructive/40 bg-destructive/5'
                      : 'border-primary/30 bg-primary/5'
                    : 'hover:bg-accent',
                )}
              >
                <div
                  className={cn(
                    'flex h-8 w-8 shrink-0 items-center justify-center',
                    active
                      ? id === 'closed'
                        ? 'bg-destructive/10 text-destructive'
                        : 'bg-primary/10 text-primary'
                      : 'bg-muted text-muted-foreground',
                  )}
                >
                  <Icon className="h-3.5 w-3.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <p
                    className={cn(
                      'text-sm font-medium',
                      active ? (id === 'closed' ? 'text-destructive' : 'text-primary') : 'text-foreground',
                    )}
                  >
                    {label}
                  </p>
                  <p className="text-xs text-muted-foreground">{description}</p>
                </div>
                <RadioGroupItem
                  value={id}
                  id={id}
                  className={cn(
                    active
                      ? id === 'closed'
                        ? 'border-destructive bg-destructive text-destructive-foreground'
                        : 'border-primary bg-primary text-primary-foreground'
                      : '',
                  )}
                />
              </Label>
            )
          })}
        </RadioGroup>
      </Section>
      <InviteTokensSection />
    </div>
  )
}
