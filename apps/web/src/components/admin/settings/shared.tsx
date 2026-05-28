import { useMutation } from '@tanstack/react-query'
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useState } from 'react'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { cn } from '#/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import type { SystemSettings } from './types'

export function Section({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <div className="border bg-card/50">
      <div className="flex items-center justify-between border-b px-5 py-3">
        <div>
          <p className="text-sm font-semibold">{title}</p>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
      </div>
      <div className="space-y-4 p-5">{children}</div>
    </div>
  )
}

export function Field({ label, children, hint }: { label: string; children: React.ReactNode; hint?: string }) {
  return (
    <div className="space-y-1">
      <Label className="text-xs font-medium text-muted-foreground">{label}</Label>
      {children}
      {hint && <p className="text-[10px] text-muted-foreground/60">{hint}</p>}
    </div>
  )
}

export function TextInput({
  value,
  onChange,
  placeholder,
  mono,
  disabled,
  type,
  min,
  max,
  pattern,
  required,
  error,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  mono?: boolean
  disabled?: boolean
  type?: 'text' | 'number' | 'url' | 'email'
  min?: number
  max?: number
  pattern?: string
  required?: boolean
  error?: string
}) {
  return (
    <div>
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        type={type ?? 'text'}
        min={min}
        max={max}
        pattern={pattern}
        required={required}
        className={cn('h-8 px-1 text-xs', error ? 'border-destructive' : 'border-transparent', mono && 'font-mono')}
      />
      {error && <p className="mt-0.5 text-[10px] text-destructive">{error}</p>}
    </div>
  )
}

export function Toggle({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  hint?: string
}) {
  return (
    <div>
      <span className="flex cursor-pointer items-center gap-2">
        <Switch checked={checked} onCheckedChange={onChange} />
        <span className="text-xs">{label}</span>
      </span>
      {hint && <p className="mt-0.5 text-[10px] text-muted-foreground/60">{hint}</p>}
    </div>
  )
}

export function validateLocalSettings(s: SystemSettings): Record<string, string> {
  const errors: Record<string, string> = {}

  // Server port
  const port = Number(s.serverPort)
  if (s.serverPort !== '' && (Number.isNaN(port) || port < 1 || port > 65535)) {
    errors.serverPort = 'Must be between 1 and 65535'
  }

  // Email
  if (s.serverEmail !== '' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.serverEmail)) {
    errors.serverEmail = 'Invalid email format'
  }

  // Helper: check if string is a valid absolute URL (optional scheme — bare host:port OK)
  function isValidURL(val: string, allowSchemeLess: boolean): boolean {
    // If it parses as an absolute URL, check for supported scheme
    try {
      const u = new URL(val)
      if (u.protocol !== 'http:' && u.protocol !== 'https:' && u.protocol !== 'ws:' && u.protocol !== 'wss:') {
        return false
      }
      return true
    } catch {
      // Not an absolute URL — allow bare host:port if the field permits it
      if (allowSchemeLess && /^[a-zA-Z0-9.-]+(:\d+)?$/.test(val)) {
        return true
      }
      return false
    }
  }

  // URL fields that require absolute URLs with scheme
  const strictUrlFields: [string, string][] = [
    ['frontendUrl', s.frontendUrl],
    ['googleRedirectUrl', s.googleRedirectUrl],
    ['githubRedirectUrl', s.githubRedirectUrl],
    ['twitterRedirectUrl', s.twitterRedirectUrl],
    ['chatUploadS3Endpoint', s.chatUploadS3Endpoint],
    ['chatUploadS3PublicUrl', s.chatUploadS3PublicUrl],
  ]
  for (const [name, val] of strictUrlFields) {
    if (!val) continue
    if (!isValidURL(val, false)) {
      errors[name] = 'Invalid URL'
    }
  }

  // livekitHost — also accepts bare host:port (no scheme required)
  if (s.livekitHost && !isValidURL(s.livekitHost, true)) {
    errors.livekitHost = 'Invalid LiveKit host'
  }

  // CORS credentials + wildcard
  if (s.corsAllowCredentials) {
    if (s.corsAllowedOrigins === '' || s.corsAllowedOrigins === '*') {
      errors.corsAllowedOrigins = "Can't use wildcard with credentials"
    } else {
      const parts = s.corsAllowedOrigins.split(',')
      if (parts.some((o) => o.trim() === '*')) {
        errors.corsAllowedOrigins = "Can't use wildcard with credentials"
      }
    }
  }

  // corsMaxAge
  if (s.corsMaxAge < 0 || s.corsMaxAge > 86400) {
    errors.corsMaxAge = 'Must be between 0 and 86400'
  }

  // tokenDuration
  if (s.tokenDuration !== 0 && (s.tokenDuration < 1 || s.tokenDuration > 8760)) {
    errors.tokenDuration = 'Must be between 1 and 8760, or 0'
  }

  // maxParticipantsLimit
  if (s.maxParticipantsLimit < 0 || s.maxParticipantsLimit > 100000) {
    errors.maxParticipantsLimit = 'Must be between 0 and 100000'
  }

  // maxRoomsPerUser
  if (s.maxRoomsPerUser < 0 || s.maxRoomsPerUser > 100000) {
    errors.maxRoomsPerUser = 'Must be between 0 and 100000'
  }

  // Upload sizes
  if (s.chatUploadMaxBytes < 0) {
    errors.chatUploadMaxBytes = 'Cannot be negative'
  }
  if (s.chatUploadInlineMax < 0) {
    errors.chatUploadInlineMax = 'Cannot be negative'
  }
  if (s.maxUploadBytesPerUser < 0) {
    errors.maxUploadBytesPerUser = 'Cannot be negative'
  }
  if (s.globalDiskThresholdBytes < 0) {
    errors.globalDiskThresholdBytes = 'Cannot be negative'
  }

  // Chat retention
  if (s.chatMaxMessageCount < 0) {
    errors.chatMaxMessageCount = 'Cannot be negative'
  }
  if (s.chatMessageTTLHours < 0) {
    errors.chatMessageTTLHours = 'Cannot be negative'
  }

  // TODO oncoming feature
  // Recording limits
  if (s.recordingMaxDurationMins < 0) {
    errors.recordingMaxDurationMins = 'Cannot be negative'
  }
  // TODO oncoming feature
  if (s.recordingMaxFileSizeMB < 0) {
    errors.recordingMaxFileSizeMB = 'Cannot be negative'
  }

  // JWT secret length
  if (s.jwtSecret !== '' && s.jwtSecret.length < 32) {
    errors.jwtSecret = 'Must be at least 32 characters'
  }

  // Cross-field: TLS + !ACME → cert + key required
  if (s.serverEnableTls && !s.serverUseAcme) {
    if (!s.serverCertFile) errors.serverCertFile = 'Required when TLS enabled without ACME'
    if (!s.serverKeyFile) errors.serverKeyFile = 'Required when TLS enabled without ACME'
  }

  // Cross-field: ACME → email required + valid
  if (s.serverUseAcme) {
    if (!s.serverEnableTls) errors.serverUseAcme = 'ACME requires TLS to be enabled'
    if (!s.serverEmail) errors.serverEmail = 'Required when using ACME'
  }

  // Cross-field: LiveKit external → key + secret required
  if (s.livekitExternal) {
    if (!s.livekitApiKey) errors.livekitApiKey = 'Required for external LiveKit'
    if (!s.livekitApiSecret) errors.livekitApiSecret = 'Required for external LiveKit'
  }

  // Email branding — hex colors
  if (s.emailHeaderBg && !/^#[0-9a-fA-F]{6}$/.test(s.emailHeaderBg)) {
    errors.emailHeaderBg = 'Must be a hex color (#rrggbb)'
  }
  if (s.emailButtonBg && !/^#[0-9a-fA-F]{6}$/.test(s.emailButtonBg)) {
    errors.emailButtonBg = 'Must be a hex color (#rrggbb)'
  }

  // Email branding — support email
  if (s.emailSupportEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.emailSupportEmail)) {
    errors.emailSupportEmail = 'Invalid email format'
  }

  // Email branding — instance URL
  if (s.emailInstanceUrl) {
    try {
      const u = new URL(s.emailInstanceUrl)
      if (u.protocol !== 'http:' && u.protocol !== 'https:') {
        errors.emailInstanceUrl = 'Must be http or https URL'
      }
    } catch {
      errors.emailInstanceUrl = 'Invalid URL'
    }
  }

  // SMTP port
  if (s.emailSmtpPort && (s.emailSmtpPort < 1 || s.emailSmtpPort > 65535)) {
    errors.emailSmtpPort = 'Must be between 1 and 65535'
  }

  return errors
}

type CheckResult = { status: string; message?: string }

export function ValidateButton({
  label,
  payload,
  group,
  disabled,
}: {
  label: string
  payload: Partial<SystemSettings>
  group: string
  disabled?: boolean
}) {
  const [result, setResult] = useState<CheckResult | null>(null)

  const check = useMutation({
    mutationFn: () => api.post<{ checks: Record<string, CheckResult> }>('/api/admin/settings/validate', payload),
    onSuccess: (data) => {
      setResult(data.checks[group] ?? { status: 'skipped', message: 'No result for this check' })
    },
    onError: (err) => {
      setResult({ status: 'error', message: getErrorMessage(err, 'Validation request failed') })
    },
  })

  return (
    <div className="space-y-1.5">
      <Button
        variant="outline"
        size="sm"
        type="button"
        onClick={() => {
          setResult(null)
          check.mutate()
        }}
        disabled={check.isPending || disabled}
        className={cn(
          'h-8 gap-2 border px-3 text-xs font-normal',
          result?.status === 'ok' && 'border-emerald-500/40 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400',
          result?.status === 'error' && 'border-destructive/40 bg-destructive/5 text-destructive',
          result?.status === 'warning' && 'border-amber-500/40 bg-amber-500/5 text-amber-600 dark:text-amber-400',
        )}
      >
        {check.isPending ? (
          <>
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            <span>Validating…</span>
          </>
        ) : result ? (
          <>
            {result.status === 'ok' && <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />}
            {result.status === 'error' && <XCircle className="h-3.5 w-3.5 shrink-0" />}
            {result.status === 'warning' && (
              <span className="inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border border-current text-[10px] font-bold leading-none">
                !
              </span>
            )}
            {result.status === 'skipped' && (
              <span className="h-3.5 w-3.5 shrink-0 rounded-full border border-current" />
            )}
            <span>{result.message || result.status}</span>
          </>
        ) : (
          <>
            <span className="h-3.5 w-3.5 shrink-0 rounded-full border border-current" />
            <span>{label}</span>
          </>
        )}
      </Button>
    </div>
  )
}
