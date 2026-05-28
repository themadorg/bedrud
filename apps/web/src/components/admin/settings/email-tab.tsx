import { useState } from 'react'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { cn } from '#/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, Section, TextInput, Toggle } from './shared'
import type { SystemSettings } from './types'

const TEMPLATE_TYPES = [
  { id: 'verify', label: 'Verify Email' },
  { id: 'welcome', label: 'Welcome' },
  { id: 'reset', label: 'Password Reset' },
  { id: 'changed', label: 'Password Changed' },
  { id: 'invite', label: 'Room Invite' },
] as const

function subjectField(id: string): keyof SystemSettings {
  return `emailSubject${id.charAt(0).toUpperCase() + id.slice(1)}` as keyof SystemSettings
}

function preheaderField(id: string): keyof SystemSettings {
  return `emailPreheader${id.charAt(0).toUpperCase() + id.slice(1)}` as keyof SystemSettings
}

function safeStr(s: SystemSettings, field: keyof SystemSettings): string {
  return (s[field] as string) ?? ''
}

export function EmailTab({
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
  const [testEmailTo, setTestEmailTo] = useState('')
  const [testSending, setTestSending] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  async function handleSendTestEmail() {
    setTestSending(true)
    setTestResult(null)
    try {
      const res = await api.post<{ status: string; message: string }>('/api/admin/settings/send-test-email', {
        to: testEmailTo,
      })
      setTestResult({ ok: true, message: res.message })
    } catch (err: unknown) {
      setTestResult({ ok: false, message: getErrorMessage(err, 'Failed to send test email') })
    } finally {
      setTestSending(false)
    }
  }

  return (
    <>
      <Section title="Branding" description="Instance name and contact info for email footers">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Instance name" hint="Displayed in email headers and body text. Default: Bedrud">
            <TextInput
              value={settings.emailInstanceName}
              onChange={(v) => setSettings({ ...settings, emailInstanceName: v })}
              placeholder="Bedrud"
            />
          </Field>
          <Field label="Support email" hint="Shown in email footers as contact. Leave empty to hide.">
            <TextInput
              type="email"
              value={settings.emailSupportEmail}
              onChange={(v) => {
                ce('emailSupportEmail')
                setSettings({ ...settings, emailSupportEmail: v })
              }}
              placeholder="admin@example.com"
              error={errors?.emailSupportEmail}
            />
          </Field>
          <Field label="Instance URL" hint="Link in email footers. Leave empty to hide.">
            <TextInput
              type="url"
              value={settings.emailInstanceUrl}
              onChange={(v) => {
                ce('emailInstanceUrl')
                setSettings({ ...settings, emailInstanceUrl: v })
              }}
              placeholder="https://bedrud.example.com"
              error={errors?.emailInstanceUrl}
            />
          </Field>
        </div>
      </Section>

      <Section title="Colors" description="Brand colors used in email templates">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Header background" hint="Email header banner color. Default: #1a1a2e">
            <div className="flex items-center gap-2">
              <div
                className="h-8 w-8 shrink-0 rounded border"
                style={{ background: settings.emailHeaderBg || '#1a1a2e' }}
              />
              <TextInput
                value={settings.emailHeaderBg}
                onChange={(v) => {
                  ce('emailHeaderBg')
                  setSettings({ ...settings, emailHeaderBg: v })
                }}
                placeholder="#1a1a2e"
                mono
                error={errors?.emailHeaderBg}
              />
            </div>
          </Field>
          <Field label="Button background" hint="Primary CTA button color. Default: #e11d48">
            <div className="flex items-center gap-2">
              <div
                className="h-8 w-8 shrink-0 rounded border"
                style={{ background: settings.emailButtonBg || '#e11d48' }}
              />
              <TextInput
                value={settings.emailButtonBg}
                onChange={(v) => {
                  ce('emailButtonBg')
                  setSettings({ ...settings, emailButtonBg: v })
                }}
                placeholder="#e11d48"
                mono
                error={errors?.emailButtonBg}
              />
            </div>
          </Field>
        </div>
      </Section>

      <Section title="Subject Lines" description="Override email subjects per template type">
        <div className="grid gap-4 sm:grid-cols-2">
          {TEMPLATE_TYPES.map((t) => (
            <Field key={t.id} label={t.label}>
              <TextInput
                value={safeStr(settings, subjectField(t.id))}
                onChange={(v) => setSettings({ ...settings, [subjectField(t.id)]: v })}
                placeholder={`Leave empty for default`}
              />
            </Field>
          ))}
        </div>
      </Section>

      <Section title="Preheader Text" description="Short summary shown after subject line in inbox">
        <div className="grid gap-4 sm:grid-cols-2">
          {TEMPLATE_TYPES.map((t) => (
            <Field key={t.id} label={t.label}>
              <TextInput
                value={safeStr(settings, preheaderField(t.id))}
                onChange={(v) => setSettings({ ...settings, [preheaderField(t.id)]: v })}
                placeholder={`Leave empty for no preheader`}
              />
            </Field>
          ))}
        </div>
      </Section>

      <Section title="SMTP" description="Email delivery server settings">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Host" hint="SMTP server hostname. Leave empty to use config.yaml value.">
            <TextInput
              value={settings.emailSmtpHost}
              onChange={(v) => setSettings({ ...settings, emailSmtpHost: v })}
              placeholder="smtp.example.com"
            />
          </Field>
          <Field label="Port" hint="SMTP server port (587, 465, 25). 0 = use config.yaml.">
            <TextInput
              type="number"
              min={0}
              max={65535}
              value={String(settings.emailSmtpPort || 0)}
              onChange={(v) => {
                ce('emailSmtpPort')
                const parsed = Number(v)
                setSettings({ ...settings, emailSmtpPort: Number.isNaN(parsed) ? 0 : parsed })
              }}
              placeholder="587"
              error={errors?.emailSmtpPort}
            />
          </Field>
          <Field label="Username" hint="SMTP auth username. Leave empty to use config.yaml.">
            <TextInput
              value={settings.emailUsername}
              onChange={(v) => setSettings({ ...settings, emailUsername: v })}
              placeholder="user@example.com"
            />
          </Field>
          <Field
            label="Password"
            hint={
              settings.emailPassword === '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022'
                ? 'Hidden — enter new value to change. SMTP password.'
                : 'SMTP password. Leave empty to use config.yaml value.'
            }
          >
            <TextInput
              value={settings.emailPassword}
              onChange={(v) => setSettings({ ...settings, emailPassword: v })}
              mono
              placeholder="SMTP password"
            />
          </Field>
          <Field label="From address" hint="Sender email address. Leave empty to use config.yaml.">
            <TextInput
              type="email"
              value={settings.emailFromAddress}
              onChange={(v) => setSettings({ ...settings, emailFromAddress: v })}
              placeholder="noreply@example.com"
            />
          </Field>
          <Field label="From name" hint="Sender display name. Leave empty to use config.yaml.">
            <TextInput
              value={settings.emailFromName}
              onChange={(v) => setSettings({ ...settings, emailFromName: v })}
              placeholder="Bedrud"
            />
          </Field>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 pt-2">
          <Toggle
            checked={settings.emailTlsSkipVerify}
            onChange={(v) => setSettings({ ...settings, emailTlsSkipVerify: v })}
            label="Skip TLS verification"
            hint="Disable TLS certificate validation (development only)"
          />
          <Toggle
            checked={settings.emailSmtpsMode}
            onChange={(v) => setSettings({ ...settings, emailSmtpsMode: v })}
            label="SMTPS mode"
            hint="Direct TLS connection (port 465). Leave unchecked for STARTTLS (587/25)."
          />
        </div>

        {/* Send Test Email */}
        <div className="space-y-3 pt-2">
          <Field label="Send Test Email" hint="Send a real test email to verify SMTP config works end-to-end.">
            <div className="flex gap-2">
              <Input
                type="email"
                placeholder="admin@example.com"
                value={testEmailTo}
                onChange={(e) => {
                  setTestEmailTo(e.target.value)
                  setTestResult(null)
                }}
                disabled={testSending}
                className="h-8 px-1 text-xs"
              />
              <Button
                onClick={handleSendTestEmail}
                disabled={testSending || !testEmailTo}
                className="h-8 shrink-0 px-3 text-xs"
              >
                {testSending ? 'Sending…' : 'Send Test'}
              </Button>
            </div>
            {testResult && (
              <div
                className={cn(
                  'mt-2 rounded border px-3 py-2 text-xs',
                  testResult.ok
                    ? 'border-emerald-500/40 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400'
                    : 'border-destructive/40 bg-destructive/5 text-destructive',
                )}
              >
                {testResult.message}
              </div>
            )}
          </Field>
        </div>
      </Section>
    </>
  )
}
