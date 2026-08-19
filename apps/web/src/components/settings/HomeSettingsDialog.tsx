import { Camera, FlaskConical, Mic, Palette, User } from 'lucide-react'
import { useEffect, useState } from 'react'
import { AppearanceSettingsPanel } from '#/components/settings/AppearanceSettingsPanel'
import { AudioSettingsPanel } from '#/components/settings/AudioSettingsPanel'
import { ExperimentalSettingsPanel } from '#/components/settings/ExperimentalSettingsPanel'
import { ProfileSettingsPanel } from '#/components/settings/ProfileSettingsPanel'
import { VideoSettingsPanel } from '#/components/settings/VideoSettingsPanel'
import { appScrollClass, appScrollYClass } from '#/components/settings/settingsPanelTone'
import { cn } from '#/lib/utils'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const TABS = [
  { id: 'profile', label: 'Profile', icon: User },
  { id: 'appearance', label: 'Appearance', icon: Palette },
  { id: 'audio', label: 'Audio', icon: Mic },
  { id: 'video', label: 'Video', icon: Camera },
  { id: 'experimental', label: 'Experimental', icon: FlaskConical },
] as const

type TabId = (typeof TABS)[number]['id']

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function HomeSettingsDialog({ open, onOpenChange }: Props) {
  const [tab, setTab] = useState<TabId>('profile')

  useEffect(() => {
    if (!open) setTab('profile')
  }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[min(90svh,720px)] max-h-[min(90svh,720px)] w-[min(760px,calc(var(--app-width,100svw)-2rem))] max-w-[760px] flex-col gap-0 overflow-hidden p-0">
        <DialogHeader className="shrink-0 border-b border-border px-4 py-3">
          <DialogTitle className="text-base font-semibold">Settings</DialogTitle>
        </DialogHeader>

        <Tabs
          value={tab}
          onValueChange={(v) => setTab(v as TabId)}
          className="flex min-h-0 flex-1 flex-col overflow-hidden sm:flex-row"
          orientation="vertical"
        >
          <div className="shrink-0 border-b border-border bg-muted/40 px-2 py-2 sm:w-[148px] sm:border-b-0 sm:border-e sm:py-3">
            <TabsList
              className={cn(
                'flex h-auto w-full flex-row items-stretch gap-0.5 overflow-x-auto bg-transparent p-0 sm:flex-col',
                appScrollClass,
              )}
            >
              {TABS.map(({ id, label, icon: Icon }) => (
                <TabsTrigger
                  key={id}
                  value={id}
                  className={cn(
                    'h-auto justify-start gap-2 px-3 py-2 text-xs shadow-none data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm',
                  )}
                >
                  <Icon className="h-3.5 w-3.5 shrink-0" />
                  {label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          <div className={cn('min-h-0 min-w-0 flex-1 overflow-y-auto p-4', appScrollYClass)}>
            <TabsContent value="profile" className="mt-0 outline-none">
              <ProfileSettingsPanel />
            </TabsContent>
            <TabsContent value="appearance" className="mt-0 outline-none">
              <AppearanceSettingsPanel />
            </TabsContent>
            <TabsContent value="audio" className="mt-0 outline-none">
              <AudioSettingsPanel />
            </TabsContent>
            <TabsContent value="video" className="mt-0 outline-none">
              <VideoSettingsPanel />
            </TabsContent>
            <TabsContent value="experimental" className="mt-0 outline-none">
              <ExperimentalSettingsPanel />
            </TabsContent>
          </div>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
