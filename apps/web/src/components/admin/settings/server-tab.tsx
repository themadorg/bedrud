import { useQuery } from '@tanstack/react-query'
import { AlertCircle, Check, RefreshCw } from 'lucide-react'
import { api } from '#/lib/api'
import { cn } from '#/lib/utils'
import { Field, Section, TextInput, Toggle, ValidateButton } from './shared'
import type { SystemSettings } from './types'

function CertStatusIndicator() {
  const { data: cert } = useQuery({
    queryKey: ['admin', 'cert-info'],
    queryFn: () =>
      api.get<{ enabled: boolean; status: string; daysRemaining?: number; notAfter?: string; error?: string }>(
        '/api/admin/cert-info',
      ),
  })

  if (!cert?.enabled) return null

  const status = cert.status
  const isError = status === 'error' || status === 'expired'
  const isExpiring = status === 'expiring'

  return (
    <div
      className={cn(
        'flex items-center gap-2 px-3 py-2 text-xs',
        isError
          ? 'border border-destructive/30 bg-destructive/10 text-destructive'
          : isExpiring
            ? 'border border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400'
            : 'border border-emerald-500/20 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400',
      )}
    >
      {isError ? <AlertCircle className="h-3 w-3 shrink-0" /> : <Check className="h-3 w-3 shrink-0" />}
      {isError && cert.error && <span>{cert.error}</span>}
      {status === 'expired' && <span>Certificate has expired</span>}
      {isExpiring && cert.daysRemaining != null && <span>Expires in {cert.daysRemaining} days</span>}
      {status === 'valid' && cert.daysRemaining != null && (
        <span>
          Valid — expires in {cert.daysRemaining} days (
          {cert.notAfter ? new Date(cert.notAfter).toLocaleDateString() : ''})
        </span>
      )}
    </div>
  )
}

function numOrPrev(v: string, prev: number): number {
  const parsed = Number(v)
  return Number.isNaN(parsed) ? prev : parsed
}

export function ServerTab({
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
    <Section title="Server" description="Requires restart to take effect">
      <div className="mb-3 flex items-center gap-2 border border-primary/20 bg-primary/5 px-3 py-2 text-xs text-primary">
        <RefreshCw className="h-3 w-3 shrink-0" />
        Changes here require a server restart to take effect.
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          label="Server name"
          hint="Human-readable instance name shown in UI and emails. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            value={settings.serverName}
            onChange={(v) => setSettings({ ...settings, serverName: v })}
            placeholder="Bedrud Meet"
          />
        </Field>
        <Field label="Port" hint="HTTP listen port. Saved to database. Leave empty to use config.yaml value.">
          <TextInput
            type="number"
            min={1}
            max={65535}
            value={settings.serverPort}
            onChange={(v) => {
              ce('serverPort')
              setSettings({ ...settings, serverPort: v })
            }}
            placeholder="8090"
            error={errors?.serverPort}
          />
        </Field>
        <Field
          label="Host"
          hint="Bind address. Saved to database. Leave empty to use config.yaml value (default: 0.0.0.0)."
        >
          <TextInput
            value={settings.serverHost}
            onChange={(v) => setSettings({ ...settings, serverHost: v })}
            placeholder="localhost"
          />
        </Field>
        <Field
          label="Domain"
          hint="Public domain for TLS certificates. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            type="url"
            value={settings.serverDomain}
            onChange={(v) => {
              ce('serverDomain')
              setSettings({ ...settings, serverDomain: v })
            }}
            placeholder="example.com"
            error={errors?.serverDomain}
          />
        </Field>
        <Field
          label="Email (ACME)"
          hint="Used by Let's Encrypt for expiry notifications. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            type="email"
            value={settings.serverEmail}
            onChange={(v) => {
              ce('serverEmail')
              setSettings({ ...settings, serverEmail: v })
            }}
            placeholder="admin@example.com"
            error={errors?.serverEmail}
          />
        </Field>
      </div>
      <div className="space-y-2 pt-2">
        <Toggle
          checked={settings.serverEnableTls}
          onChange={(v) => {
            ce('serverEnableTls')
            ce('serverCertFile')
            ce('serverKeyFile')
            setSettings({ ...settings, serverEnableTls: v })
          }}
          label="Enable TLS"
          hint="Requires cert files or ACME. Enables HTTPS and secure WebSocket."
        />
        <Toggle
          checked={settings.serverUseAcme}
          onChange={(v) => {
            ce('serverUseAcme')
            ce('serverEmail')
            setSettings({ ...settings, serverUseAcme: v })
          }}
          label="Use Let's Encrypt (ACME)"
          hint="Auto-provisions TLS certs. Requires TLS enabled + valid email + public domain DNS."
        />
        <Toggle
          checked={settings.behindProxy}
          onChange={(v) => setSettings({ ...settings, behindProxy: v })}
          label="Behind reverse proxy"
          hint="Trusts X-Forwarded-* headers from nginx, Cloudflare, etc. Disables automatic TLS redirect."
        />
      </div>
      <div className="grid gap-4 pt-2 sm:grid-cols-2">
        <Field
          label="Cert file"
          hint="Absolute path to PEM-encoded TLS certificate. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            value={settings.serverCertFile}
            onChange={(v) => {
              ce('serverCertFile')
              setSettings({ ...settings, serverCertFile: v })
            }}
            placeholder="/etc/bedrud/cert.pem"
            mono
            error={errors?.serverCertFile}
          />
        </Field>
        <Field
          label="Key file"
          hint="Absolute path to PEM-encoded TLS private key. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            value={settings.serverKeyFile}
            onChange={(v) => {
              ce('serverKeyFile')
              setSettings({ ...settings, serverKeyFile: v })
            }}
            placeholder="/etc/bedrud/key.pem"
            mono
            error={errors?.serverKeyFile}
          />
        </Field>
      </div>
      {settings.serverEnableTls && !settings.serverUseAcme && (
        <ValidateButton
          label="Validate certificate files"
          group="tls"
          payload={{ serverCertFile: settings.serverCertFile, serverKeyFile: settings.serverKeyFile }}
        />
      )}
      {settings.serverUseAcme && (
        <ValidateButton label="Check email domain" group="email" payload={{ serverEmail: settings.serverEmail }} />
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          label="Max participants limit"
          hint="Hard ceiling for room capacity. 0 = unlimited. Saved to database. Leave empty to use config.yaml value (default: 1000)."
        >
          <TextInput
            type="number"
            min={0}
            max={100000}
            value={String(settings.maxParticipantsLimit ?? 1000)}
            onChange={(v) => {
              ce('maxParticipantsLimit')
              setSettings({ ...settings, maxParticipantsLimit: numOrPrev(v, settings.maxParticipantsLimit ?? 1000) })
            }}
            error={errors?.maxParticipantsLimit}
          />
        </Field>
        <Field
          label="Max rooms per user"
          hint="Maximum rooms a single user can create. 0 = unlimited. Saved to database. Leave empty to use config.yaml value (default: 100)."
        >
          <TextInput
            type="number"
            min={0}
            max={100000}
            value={String(settings.maxRoomsPerUser ?? 100)}
            onChange={(v) => {
              ce('maxRoomsPerUser')
              setSettings({ ...settings, maxRoomsPerUser: numOrPrev(v, settings.maxRoomsPerUser ?? 100) })
            }}
            error={errors?.maxRoomsPerUser}
          />
        </Field>
      </div>
      {settings.serverEnableTls && <CertStatusIndicator />}
    </Section>
  )
}
