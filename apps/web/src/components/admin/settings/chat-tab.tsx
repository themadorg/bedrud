import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Field, Section, TextInput, ValidateButton } from './shared'
import type { SystemSettings } from './types'

function numOrPrev(v: string, prev: number): number {
  const parsed = Number(v)
  return Number.isNaN(parsed) ? prev : parsed
}

function numOrPrev64(v: string, prev: number): number {
  const parsed = Number(v)
  return Number.isNaN(parsed) ? prev : parsed
}

export function ChatTab({
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
    <>
      <Section title="Chat Uploads" description="Image upload storage for in-room chat">
        <Field
          label="Backend"
          hint="Disk — local filesystem. S3 — remote object storage. Inline — base64 embedded in chat messages (no persistance). Saved to database. Leave empty to use config.yaml value."
        >
          <Select
            value={settings.chatUploadBackend || 'disk'}
            onValueChange={(v) => setSettings({ ...settings, chatUploadBackend: v })}
          >
            <SelectTrigger className="h-8 w-full text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="disk">Disk</SelectItem>
              <SelectItem value="s3">S3-compatible</SelectItem>
              <SelectItem value="inline">Inline (base64)</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label="Max upload size (bytes)"
            hint="Maximum file size per upload. 10MB = 10485760. Saved to database. Leave empty to use config.yaml value."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.chatUploadMaxBytes || 0)}
              onChange={(v) => {
                ce('chatUploadMaxBytes')
                setSettings({ ...settings, chatUploadMaxBytes: numOrPrev64(v, settings.chatUploadMaxBytes) })
              }}
              placeholder="10485760"
              error={errors?.chatUploadMaxBytes}
            />
          </Field>
          <Field
            label="Inline max bytes"
            hint="Max size for base64-embedded images. Files above this use the configured backend. Saved to database. Leave empty to use config.yaml value."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.chatUploadInlineMax || 0)}
              onChange={(v) => {
                ce('chatUploadInlineMax')
                setSettings({ ...settings, chatUploadInlineMax: numOrPrev64(v, settings.chatUploadInlineMax) })
              }}
              placeholder="512000"
              error={errors?.chatUploadInlineMax}
            />
          </Field>
          <Field
            label="Max uploads per user (bytes)"
            hint="Hard ceiling for total uploaded bytes per user. 0 = unlimited. Saved to database. Leave empty to use config.yaml value."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.maxUploadBytesPerUser || 0)}
              onChange={(v) => {
                ce('maxUploadBytesPerUser')
                setSettings({ ...settings, maxUploadBytesPerUser: numOrPrev64(v, settings.maxUploadBytesPerUser) })
              }}
              placeholder="524288000"
              error={errors?.maxUploadBytesPerUser}
            />
          </Field>
          <Field
            label="Global disk threshold (bytes)"
            hint="Stop accepting uploads when disk usage exceeds this across all users. 0 = unlimited. Saved to database. Leave empty to use config.yaml value."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.globalDiskThresholdBytes || 0)}
              onChange={(v) => {
                ce('globalDiskThresholdBytes')
                setSettings({
                  ...settings,
                  globalDiskThresholdBytes: numOrPrev64(v, settings.globalDiskThresholdBytes),
                })
              }}
              placeholder="0"
              error={errors?.globalDiskThresholdBytes}
            />
          </Field>
        </div>
        <Field
          label="Disk directory"
          hint="Directory for stored uploads. Relative to server working directory. Saved to database. Leave empty to use config.yaml value."
        >
          <TextInput
            value={settings.chatUploadDiskDir}
            onChange={(v) => setSettings({ ...settings, chatUploadDiskDir: v })}
            placeholder="./data/uploads/chat"
          />
        </Field>

        {settings.chatUploadBackend === 's3' && (
          <div className="space-y-3 border-t pt-4">
            <p className="text-xs font-medium text-muted-foreground">S3 Configuration</p>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                label="Endpoint"
                hint="S3-compatible endpoint URL. Saved to database. Leave empty to use config.yaml value."
              >
                <TextInput
                  type="url"
                  value={settings.chatUploadS3Endpoint}
                  onChange={(v) => {
                    ce('chatUploadS3Endpoint')
                    setSettings({ ...settings, chatUploadS3Endpoint: v })
                  }}
                  placeholder="https://s3.amazonaws.com"
                  error={errors?.chatUploadS3Endpoint}
                />
              </Field>
              <Field
                label="Bucket"
                hint="Bucket name to store chat uploads. Saved to database. Leave empty to use config.yaml value."
              >
                <TextInput
                  value={settings.chatUploadS3Bucket}
                  onChange={(v) => setSettings({ ...settings, chatUploadS3Bucket: v })}
                  placeholder="my-bucket"
                />
              </Field>
              <Field
                label="Region"
                hint="AWS region or equivalent (e.g. us-east-1). Saved to database. Leave empty to use config.yaml value."
              >
                <TextInput
                  value={settings.chatUploadS3Region}
                  onChange={(v) => setSettings({ ...settings, chatUploadS3Region: v })}
                  placeholder="us-east-1"
                />
              </Field>
              <Field
                label="Access Key"
                hint="S3 access key ID. Saved to database. Leave empty to use config.yaml value."
              >
                <TextInput
                  value={settings.chatUploadS3AccessKey}
                  onChange={(v) => setSettings({ ...settings, chatUploadS3AccessKey: v })}
                  mono
                />
              </Field>
              <Field
                label="Secret Key"
                hint={
                  settings.chatUploadS3SecretKey === '••••••••'
                    ? 'Hidden — enter new value to change. S3 secret key.'
                    : 'S3 secret access key. Saved to database. Leave empty to use config.yaml value.'
                }
              >
                <TextInput
                  value={settings.chatUploadS3SecretKey}
                  onChange={(v) => setSettings({ ...settings, chatUploadS3SecretKey: v })}
                  mono
                />
              </Field>
              <Field
                label="Public Base URL"
                hint="Public URL prefix for serving files. Leave empty to use endpoint + bucket. Saved to database. Leave empty to use config.yaml value."
              >
                <TextInput
                  type="url"
                  value={settings.chatUploadS3PublicUrl}
                  onChange={(v) => {
                    ce('chatUploadS3PublicUrl')
                    setSettings({ ...settings, chatUploadS3PublicUrl: v })
                  }}
                  placeholder="https://cdn.example.com"
                  error={errors?.chatUploadS3PublicUrl}
                />
              </Field>
            </div>
            <ValidateButton
              label="Test S3 connection"
              group="s3"
              payload={{
                chatUploadBackend: 's3',
                chatUploadS3Endpoint: settings.chatUploadS3Endpoint,
                chatUploadS3Bucket: settings.chatUploadS3Bucket,
                chatUploadS3AccessKey: settings.chatUploadS3AccessKey,
                chatUploadS3SecretKey:
                  settings.chatUploadS3SecretKey === '••••••••' ? '' : settings.chatUploadS3SecretKey,
              }}
            />
          </div>
        )}
      </Section>

      <Section title="Chat History" description="Message retention limits for in-room chat">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label="Max messages per room"
            hint="Oldest messages are trimmed when exceeded. 0 = unlimited. Saved to database. Leave empty to use config.yaml value (default: 10000)."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.chatMaxMessageCount ?? 10000)}
              onChange={(v) => {
                ce('chatMaxMessageCount')
                setSettings({
                  ...settings,
                  chatMaxMessageCount: numOrPrev(v, settings.chatMaxMessageCount ?? 10000),
                })
              }}
              placeholder="10000"
              error={errors?.chatMaxMessageCount}
            />
          </Field>
          <Field
            label="Message TTL (hours)"
            hint="Messages older than this are purged. 0 = forever. Saved to database. Leave empty to use config.yaml value (default: 2160 = 90 days)."
          >
            <TextInput
              type="number"
              min={0}
              value={String(settings.chatMessageTTLHours ?? 2160)}
              onChange={(v) => {
                ce('chatMessageTTLHours')
                setSettings({
                  ...settings,
                  chatMessageTTLHours: numOrPrev(v, settings.chatMessageTTLHours ?? 2160),
                })
              }}
              placeholder="2160"
              error={errors?.chatMessageTTLHours}
            />
          </Field>
        </div>
      </Section>
    </>
  )
}
