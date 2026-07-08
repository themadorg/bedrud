import { Field, Section, TextInput, Toggle, ValidateButton } from './shared'
import type { SystemSettings } from './types'

export function LiveKitTab({
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
    <Section title="LiveKit" description="Real-time audio/video infrastructure">
      <Toggle
        checked={settings.livekitExternal}
        onChange={(v) => {
          ce('livekitExternal')
          ce('livekitApiKey')
          ce('livekitApiSecret')
          setSettings({ ...settings, livekitExternal: v })
        }}
        label="Use external LiveKit server"
      />
      <Field
        label="Host"
        hint="LiveKit server URL. Uses internal host when external off, this value when external on. Saved to database. Leave empty to use config.yaml value."
      >
        <TextInput
          type="url"
          value={settings.livekitHost}
          onChange={(v) => {
            ce('livekitHost')
            setSettings({ ...settings, livekitHost: v })
          }}
          placeholder="http://localhost:7072"
          mono
          error={errors?.livekitHost}
        />
      </Field>
      <Field
        label="API Key"
        hint="From LiveKit config file (api_key) or generated key pair. Saved to database. Leave empty to use config.yaml value."
      >
        <TextInput
          value={settings.livekitApiKey}
          onChange={(v) => {
            ce('livekitApiKey')
            setSettings({ ...settings, livekitApiKey: v })
          }}
          placeholder="Enter API key"
          mono
          error={errors?.livekitApiKey}
        />
      </Field>
      <Field
        label="API Secret"
        hint={
          settings.livekitApiSecret === '••••••••'
            ? 'Hidden — enter new value to change'
            : 'Saved to database. Leave empty to use config.yaml value.'
        }
      >
        <TextInput
          value={settings.livekitApiSecret}
          onChange={(v) => {
            ce('livekitApiSecret')
            setSettings({ ...settings, livekitApiSecret: v })
          }}
          placeholder="Enter API secret"
          mono
          error={errors?.livekitApiSecret}
        />
      </Field>
      {(settings.livekitHost || settings.livekitApiKey || settings.livekitApiSecret) && (
        <ValidateButton
          label="Test LiveKit connection"
          group="livekit"
          payload={{
            livekitHost: settings.livekitHost,
            livekitApiKey: settings.livekitApiKey,
            livekitApiSecret: settings.livekitApiSecret === '••••••••' ? '' : settings.livekitApiSecret,
          }}
        />
      )}
    </Section>
  )
}
