import { Field, Section, TextInput, Toggle } from './shared'
import type { SystemSettings } from './types'

function numOrPrev(v: string, prev: number): number {
  const parsed = Number(v)
  return Number.isNaN(parsed) ? prev : parsed
}

export function CorsTab({
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
  const ce = (field: string) => clearFieldError?.(field)

  return (
    <Section title="CORS" description="Cross-origin resource sharing">
      <Field
        label="Allowed Origins"
        hint="Comma-separated or *. Saved to database. Leave empty to use config.yaml value."
      >
        <TextInput
          value={settings.corsAllowedOrigins}
          onChange={(v) => {
            ce('corsAllowedOrigins')
            setSettings({ ...settings, corsAllowedOrigins: v })
          }}
          placeholder="http://localhost:5173"
          error={errors?.corsAllowedOrigins}
        />
      </Field>
      <Field
        label="Allowed Headers"
        hint="Comma-separated. Saved to database. Leave empty to use config.yaml value (default: Origin, Content-Type, Accept, Authorization)."
      >
        <TextInput
          value={settings.corsAllowedHeaders}
          onChange={(v) => setSettings({ ...settings, corsAllowedHeaders: v })}
          placeholder="Origin, Content-Type, Accept, Authorization"
        />
      </Field>
      <Field
        label="Allowed Methods"
        hint="Comma-separated HTTP methods. Saved to database. Leave empty to use config.yaml value (default: GET, POST, PUT, DELETE, OPTIONS)."
      >
        <TextInput
          value={settings.corsAllowedMethods}
          onChange={(v) => setSettings({ ...settings, corsAllowedMethods: v })}
          placeholder="GET, POST, PUT, DELETE, OPTIONS"
        />
      </Field>
      <Field
        label="Max Age (seconds)"
        hint="Max 86400 (24 hours). 0 = no cache. Saved to database. Leave empty to use config.yaml value."
      >
        <TextInput
          type="number"
          min={0}
          max={86400}
          value={String(settings.corsMaxAge)}
          onChange={(v) => {
            ce('corsMaxAge')
            setSettings({ ...settings, corsMaxAge: numOrPrev(v, settings.corsMaxAge) })
          }}
          placeholder="0"
          error={errors?.corsMaxAge}
        />
      </Field>
      <Toggle
        checked={settings.corsAllowCredentials}
        onChange={(v) => {
          ce('corsAllowCredentials')
          ce('corsAllowedOrigins')
          setSettings({ ...settings, corsAllowCredentials: v })
        }}
        label="Allow credentials"
        hint="When enabled, cookies and auth headers are sent cross-origin. Cannot be used with wildcard origins. Saved to database. Leave empty to use config.yaml value."
      />
    </Section>
  )
}
