import { Field, numOrPrev, Section, TextInput, Toggle, WELCOME_SOUND_LIMIT_BOUNDS } from './shared'
import type { SystemSettings } from './types'

type WelcomeSoundLimitField =
  | 'welcomeSoundMaxBytes'
  | 'welcomeSoundMaxDurationSeconds'
  | 'welcomeSoundMaxPeakDbfs'
  | 'welcomeSoundMaxLoudnessLufs'

const LIMIT_FIELDS: {
  field: WelcomeSoundLimitField
  label: string
  hint: string
  placeholder: string
  min: number
  max: number
}[] = [
  {
    field: 'welcomeSoundMaxBytes',
    label: 'Max file size (bytes)',
    hint: 'Largest MP3 a user may upload. 32 KB = 32768.',
    placeholder: '32768',
    min: 0,
    max: WELCOME_SOUND_LIMIT_BOUNDS.maxBytes,
  },
  {
    field: 'welcomeSoundMaxDurationSeconds',
    label: 'Max length (seconds)',
    hint: 'Longest clip a user may upload.',
    placeholder: '7',
    min: 0,
    max: WELCOME_SOUND_LIMIT_BOUNDS.maxDurationSeconds,
  },
  {
    field: 'welcomeSoundMaxPeakDbfs',
    label: 'Max peak (dBFS)',
    hint: 'Clips whose loudest sample reaches this level are rejected.',
    placeholder: '-1',
    min: WELCOME_SOUND_LIMIT_BOUNDS.minPeakDbfs,
    max: 0,
  },
  {
    field: 'welcomeSoundMaxLoudnessLufs',
    label: 'Max loudness (LUFS)',
    hint: 'Clips louder overall than this are rejected. Lower is quieter.',
    placeholder: '-20',
    min: WELCOME_SOUND_LIMIT_BOUNDS.minLoudnessLufs,
    max: 0,
  },
]

export function WelcomeSoundSection({
  settings,
  setSettings,
  errors,
  clearFieldError,
}: {
  settings: SystemSettings
  setSettings: (s: SystemSettings) => void
  errors?: Record<string, string>
  clearFieldError?: (field: string) => void
}) {
  return (
    <Section
      title="Welcome sounds"
      description="A short MP3 on each user's profile that plays for everyone in the room when they join. Uploads are checked on the server for length, peak and loudness."
    >
      <Field
        label="Allow welcome sounds"
        hint="Off by default. When off, nobody can upload one and no sound plays on join."
      >
        <Toggle
          checked={settings.welcomeSoundEnabled}
          onChange={(next) => setSettings({ ...settings, welcomeSoundEnabled: next })}
          label={settings.welcomeSoundEnabled ? 'Users may set a welcome sound' : 'Welcome sounds off (default)'}
        />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        {LIMIT_FIELDS.map(({ field, label, hint, placeholder, min, max }) => (
          <Field key={field} label={label} hint={`${hint} 0 uses config.yaml.`}>
            <TextInput
              type="number"
              min={min}
              max={max}
              value={String(settings[field] || 0)}
              onChange={(v) => {
                clearFieldError?.(field)
                setSettings({ ...settings, [field]: numOrPrev(v, settings[field]) })
              }}
              placeholder={placeholder}
              error={errors?.[field]}
            />
          </Field>
        ))}
      </div>
    </Section>
  )
}
