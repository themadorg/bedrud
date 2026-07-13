// TODO oncoming feature
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { Loader2, Save } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import {
  AudioTab,
  AuthTab,
  ChatTab,
  CorsTab,
  EmailTab,
  GeneralTab,
  LiveKitTab,
  LoggingTab,
  ServerTab,
  type SystemSettings,
  validateLocalSettings,
  WebxdcTab,
} from '#/components/admin/settings'
import { WebhookSection } from '#/components/admin/settings/webhook-section'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { refreshPublicSettings } from '#/lib/use-public-settings'
import { useUserStore } from '#/lib/user.store'
import { cn } from '#/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function requireAdminSettingsAccess() {
  if (typeof window === 'undefined') return
  const user = useUserStore.getState().user
  const accesses = user?.accesses ?? []
  if (accesses.includes('moderator') || (!user?.isSuperAdmin && !accesses.includes('admin'))) {
    throw redirect({ to: '/dashboard/admin' })
  }
}

const TABS = [
  { id: 'general', label: 'General' },
  { id: 'auth', label: 'Authentication' },
  { id: 'livekit', label: 'LiveKit' },
  { id: 'server', label: 'Server' },
  { id: 'email', label: 'Email' },
  { id: 'cors', label: 'CORS' },
  { id: 'chat', label: 'Chat' },
  { id: 'audio', label: 'Audio' },
  // TODO oncoming feature
  // { id: 'recordings', label: 'Recordings' },
  { id: 'webxdc', label: 'WebXDC' },
  { id: 'logging', label: 'Logging' },
  { id: 'webhooks', label: 'Webhooks' },
] as const

export type AdminSettingsTabId = (typeof TABS)[number]['id']

const TAB_IDS = new Set<string>(TABS.map((t) => t.id))

function parseTab(raw: unknown): AdminSettingsTabId | undefined {
  if (typeof raw !== 'string') return undefined
  if (raw === 'audio') return undefined // audio uses its own path
  if (TAB_IDS.has(raw)) return raw as AdminSettingsTabId
  return undefined
}

type SettingsSearch = { tab?: AdminSettingsTabId }

export const Route = createFileRoute('/dashboard/admin/settings')({
  beforeLoad: requireAdminSettingsAccess,
  validateSearch: (search: Record<string, unknown>): SettingsSearch => {
    const tab = parseTab(search.tab)
    return tab ? { tab } : {}
  },
  component: AdminSettingsIndexRoute,
})

function AdminSettingsIndexRoute() {
  const { tab } = Route.useSearch()
  return <AdminSettingsPage initialTab={tab ?? 'general'} />
}

// Fields each tab owns. Save only sends the current tab's fields.
const TAB_FIELDS: Record<AdminSettingsTabId, (keyof SystemSettings)[]> = {
  general: ['registrationEnabled', 'tokenRegistrationOnly'],
  auth: [
    'passkeysEnabled',
    'googleClientId',
    'googleClientSecret',
    'googleRedirectUrl',
    'githubClientId',
    'githubClientSecret',
    'githubRedirectUrl',
    'twitterClientId',
    'twitterClientSecret',
    'twitterRedirectUrl',
    'jwtSecret',
    'sessionSecret',
    'tokenDuration',
    'frontendUrl',
    'guestLoginEnabled',
  ],
  livekit: ['livekitHost', 'livekitApiKey', 'livekitApiSecret', 'livekitExternal'],
  server: [
    'serverPort',
    'serverHost',
    'serverDomain',
    'serverEmail',
    'serverEnableTls',
    'serverCertFile',
    'serverKeyFile',
    'serverUseAcme',
    'behindProxy',
    'serverName',
    'maxParticipantsLimit',
    'maxRoomsPerUser',
  ],
  email: [
    'emailInstanceName',
    'emailSupportEmail',
    'emailInstanceUrl',
    'emailHeaderBg',
    'emailButtonBg',
    'emailSubjectVerify',
    'emailSubjectWelcome',
    'emailSubjectReset',
    'emailSubjectChanged',
    'emailSubjectInvite',
    'emailPreheaderVerify',
    'emailPreheaderWelcome',
    'emailPreheaderReset',
    'emailPreheaderChanged',
    'emailPreheaderInvite',
    'emailSmtpHost',
    'emailSmtpPort',
    'emailUsername',
    'emailPassword',
    'emailFromAddress',
    'emailFromName',
    'emailTlsSkipVerify',
    'emailSmtpsMode',
  ],
  cors: ['corsAllowedOrigins', 'corsAllowedHeaders', 'corsAllowedMethods', 'corsAllowCredentials', 'corsMaxAge'],
  chat: [
    'chatUploadBackend',
    'chatUploadMaxBytes',
    'chatUploadMaxDimension',
    'chatUploadInlineMax',
    'chatUploadDiskDir',
    'chatUploadS3Endpoint',
    'chatUploadS3Bucket',
    'chatUploadS3Region',
    'chatUploadS3AccessKey',
    'chatUploadS3SecretKey',
    'chatUploadS3PublicUrl',
    'chatMaxMessageCount',
    'chatMessageTTLHours',
    'maxUploadBytesPerUser',
    'globalDiskThresholdBytes',
  ],
  webxdc: [
    'webxdcGalleryEnabled',
    'webxdcGalleryRemoteCatalogUrl',
    'webxdcGallerySource',
    'webxdcGalleryAllowRemoteDownload',
    'webxdcGalleryInstanceCatalogEnabled',
    'webxdcMaxArchiveMB',
    'webxdcMaxUncompressedMB',
    'webxdcMaxSingleFileMB',
    'webxdcMaxEntries',
  ],
  audio: ['rnnoiseEnabled', 'krispEnabled'],
  logging: ['logLevel'],
  webhooks: [],
}

export function AdminSettingsPage({ initialTab }: { initialTab: AdminSettingsTabId }) {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<AdminSettingsTabId>(initialTab)
  const [drafts, setDrafts] = useState<Partial<Record<AdminSettingsTabId, Partial<SystemSettings>>>>({})
  const [localErrors, setLocalErrors] = useState<Record<string, string>>({})
  const queryClient = useQueryClient()

  useEffect(() => {
    setActiveTab(initialTab)
  }, [initialTab])

  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: ['admin', 'settings'],
    queryFn: () => api.get<SystemSettings>('/api/admin/settings'),
  })

  const current = useMemo(() => {
    if (!settings) return null
    const merged = {
      ...settings,
      rnnoiseEnabled: settings.rnnoiseEnabled ?? false,
      krispEnabled: settings.krispEnabled ?? false,
    }
    for (const draft of Object.values(drafts)) {
      if (draft) Object.assign(merged, draft)
    }
    return merged
  }, [settings, drafts])

  function clearFieldError(field: string) {
    setLocalErrors((prev) => {
      if (!(field in prev)) return prev
      const next = { ...prev }
      delete next[field]
      return next
    })
  }

  const saveSettings = useMutation({
    mutationFn: (s: Partial<SystemSettings>) => api.put('/api/admin/settings', s),
    onSuccess: () => {
      setDrafts((prev) => {
        const next = { ...prev }
        delete next[activeTab]
        return next
      })
      setLocalErrors({})
      queryClient.invalidateQueries({ queryKey: ['admin', 'settings'] })
      refreshPublicSettings()
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to save settings'))
    },
  })

  function onTabChange(v: AdminSettingsTabId) {
    saveSettings.reset()
    setLocalErrors({})
    if (v === 'audio') {
      void navigate({ to: '/dashboard/admin/settings/audio' })
      return
    }
    // Leaving audio deep-link or switching tabs on index: keep selection via search
    void navigate({
      to: '/dashboard/admin/settings',
      search: v === 'general' ? {} : { tab: v },
    })
  }

  function handlePatch(partial: Partial<SystemSettings>) {
    if (!settings) return
    if (saveSettings.isPending) return
    saveSettings.mutate(partial)
  }

  function handleTabChange(newSettings: SystemSettings) {
    if (!settings) return
    const tabFields = TAB_FIELDS[activeTab]
    const changes: Partial<SystemSettings> = {}
    for (const field of tabFields) {
      if (newSettings[field] !== settings[field]) {
        ;(changes as Record<string, unknown>)[field] = newSettings[field]
      }
    }
    setDrafts((prev) => ({
      ...prev,
      [activeTab]: Object.keys(changes).length > 0 ? changes : undefined,
    }))
  }

  function saveCurrentTab() {
    if (!settings) return
    if (saveSettings.isPending) return

    const draft = drafts[activeTab]
    if (!draft) return

    const merged = { ...settings }
    for (const d of Object.values(drafts)) {
      if (d) Object.assign(merged, d)
    }
    const allErrors = validateLocalSettings(merged)
    const tabFields = new Set(TAB_FIELDS[activeTab])
    const errors: Record<string, string> = {}
    for (const [field, msg] of Object.entries(allErrors)) {
      if (tabFields.has(field as keyof SystemSettings)) {
        errors[field] = msg
      }
    }
    setLocalErrors(errors)
    if (Object.keys(errors).length > 0) return

    saveSettings.mutate(draft)
  }

  const hasDraft = !!drafts[activeTab] && Object.keys(drafts[activeTab]!).length > 0

  return (
    <div className="mx-auto max-w-6xl space-y-6 px-4">
      <div>
        <h1 className="text-sm font-semibold">System settings</h1>
        <p className="text-xs text-muted-foreground">Manage auth, infrastructure, and server configuration.</p>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => onTabChange(v as AdminSettingsTabId)}>
        <TabsList className="flex h-auto w-full flex-wrap justify-start rounded-none border-b bg-transparent p-0">
          {TABS.map((tab) => (
            <TabsTrigger
              key={tab.id}
              value={tab.id}
              className={cn(
                'shrink-0 rounded-none px-3 py-2.5 text-xs font-medium data-[state=active]:shadow-none',
                'data-[state=active]:border-b-2 data-[state=active]:border-primary data-[state=active]:text-primary',
                'data-[state=inactive]:text-muted-foreground data-[state=inactive]:hover:text-foreground',
              )}
            >
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

        {settingsLoading ? (
          <div className="space-y-4">
            {[...Array(2)].map((_, i) => (
              <Skeleton key={i} className="h-40" />
            ))}
          </div>
        ) : current ? (
          <div className="space-y-4">
            <div className="sticky top-0 z-10 flex items-center gap-3 border-b bg-background/95 px-5 py-3 backdrop-blur">
              <Button
                type="button"
                variant={hasDraft ? 'default' : 'outline'}
                size="sm"
                onClick={saveCurrentTab}
                disabled={!hasDraft || saveSettings.isPending || Object.keys(localErrors).length > 0}
                className="gap-1.5"
              >
                {saveSettings.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
                Save changes
              </Button>
              {saveSettings.isSuccess && <span className="text-xs text-emerald-600">Saved</span>}
              {!hasDraft && !saveSettings.isSuccess && (
                <span className="text-[10px] text-muted-foreground">No pending changes on this tab</span>
              )}
              {hasDraft && (
                <>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    onClick={() =>
                      setDrafts((prev) => {
                        const next = { ...prev }
                        delete next[activeTab]
                        return next
                      })
                    }
                  >
                    Discard
                  </Button>
                  {!saveSettings.isSuccess && !saveSettings.isPending && (
                    <span className="text-[10px] text-muted-foreground">Unsaved changes on this tab</span>
                  )}
                </>
              )}
              {Object.keys(localErrors).length > 0 && (
                <div role="alert" aria-live="polite" aria-atomic="true" className="flex flex-col gap-0.5">
                  <span className="text-xs font-medium text-destructive">
                    {Object.keys(localErrors).length} field error{Object.keys(localErrors).length > 1 ? 's' : ''}
                  </span>
                  {Object.entries(localErrors).map(([field, msg]) => (
                    <span key={field} className="text-[10px] text-destructive/80">
                      {field}: {msg}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <TabsContent value="general">
              <GeneralTab settings={current} onPatch={handlePatch} saving={saveSettings.isPending} />
            </TabsContent>
            <TabsContent value="auth">
              <AuthTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="livekit">
              <LiveKitTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="server">
              <ServerTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="email">
              <EmailTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="cors">
              <CorsTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="chat">
              <ChatTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            {/* TODO oncoming feature — RecordingsTab removed */}
            <TabsContent value="webxdc">
              <WebxdcTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent>
            <TabsContent value="audio">
              <AudioTab settings={current} setSettings={handleTabChange} />
            </TabsContent>
            <TabsContent value="logging">
              <LoggingTab settings={current} setSettings={handleTabChange} />
            </TabsContent>
            <TabsContent value="webhooks">
              <WebhookSection settings={current} />
            </TabsContent>
          </div>
        ) : null}
      </Tabs>
    </div>
  )
}
