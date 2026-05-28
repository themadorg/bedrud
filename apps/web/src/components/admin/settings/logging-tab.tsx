import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Field, Section } from './shared'
import type { SystemSettings } from './types'

export function LoggingTab({
  settings,
  setSettings,
}: {
  settings: SystemSettings
  setSettings: (s: SystemSettings) => void
}) {
  return (
    <Section title="Logging" description="Server log verbosity">
      <Field label="Log Level" hint="Saved to database. Leave empty to use config.yaml value (default: info).">
        <Select value={settings.logLevel || 'info'} onValueChange={(v) => setSettings({ ...settings, logLevel: v })}>
          <SelectTrigger className="h-8 w-full text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="debug">Debug</SelectItem>
            <SelectItem value="info">Info</SelectItem>
            <SelectItem value="warn">Warn</SelectItem>
            <SelectItem value="error">Error</SelectItem>
            <SelectItem value="trace">Trace</SelectItem>
          </SelectContent>
        </Select>
      </Field>
    </Section>
  )
}
