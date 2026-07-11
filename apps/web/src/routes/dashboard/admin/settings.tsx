// TODO oncoming feature
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { Loader2, Save } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import {
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
import { useUserStore } from '#/lib/user.store'
import { cn } from '#/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export const Route = createFileRoute('/dashboard/admin/settings')({
  beforeLoad: () => {
    if (typeof window === 'undefined') return
    const user = useUserStore.getState().user
    const accesses = user?.accesses ?? []
    if (accesses.includes('moderator') || (!user?.isSuperAdmin && !accesses.includes('admin'))) {
      throw redirect({ to: '/dashboard/admin' })
    }
  },
  component: AdminSettingsPage,
})

const TABS = [
  { id: 'general', label: 'General' },
  { id: 'auth', label: 'Authentication' },
  { id: 'livekit', label: 'LiveKit' },
  { id: 'server', label: 'Server' },
  { id: 'email', label: 'Email' },
  { id: 'cors', label: 'CORS' },
  { id: 'chat', label: 'Chat' },
  // TODO oncoming feature
  // { id: 'recordings', label: 'Recordings' },
  { id: 'webxdc', label: 'WebXDC' },
  { id: 'logging', label: 'Logging' },
  { id: 'webhooks', label: 'Webhooks' },
] as const

type TabId = (typeof TABS)[number]['id']

// Fields each tab owns. Save only sends the current tab's fields.
const TAB_FIELDS: Record<TabId, (keyof SystemSettings)[]> = {
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
  logging: ['logLevel'],
  webhooks: [],
}

function AdminSettingsPage() {
  const [activeTab, setActiveTab] = useState<TabId>('general')
  // Per-tab pending partial changes. Only fields the user actually modified on each tab.
  const [drafts, setDrafts] = useState<Partial<Record<TabId, Partial<SystemSettings>>>>({})
  const [localErrors, setLocalErrors] = useState<Record<string, string>>({})
  const queryClient = useQueryClient()

  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: ['admin', 'settings'],
    queryFn: () => api.get<SystemSettings>('/api/admin/settings'),
  })

  // Merge server settings with ALL drafts for rendering — each tab sees its pending changes
  const current = useMemo(() => {
    if (!settings) return null
    const merged = { ...settings }
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
      // Clear only the saved tab's draft; other tabs' drafts stay
      setDrafts((prev) => {
        const next = { ...prev }
        delete next[activeTab]
        return next
      })
      setLocalErrors({})
      // Refetch settings so UI shows latest values
      queryClient.invalidateQueries({ queryKey: ['admin', 'settings'] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err, 'Failed to save settings'))
    },
  })

  function onTabChange(v: TabId) {
    saveSettings.reset()
    setLocalErrors({})
    setActiveTab(v)
  }

  // Used by GeneralTab for immediate auto-save
  function handlePatch(partial: Partial<SystemSettings>) {
    if (!settings) return
    if (saveSettings.isPending) return

    // GeneralTab only changes registration mode fields — no complex validation needed.
    // Backend validates and returns 400 on error, shown as toast.
    saveSettings.mutate(partial)
  }

  // Used by non-General tabs — stores only the current tab's changed fields
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

  // Save only the current tab's pending changes
  function saveCurrentTab() {
    if (!settings) return
    if (saveSettings.isPending) return

    const draft = drafts[activeTab]
    if (!draft) return

    // Merge all drafts to build the full state for cross-field validation
    const merged = { ...settings }
    for (const d of Object.values(drafts)) {
      if (d) Object.assign(merged, d)
    }
    const allErrors = validateLocalSettings(merged)
    // Only show errors for fields on the current tab — other tabs' bad values
    // shouldn't block saving on this tab.
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

  const hasDraft = !!drafts[activeTab] && Object.keys(drafts[activeTab]).length > 0

  return (
    <div className="mx-auto max-w-6xl space-y-6 px-4">
      <div>
        <h1 className="text-sm font-semibold">System settings</h1>
        <p className="text-xs text-muted-foreground">Manage auth, infrastructure, and server configuration.</p>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => onTabChange(v as TabId)}>
        <TabsList className="w-full justify-start border-b rounded-none bg-transparent p-0 h-auto">
          {TABS.map((tab) => (
            <TabsTrigger
              key={tab.id}
              value={tab.id}
              className={cn(
                'shrink-0 px-3 py-2.5 text-xs font-medium rounded-none data-[state=active]:shadow-none',
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
            {/* <TabsContent value="recordings">
              <RecordingsTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
            </TabsContent> */}
            <TabsContent value="webxdc">
              <WebxdcTab
                settings={current}
                setSettings={handleTabChange}
                errors={localErrors}
                clearFieldError={clearFieldError}
              />
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
