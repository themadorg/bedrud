import { useRoomContext } from '@livekit/components-react'
import { Camera, FlaskConical, Lock, Mic, Palette, User } from 'lucide-react'
import { useState } from 'react'
import { AppearanceSettingsPanel } from '#/components/settings/AppearanceSettingsPanel'
import { AudioSettingsPanel } from '#/components/settings/AudioSettingsPanel'
import { ExperimentalSettingsPanel } from '#/components/settings/ExperimentalSettingsPanel'
import { ProfileSettingsPanel } from '#/components/settings/ProfileSettingsPanel'
import { SecuritySettingsPanel } from '#/components/settings/SecuritySettingsPanel'
import { VideoSettingsPanel } from '#/components/settings/VideoSettingsPanel'
import { cn } from '#/lib/utils'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { meetingPanelScopeClass, settingsDialogScrollClass, settingsSidebarTabClass } from './settingsPanelTone'

const TABS = [
  { id: 'profile', label: 'Profile', icon: User },
  { id: 'appearance', label: 'Appearance', icon: Palette },
  { id: 'audio', label: 'Audio', icon: Mic },
  { id: 'video', label: 'Video', icon: Camera },
  { id: 'security', label: 'Security', icon: Lock },
  { id: 'experimental', label: 'Experimental', icon: FlaskConical },
] as const

type TabId = (typeof TABS)[number]['id']

function MeetingVideoSettingsPanel() {
  const room = useRoomContext()
  return (
    <VideoSettingsPanel
      tone="meeting"
      onCameraDeviceChange={(deviceId) => {
        void room.switchActiveDevice('videoinput', deviceId).catch(() => {})
      }}
    />
  )
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function BedrudSettingsDialog({ open, onOpenChange }: Props) {
  const [tab, setTab] = useState<TabId>('audio')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="meet-dialog flex h-[min(90vh,720px)] w-[min(92vw,760px)] max-w-[min(92vw,760px)] flex-col gap-0 overflow-hidden p-0 shadow-2xl">
        <DialogHeader className="shrink-0 border-b border-[var(--meet-border)] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-[var(--meet-fg)]">Settings</DialogTitle>
        </DialogHeader>

        <Tabs
          value={tab}
          onValueChange={(v) => setTab(v as TabId)}
          className="flex min-h-0 flex-1 flex-row overflow-hidden"
          orientation="vertical"
        >
          <div className="flex w-[148px] shrink-0 flex-col border-r border-[var(--meet-border)] bg-[var(--meet-surface-muted)] px-2 py-3">
            <TabsList className="flex h-auto w-full flex-col items-stretch gap-0.5 bg-transparent p-0 text-[var(--meet-fg-muted)]">
              {TABS.map(({ id, label, icon: Icon }) => (
                <TabsTrigger
                  key={id}
                  value={id}
                  className={cn(
                    'h-auto w-full justify-start gap-2 rounded-md px-3 py-2 text-xs shadow-none ring-offset-[var(--meet-bg-panel)]',
                    settingsSidebarTabClass,
                  )}
                >
                  <Icon className="h-3.5 w-3.5 shrink-0" />
                  {label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          <div
            className={cn(
              'min-h-0 min-w-0 flex-1 overflow-y-auto p-4',
              settingsDialogScrollClass,
              meetingPanelScopeClass,
            )}
          >
            <TabsContent value="profile" className="mt-0 outline-none">
              <ProfileSettingsPanel tone="meeting" />
            </TabsContent>
            <TabsContent value="appearance" className="mt-0 outline-none">
              <AppearanceSettingsPanel tone="meeting" />
            </TabsContent>
            <TabsContent value="security" className="mt-0 outline-none">
              <SecuritySettingsPanel />
            </TabsContent>
            <TabsContent value="audio" className="mt-0 outline-none">
              <AudioSettingsPanel tone="meeting" />
            </TabsContent>
            <TabsContent value="video" className="mt-0 outline-none">
              <MeetingVideoSettingsPanel />
            </TabsContent>
            <TabsContent value="experimental" className="mt-0 outline-none">
              <ExperimentalSettingsPanel tone="meeting" />
            </TabsContent>
          </div>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
